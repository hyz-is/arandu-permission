package feature_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/framework/data"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/security"
	hdatabase "github.com/arandu-io/hesape/database"
	hview "github.com/arandu-io/hesape/view"

	permission "github.com/hyz-is/arandu-permission"

	// The driver, for the tests and for nothing else. It is a test import, so
	// nothing an application installs pulls it in: what a dependency needs to
	// prove itself is not what a dependency needs to run.
	_ "github.com/arandu-io/hesape/database/connectors/sqlite"
)

// The four protections this package exists to have, each against real rows.
//
// They are here rather than beside the policy tests because none of them is a
// property of one function: a permission that survives revocation, a row that
// crosses a customer boundary and a group left with nobody in it are all states
// that take a write, a read and something in between to produce. A test that
// mocked the middle would be testing the mock.
//
// The four are named in one place, so that a reader can find them:
//
//	TestAnActorCannotGrantAnActionItDoesNotHold        self-elevation
//	TestAnActorCannotPutItselfIntoAGroup               self-elevation
//	TestNothingOfOneTenantIsReachableFromAnother       cross-tenant access
//	TestARevokedActionIsNotServedFromTheResolver       stale cache
//	TestTheLastMemberOfASystemGroupCannotBeRemoved     the last administrator
//	TestASystemGroupIsNotDeletedThroughTheService      the last administrator
//	TestTheRouteRefusesWhatTheScreenWouldNotOffer      verification on the server

// store opens an empty database with this module's schema applied.
//
// It is a file rather than :memory:, because a pool over an in-memory database
// gives each connection a database of its own and the second statement of a
// test lands somewhere the first one never wrote.
func store(t *testing.T) *data.DB {
	t.Helper()

	pool, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "permission.db"))
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	handle := data.Wrap(pool, data.DialectSQLite)
	module, err := build(t, settings(), handle)
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	connection := hdatabase.NewConnection(pool, "main", "", map[string]any{"driver": "sqlite"})
	for _, migration := range module.Migrations() {
		if err := migration.Up(context.Background(), hdatabase.ForMigrations(connection)); err != nil {
			t.Fatalf("applying %s: %v", migration.GetName(), err)
		}
	}
	return handle
}

// service is the service over an empty, migrated database.
func service(t *testing.T) *permission.PermissionService {
	t.Helper()

	catalogue, err := permission.NewCatalogue(settings().Actions...)
	if err != nil {
		t.Fatalf("building the catalogue: %v", err)
	}
	return permission.NewPermissionService(store(t), catalogue)
}

// operator is the subject a seed acts as: an identifier, a tenant, and this
// package's own actions as what it carries.
//
// It is how the first group of an installation is created, and it is the only
// place a subject's actions come from anywhere but the resolution -- because
// until the first group exists there is nothing to resolve.
func operator(tenant string, extra ...security.Action) security.Subject {
	actions := append(permission.Actions(), extra...)
	return security.Subject{ID: "seed", Tenant: tenant, Actions: actions, Verified: true}
}

// seed creates one group and returns it.
func seed(t *testing.T, svc *permission.PermissionService, actor security.Subject, in permission.CreateGroupRequest) *permission.Group {
	t.Helper()

	group, err := svc.CreateGroup(context.Background(), actor, in)
	if err != nil {
		t.Fatalf("creating the group %s: %v", in.Slug, err)
	}
	return group
}

// TestAnActorCannotGrantAnActionItDoesNotHold is the self-elevation refusal.
//
// The subject here may administer permissions in every way this package
// declares, and does not hold invoice.delete. Without the rule, the shortest
// path to holding it is three clicks: attach it to a group, put yourself in the
// group, reload.
func TestAnActorCannotGrantAnActionItDoesNotHold(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	actor := operator("acme")
	group := seed(t, svc, actor, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})

	_, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"})
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("granting an action the subject does not hold answered %v, want ErrForbidden", err)
	}

	// Nothing was written, which is the half a refusal that returned after the
	// first row would fail.
	held, err := svc.ActionsOf(ctx, actor, group.ID)
	if err != nil {
		t.Fatalf("reading what the group carries: %v", err)
	}
	if len(held) != 0 {
		t.Fatalf("the group carries %v after a refused grant", held)
	}

	// Holding it is the whole difference, and it is what a person with the
	// permission is meant to be able to do.
	holder := operator("acme", "invoice.delete")
	if _, err := svc.SetActions(ctx, holder, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("a subject that holds the action was refused: %v", err)
	}
}

// TestAnActorCannotPutItselfIntoAGroup is the other half of self-elevation.
func TestAnActorCannotPutItselfIntoAGroup(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	actor := operator("acme")
	group := seed(t, svc, actor, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})

	_, err := svc.SetMembers(ctx, actor, group.ID, []string{actor.ID})
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("a subject put itself into a group: %v", err)
	}
	members, err := svc.MembersOf(ctx, actor, group.ID)
	if err != nil {
		t.Fatalf("reading the members: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("the group holds %v after a refused assignment", members)
	}

	// Somebody else is exactly what the screen is for.
	if _, err := svc.SetMembers(ctx, actor, group.ID, []string{"user-9"}); err != nil {
		t.Fatalf("putting somebody else into a group was refused: %v", err)
	}
}

// TestNothingOfOneTenantIsReachableFromAnother is the cross-tenant protection,
// over rows that exist.
//
// Both halves are checked. The read is scoped, so a group of another customer
// is not found -- and the policy compares the tenant on the row that came back,
// so a read that stopped being scoped would still be refused rather than
// answered.
func TestNothingOfOneTenantIsReachableFromAnother(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()

	ours := operator("acme")
	theirs := operator("globex")

	mine := seed(t, svc, ours, permission.CreateGroupRequest{Slug: "editors", Name: "Ours"})
	yours := seed(t, svc, theirs, permission.CreateGroupRequest{Slug: "editors", Name: "Theirs"})
	if mine.ID == yours.ID {
		t.Fatal("two tenants were given one group, so nothing below proves anything")
	}

	// The same slug in two customers is the case an index that left the tenant
	// out would have made impossible.
	if mine.Slug != yours.Slug {
		t.Fatalf("the two groups have different slugs, %q and %q", mine.Slug, yours.Slug)
	}

	if _, err := svc.FindGroup(ctx, ours, yours.ID); !errors.Is(err, permission.ErrNotFound) {
		t.Errorf("a group of another tenant was found: %v", err)
	}
	if _, err := svc.ActionsOf(ctx, ours, yours.ID); !errors.Is(err, permission.ErrNotFound) {
		t.Errorf("the actions of another tenant's group were read: %v", err)
	}
	if _, err := svc.MembersOf(ctx, ours, yours.ID); !errors.Is(err, permission.ErrNotFound) {
		t.Errorf("the members of another tenant's group were read: %v", err)
	}
	if _, err := svc.SetActions(ctx, ours, yours.ID, nil); !errors.Is(err, permission.ErrNotFound) {
		t.Errorf("another tenant's group was written to: %v", err)
	}
	if err := svc.DeleteGroup(ctx, ours, yours.ID); !errors.Is(err, permission.ErrNotFound) {
		t.Errorf("another tenant's group was deleted: %v", err)
	}

	// And the listing answers with one row rather than two.
	page, err := svc.ListGroups(ctx, ours, permission.GroupQuery{})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != mine.ID {
		t.Fatalf("the listing answered %d rows, want only this tenant's one", len(page.Items))
	}

	// A membership of one customer does not reach the other, which is the leak
	// that survives a correct group listing: the person is in a group of theirs
	// and asks about it here.
	if _, err := svc.SetMembers(ctx, theirs, yours.ID, []string{"user-9"}); err != nil {
		t.Fatalf("putting somebody into their own group was refused: %v", err)
	}
	effective, err := svc.EffectiveFor(ctx, ours, "user-9")
	if err != nil {
		t.Fatalf("reading effective permissions: %v", err)
	}
	if len(effective.Groups) != 0 || len(effective.Grants) != 0 {
		t.Fatalf("a membership of another tenant was answered: %+v", effective)
	}
}

// TestARevokedActionIsNotServedFromTheResolver is the stale cache protection.
//
// The resolver remembers what a subject carries, and the tenant's token is what
// says the memory is still the answer. Without the token, the second resolution
// below answers with the permission the first one saw -- which is a person
// keeping access after it was taken away, for as long as the process lives.
func TestARevokedActionIsNotServedFromTheResolver(t *testing.T) {
	t.Parallel()

	svc := service(t)
	resolver := permission.NewResolver(svc, 16)
	ctx := context.Background()

	admin := operator("acme", "invoice.delete")
	group := seed(t, svc, admin, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if _, err := svc.SetActions(ctx, admin, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}
	if _, err := svc.SetMembers(ctx, admin, group.ID, []string{"user-9"}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	// A subject as the middleware would see it: signed in, carrying nothing
	// until this answers.
	member := security.Subject{ID: "user-9", Tenant: "acme"}

	before, err := resolver.Resolve(ctx, member)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if !holds(before, "invoice.delete") {
		t.Fatalf("the resolution answered %v, and the person is in a group that carries invoice.delete", before)
	}

	// Resolving twice with nothing changed answers the same thing, which is the
	// case the memory exists for.
	again, err := resolver.Resolve(ctx, member)
	if err != nil {
		t.Fatalf("resolving again: %v", err)
	}
	if !holds(again, "invoice.delete") {
		t.Fatalf("a second resolution with nothing changed answered %v", again)
	}

	// Now take it away, through the same path a screen uses.
	if _, err := svc.SetActions(ctx, admin, group.ID, nil); err != nil {
		t.Fatalf("revoking: %v", err)
	}

	after, err := resolver.Resolve(ctx, member)
	if err != nil {
		t.Fatalf("resolving after the revocation: %v", err)
	}
	if holds(after, "invoice.delete") {
		t.Fatalf("a revoked action was served from the remembered resolution: %v", after)
	}

	// The same holds for a membership rather than a grant: the person leaves
	// the group and stops carrying what it carried.
	if _, err := svc.SetActions(ctx, admin, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting again: %v", err)
	}
	restored, err := resolver.Resolve(ctx, member)
	if err != nil {
		t.Fatalf("resolving after the grant: %v", err)
	}
	if !holds(restored, "invoice.delete") {
		t.Fatalf("the resolution did not pick the action back up: %v", restored)
	}
	if _, err := svc.SetMembers(ctx, admin, group.ID, nil); err != nil {
		t.Fatalf("removing the member: %v", err)
	}
	left, err := resolver.Resolve(ctx, member)
	if err != nil {
		t.Fatalf("resolving after leaving: %v", err)
	}
	if holds(left, "invoice.delete") {
		t.Fatalf("somebody who left the group kept what it carried: %v", left)
	}
}

// TestTheVersionChangesOnEveryWriteThatCouldChangeADecision is what the test
// above depends on, checked on its own so that a failure says which of the two
// broke.
func TestTheVersionChangesOnEveryWriteThatCouldChangeADecision(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	admin := operator("acme", "invoice.delete")
	group := seed(t, svc, admin, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})

	seen := map[int64]bool{}
	record := func(step string) {
		version, err := svc.Version(ctx, admin)
		if err != nil {
			t.Fatalf("reading the version after %s: %v", step, err)
		}
		if seen[version] {
			t.Fatalf("the token did not change after %s, so a remembered answer outlives it", step)
		}
		seen[version] = true
	}

	record("the start")
	if _, err := svc.SetActions(ctx, admin, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}
	record("a grant")
	if _, err := svc.SetMembers(ctx, admin, group.ID, []string{"user-9"}); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	record("an assignment")
	if _, err := svc.SetMembers(ctx, admin, group.ID, nil); err != nil {
		t.Fatalf("unassigning: %v", err)
	}
	record("an unassignment")
	if _, err := svc.SetActions(ctx, admin, group.ID, nil); err != nil {
		t.Fatalf("revoking: %v", err)
	}
	record("a revocation")
	if err := svc.DeleteGroup(ctx, admin, group.ID); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	record("a deletion")

	// Renaming is the write that must NOT change it. Nothing about a name
	// reaches a decision, and invalidating every resolved subject for one would
	// be a stampede for nothing.
	other := seed(t, svc, admin, permission.CreateGroupRequest{Slug: "readers", Name: "Readers"})
	before, err := svc.Version(ctx, admin)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if _, err := svc.UpdateGroup(ctx, admin, other.ID, permission.UpdateGroupRequest{Name: "Reviewers"}); err != nil {
		t.Fatalf("renaming: %v", err)
	}
	after, err := svc.Version(ctx, admin)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if before != after {
		t.Error("renaming a group changed the token, which re-resolves every signed-in subject for a change no policy reads")
	}
}

// TestTheLastMemberOfASystemGroupCannotBeRemoved is the last administrator
// protection.
//
// It is the state that cannot be undone from inside the application: the group
// that carries the administration of permissions with nobody in it, and
// therefore nobody who could put anybody back.
func TestTheLastMemberOfASystemGroupCannotBeRemoved(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	admin := operator("acme")
	group := seed(t, svc, admin, permission.CreateGroupRequest{
		Slug: "administrator", Name: "Administrator", System: true,
	})

	if _, err := svc.SetMembers(ctx, admin, group.ID, []string{"user-1", "user-2"}); err != nil {
		t.Fatalf("seeding the members: %v", err)
	}

	// Down to one is allowed: there is still somebody who can administer.
	if _, err := svc.SetMembers(ctx, admin, group.ID, []string{"user-1"}); err != nil {
		t.Fatalf("removing one of two members was refused: %v", err)
	}

	// Down to none is not.
	if _, err := svc.SetMembers(ctx, admin, group.ID, nil); !errors.Is(err, permission.ErrLastMember) {
		t.Fatalf("emptying a system group answered %v, want ErrLastMember", err)
	}
	members, err := svc.MembersOf(ctx, admin, group.ID)
	if err != nil {
		t.Fatalf("reading the members: %v", err)
	}
	if len(members) != 1 || members[0] != "user-1" {
		t.Fatalf("the group holds %v after a refused removal", members)
	}

	// Swapping the last member for another is allowed, because the group is
	// never left empty: refusing it would make a departing administrator
	// permanent.
	if _, err := svc.SetMembers(ctx, admin, group.ID, []string{"user-3"}); err != nil {
		t.Fatalf("replacing the last member was refused: %v", err)
	}

	// The same is not true of an ordinary group, and the difference is the
	// whole of what the flag is for.
	ordinary := seed(t, svc, admin, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if _, err := svc.SetMembers(ctx, admin, ordinary.ID, []string{"user-4"}); err != nil {
		t.Fatalf("seeding an ordinary group: %v", err)
	}
	if _, err := svc.SetMembers(ctx, admin, ordinary.ID, nil); err != nil {
		t.Fatalf("emptying an ordinary group was refused: %v", err)
	}
}

// TestASystemGroupIsNotDeletedThroughTheService is the other way to end up with
// nobody who can administer, and it is refused in the policy rather than here.
func TestASystemGroupIsNotDeletedThroughTheService(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	admin := operator("acme")
	group := seed(t, svc, admin, permission.CreateGroupRequest{
		Slug: "administrator", Name: "Administrator", System: true,
	})
	if _, err := svc.SetMembers(ctx, admin, group.ID, []string{"user-1"}); err != nil {
		t.Fatalf("seeding the member: %v", err)
	}
	if _, err := svc.SetActions(ctx, admin, group.ID, permission.Actions()); err != nil {
		t.Fatalf("seeding the actions: %v", err)
	}

	if err := svc.DeleteGroup(ctx, admin, group.ID); !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("a system group was deleted: %v", err)
	}
	if _, err := svc.SetActions(ctx, admin, group.ID, nil); !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("a system group was stripped of its actions: %v", err)
	}

	// It still exists, carrying what it carried.
	held, err := svc.ActionsOf(ctx, admin, group.ID)
	if err != nil {
		t.Fatalf("reading what it carries: %v", err)
	}
	if len(held) != len(permission.Actions()) {
		t.Fatalf("the system group carries %d actions after a refused strip, want %d", len(held), len(permission.Actions()))
	}
}

// administered returns a database holding the group that administers
// permissions, with this person in it.
//
// It is what a seed produces, and it is what every screen below depends on: a
// subject reaches the panel because a group says so, and for no other reason.
func administered(t *testing.T, userID string) (*data.DB, *permission.PermissionService, security.Subject) {
	t.Helper()

	handle := store(t)
	catalogue, err := permission.NewCatalogue(settings().Actions...)
	if err != nil {
		t.Fatalf("building the catalogue: %v", err)
	}
	svc := permission.NewPermissionService(handle, catalogue)

	admin := operator("acme")
	group := seed(t, svc, admin, permission.CreateGroupRequest{
		Slug: "administrator", Name: "Administrator", System: true,
	})
	if _, err := svc.SetActions(context.Background(), admin, group.ID, permission.Actions()); err != nil {
		t.Fatalf("granting the administration: %v", err)
	}
	if _, err := svc.SetMembers(context.Background(), admin, group.ID, []string{userID}); err != nil {
		t.Fatalf("putting %s into the group: %v", userID, err)
	}
	return handle, svc, admin
}

// TestAMemberOfTheAdministeringGroupReachesThePanel is the other direction of
// every refusal below, and it is what makes them mean something.
//
// Nothing about this request carries a permission: the session holds an
// identifier and a tenant. What admits it is the middleware reading the group
// the person is in, which is the whole mechanism, end to end.
func TestAMemberOfTheAdministeringGroupReachesThePanel(t *testing.T) {
	handle, _, _ := administered(t, "user-1")
	router, cookie := signedIn(t, handle, "user-1")

	for _, target := range []string{
		permission.DefaultPrefix + "/groups",
		permission.DefaultPrefix + "/catalogue",
		permission.DefaultPrefix + "/matrix",
		permission.DefaultPrefix + "/users/user-1",
	} {
		rec := requestAs(t, router, cookie, http.MethodGet, target, "")
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s answered %d for a member of the administering group, want %d",
				target, rec.Code, http.StatusOK)
		}
	}
}

// TestTheFormCannotCreateASystemGroup keeps the one field the handler must not
// read out of a form from being readable out of one.
func TestTheFormCannotCreateASystemGroup(t *testing.T) {
	handle, svc, admin := administered(t, "user-1")
	router, cookie := signedIn(t, handle, "user-1")

	rec := requestAs(t, router, cookie, http.MethodPost, permission.DefaultPrefix+"/groups",
		"slug=sneaky&name=Sneaky&system=true&System=true&IsSystem=true&is_system=1")
	if rec.Code != http.StatusFound && rec.Code != http.StatusSeeOther {
		t.Fatalf("creating a group answered %d, want a redirect", rec.Code)
	}

	page, err := svc.ListGroups(context.Background(), admin, permission.GroupQuery{Search: "sneaky"})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("the form created %d groups, want one", len(page.Items))
	}

	// A group the form declared undeletable would be deletable by nobody. It is
	// not one, so it is.
	if err := svc.DeleteGroup(context.Background(), admin, page.Items[0].ID); err != nil {
		t.Fatalf("the group the form created is undeletable, so the form set the system flag: %v", err)
	}
}

// TestTheRouteRefusesWhatTheScreenWouldNotOffer is the verification that
// happens on the server.
//
// Hiding a control is a courtesy to whoever is looking at the page and it
// authorizes nothing. The request below is the one somebody makes with a
// terminal: signed in, in no group, straight at the address.
func TestTheRouteRefusesWhatTheScreenWouldNotOffer(t *testing.T) {
	t.Parallel()

	handle := store(t)
	router, cookie := signedIn(t, handle, "outsider")

	for _, request := range everyRequest(permission.DefaultPrefix) {
		rec := requestAs(t, router, cookie, request.method, request.target, request.body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s answered %d for a subject in no group, want %d",
				request.method, request.target, rec.Code, http.StatusForbidden)
		}
	}
}

// signedIn returns a router over this database and the session cookie of one
// person.
//
// The session is started through the store the module reads, so a request
// carrying the cookie goes through the same middleware an application's would:
// the subject arrives carrying nothing, and what it carries afterwards is
// whatever the groups say.
func signedIn(t *testing.T, handle *data.DB, userID string) (*fhttp.Router, string) {
	t.Helper()

	sessions := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	csrf := security.NewCSRF([]byte(appKey), time.Hour)
	module, err := permission.New(settings(), handle, sessions, csrf)
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	router := fhttp.NewRouter()
	module.Routes(router.WithRenderer(hview.NewRenderer()).ForModule(module.Name()))

	recorder := httptest.NewRecorder()
	if _, err := sessions.Start(context.Background(), recorder, security.Subject{ID: userID, Tenant: "acme"}); err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	cookie := recorder.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("the session store set no cookie, so the requests below would arrive as a visitor")
	}
	return router, strings.Split(cookie, ";")[0]
}

// The compiled views, as a published project would carry them.
//
// A view is a function registered by name from init(), and a test binary has no
// generated ones -- so the names are registered here with a renderer that
// writes nothing. What a screen draws is not what these tests are about; that
// it was reached, or refused, is.
func init() {
	for _, name := range permission.ViewNames() {
		hview.Register(name, func(io.Writer, any) error { return nil })
	}
}

// requestAs makes one request carrying a session cookie.
func requestAs(t *testing.T, router *fhttp.Router, cookie, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("Cookie", cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// holds reports whether a resolution carries an action.
func holds(actions []security.Action, action security.Action) bool {
	for _, have := range actions {
		if have == action {
			return true
		}
	}
	return false
}
