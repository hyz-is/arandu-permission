package conformance

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/framework/test"
	"github.com/arandu-io/hesape/auth"

	permission "github.com/hyz-is/arandu-permission"
)

// Fixture is one application, booted over a fresh database, as the kit sees
// it. The application builds it from its own composition, once per subtest.
type Fixture struct {
	// Server is the application framework/test Serve booted. The kit builds
	// addresses from its route names and signs people in on it with As.
	Server *test.Server

	// Handler is the handler the same boot returned in test.Application. The
	// kit sends its requests through it, in process, carrying the session As
	// started, so every request runs the application's whole pipeline.
	Handler http.Handler

	// Module is the permission module that boot registered: the same value,
	// so the groups the kit writes through its service are the rows the
	// application's requests read.
	Module *permission.Module

	// Tenants are two organizations of the application, each with no group
	// yet. The kit writes into both and expects neither to see the other.
	Tenants [2]string

	// Account creates an account the application signs in, as a member of
	// tenant, and returns who it is. Only ID and Tenant are read: roles and
	// actions are what the application's resolver and this module answer.
	Account func(t *testing.T, tenant string) auth.Subject

	// Join makes an existing account a member of another tenant, and returns
	// who it signs in as there. Nil is an application whose accounts belong to
	// one tenant; the kit then proves the same thing with an identifier that
	// has rows in both, which it writes itself.
	Join func(t *testing.T, account auth.Subject, tenant string) auth.Subject

	// Guarded is a route of the application's own that answers only a
	// subject carrying an action this module grants.
	Guarded Guarded
}

// Guarded names a GET route of the application and the action it requires.
//
// It is how the kit sees what the application's guards see. A route behind
// permissionModule.Require(action), or a handler whose policy decides on
// subject.Can(action), both qualify; the kit expects a 2xx for a subject that
// carries the action and a 403 or a 404 for one that does not.
type Guarded struct {
	// Route is the route's name, as Server.Route builds an address from it.
	Route string
	// Params fill the route's parameters, in order.
	Params []string
	// Action is the action the route requires. It has to be in the catalogue
	// the application handed permission.New, and it cannot be one of this
	// module's own.
	Action security.Action
}

// Permissions proves the invariants Arandu Permission promises against the
// application open builds, once per subtest:
//
//   - AGroupsActionReachesTheApplicationsGuard: the Roles middleware runs
//     before the application's guard, which decides on what it resolved;
//   - AnActionStaysInsideItsOrganization: the actions are read for the tenant
//     of the session, never for one the request names;
//   - NobodyGrantsWhatTheyDoNotHold: no grant beyond what the actor holds,
//     no self-assignment, no permission given to oneself, and no write when
//     any of them is refused;
//   - ARevokedActionIsGoneOnTheNextRequest: a group losing the action, the
//     person leaving the group and a direct grant taken back all end the
//     access on the next request of the same session;
//   - ReadingNeedsAGrantOfTheTenant: every screen refuses a visitor and an
//     account with no permission, and a Grant of one tenant reads nothing of
//     another.
//
// A failure is a sentence that says what the application broke.
func Permissions(t *testing.T, open func(t *testing.T) Fixture) {
	t.Helper()
	if open == nil {
		t.Fatal("conformance.Permissions was handed no fixture: pass the function that serves the application and returns its Fixture")
	}
	t.Run("AGroupsActionReachesTheApplicationsGuard", func(t *testing.T) { newWorld(t, open(t)).actionsReachTheGuard() })
	t.Run("AnActionStaysInsideItsOrganization", func(t *testing.T) { newWorld(t, open(t)).actionsStayInTheirOrganization() })
	t.Run("NobodyGrantsWhatTheyDoNotHold", func(t *testing.T) { newWorld(t, open(t)).nobodyEscalates() })
	t.Run("ARevokedActionIsGoneOnTheNextRequest", func(t *testing.T) { newWorld(t, open(t)).revocationIsImmediate() })
	t.Run("ReadingNeedsAGrantOfTheTenant", func(t *testing.T) { newWorld(t, open(t)).readingNeedsAGrant() })
}

// world is one fixture with the two administrators the kit made in it.
type world struct {
	t   *testing.T
	f   Fixture
	ctx context.Context
	svc *permission.PermissionService

	// roots are the accounts the kit made administrators, one per tenant,
	// and operators are the same accounts carrying what the module resolves
	// for them: the subject a request of theirs would carry into a policy.
	roots     [2]auth.Subject
	operators [2]security.Subject
}

// The slugs of the groups the kit writes.
const (
	rootSlug      = "conformance-root"
	carriersSlug  = "conformance-carriers"
	delegatesSlug = "conformance-delegates"
	editorsSlug   = "conformance-editors"
)

// newWorld checks the fixture and makes one administrator in each tenant,
// through the door an installation's first group is made through.
func newWorld(t *testing.T, f Fixture) *world {
	t.Helper()
	switch {
	case f.Server == nil:
		t.Fatal("conformance.Fixture.Server is nil: serve the application with test.Serve and hand the server over")
	case f.Handler == nil:
		t.Fatal("conformance.Fixture.Handler is nil: hand over the Handler of the test.Application the boot returned")
	case f.Module == nil:
		t.Fatal("conformance.Fixture.Module is nil: hand over the permission module the boot registered, not a second one")
	case f.Account == nil:
		t.Fatal("conformance.Fixture.Account is nil: the kit has to create accounts the application signs in")
	case f.Guarded.Route == "" || f.Guarded.Action == "":
		t.Fatal("conformance.Fixture.Guarded needs Route and Action: a route of the application that requires an action this module grants")
	case !security.ValidTenant(f.Tenants[0]) || !security.ValidTenant(f.Tenants[1]) || f.Tenants[0] == f.Tenants[1]:
		t.Fatalf("conformance.Fixture.Tenants are %q, and have to be two different tenants", f.Tenants)
	}
	if slices.Contains(permission.Actions(), f.Guarded.Action) {
		t.Fatalf("the guarded route requires %s, which is this module's own action; name an action of the application", f.Guarded.Action)
	}

	w := &world{t: t, f: f, ctx: t.Context(), svc: f.Module.Service()}
	if !w.svc.Catalogue().Has(f.Guarded.Action) {
		t.Fatalf("the guarded route requires %s, which is not in the catalogue the application handed permission.New: add it to Config.Actions, or no group can carry it", f.Guarded.Action)
	}
	for i, tenant := range f.Tenants {
		root := w.account(tenant)
		_, err := w.svc.Bootstrap(w.ctx, tenant, permission.BootstrapRequest{
			Slug:    rootSlug,
			Name:    "Conformance root",
			Actions: []security.Action{f.Guarded.Action},
			Members: []string{root.ID},
		})
		if errors.Is(err, permission.ErrBootstrapped) {
			t.Fatalf("the tenant %s already has a group when the application has just booted: the kit needs both tenants empty, so seed groups outside the boot the fixture serves", tenant)
		}
		if err != nil {
			t.Fatalf("making the first group of %s: %v", tenant, err)
		}
		w.roots[i] = root
		w.operators[i] = w.resolved(root)
	}
	return w
}

// account creates an account in tenant and keeps only who it is.
func (w *world) account(tenant string) auth.Subject {
	w.t.Helper()
	made := w.f.Account(w.t, tenant)
	if made.ID == "" || made.Tenant != tenant {
		w.t.Fatalf("conformance.Fixture.Account(%q) returned an account %q of tenant %q: it has to create an account of the tenant it was asked for", tenant, made.ID, made.Tenant)
	}
	return auth.Subject{ID: made.ID, Tenant: made.Tenant}
}

// resolved is the account carrying what the module resolves for it, which is
// what the Roles middleware would put on a request of theirs.
func (w *world) resolved(who auth.Subject) security.Subject {
	w.t.Helper()
	resolution, err := w.svc.ResolveOwn(w.ctx, security.Subject{ID: who.ID, Tenant: who.Tenant})
	if err != nil {
		w.t.Fatalf("resolving what %s carries in %s: %v", who.ID, who.Tenant, err)
	}
	return security.Subject{ID: who.ID, Tenant: who.Tenant, Actions: resolution.Actions, Verified: true}
}

// group creates a group in the tenant at index, carrying actions and holding
// members, as that tenant's administrator.
func (w *world) group(tenant int, slug string, actions []security.Action, members []string) *permission.Group {
	w.t.Helper()
	created, err := w.svc.CreateGroup(w.ctx, w.operators[tenant], permission.CreateGroupRequest{Slug: slug, Name: slug})
	if err != nil {
		w.t.Fatalf("creating the group %s in %s: %v", slug, w.f.Tenants[tenant], err)
	}
	w.setActions(tenant, created, actions)
	w.setMembers(tenant, created, members)
	return created
}

// setActions makes the group carry exactly actions.
func (w *world) setActions(tenant int, g *permission.Group, actions []security.Action) {
	w.t.Helper()
	if _, err := w.svc.SetActions(w.ctx, w.operators[tenant], g.ID, actions); err != nil {
		w.t.Fatalf("making %s of %s carry %v: %v", g.Slug, w.f.Tenants[tenant], actions, err)
	}
}

// setMembers makes the group hold exactly members.
func (w *world) setMembers(tenant int, g *permission.Group, members []string) {
	w.t.Helper()
	if _, err := w.svc.SetMembers(w.ctx, w.operators[tenant], g.ID, members); err != nil {
		w.t.Fatalf("putting %v into %s of %s: %v", members, g.Slug, w.f.Tenants[tenant], err)
	}
}

// setDirect makes the person carry exactly actions in their own right.
func (w *world) setDirect(tenant int, userID string, actions []security.Action) {
	w.t.Helper()
	if _, err := w.svc.SetDirectActions(w.ctx, w.operators[tenant], userID, actions); err != nil {
		w.t.Fatalf("giving %s %v in %s: %v", userID, actions, w.f.Tenants[tenant], err)
	}
}

// guarded is the address of the application's guarded route.
func (w *world) guarded() string {
	w.t.Helper()
	return w.f.Server.Route(w.f.Guarded.Route, w.f.Guarded.Params...)
}

// route is the address of one of the module's routes.
func (w *world) route(name string, params ...string) string {
	w.t.Helper()
	return w.f.Server.Route(name, params...)
}

// actionsReachTheGuard is the direction every refusal below is measured
// against: a member of a group carrying the action passes the application's
// guard, and an account in no group does not.
func (w *world) actionsReachTheGuard() {
	t, action := w.t, w.f.Guarded.Action
	tenant := w.f.Tenants[0]
	member, outsider := w.account(tenant), w.account(tenant)
	w.group(0, carriersSlug, []security.Action{action}, []string{member.ID})

	if code := w.signIn(member).get(w.guarded()); !succeeded(code) {
		t.Fatalf("a member of a group carrying %s was answered %d by %s, which requires it: the application's guard does not see the actions this module resolves. Mount permissionModule.Roles() after the session is loaded and before the guard, and decide on subject.Can(%s)",
			action, code, w.f.Guarded.Route, action)
	}
	if code := w.signIn(outsider).get(w.guarded()); !refused(code) {
		t.Errorf("an account in no group was answered %d by %s, which requires %s: the guard admits a subject that carries nothing",
			code, w.f.Guarded.Route, action)
	}
	if code := w.signIn(member).get(w.route("permission.index")); code != http.StatusForbidden {
		t.Errorf("a member whose groups carry %s and not permission.list was answered %d by permission.index, want 403: the module's screens see actions the groups do not grant",
			action, code)
	}
}

// actionsStayInTheirOrganization holds the tenant to the session's: rows of
// another tenant written for the same identifier never reach the subject, and
// nothing the request names moves it there.
func (w *world) actionsStayInTheirOrganization() {
	t, action := w.t, w.f.Guarded.Action
	home, other := w.f.Tenants[0], w.f.Tenants[1]
	member, stranger := w.account(home), w.account(other)
	homeGroup := w.group(0, carriersSlug, []security.Action{action}, []string{member.ID})
	otherGroup := w.group(1, carriersSlug, []security.Action{action}, nil)

	signedIn := w.signIn(member)
	if code := signedIn.get(w.guarded()); !succeeded(code) {
		t.Fatalf("a member of %s's group carrying %s was answered %d by %s: run AGroupsActionReachesTheApplicationsGuard first, everything here is measured against that answer",
			home, action, code, w.f.Guarded.Route)
	}
	if code := w.signIn(stranger).get(w.guarded()); !refused(code) {
		t.Errorf("an account of %s in no group there was answered %d by %s while a group of %s carries %s: an action granted in one organization reached a subject of another",
			other, code, w.f.Guarded.Route, home, action)
	}

	// The same identifier is now a member of the other tenant's group and of
	// no group at home. Signed in at home, it carries nothing.
	w.setMembers(1, otherGroup, []string{member.ID})
	w.setMembers(0, homeGroup, nil)
	if code := signedIn.get(w.guarded()); !refused(code) {
		t.Errorf("%s, a member of a group of %s carrying %s and of none in %s, was answered %d by %s while signed into %s: the actions were read for a tenant other than the session's",
			member.ID, other, action, home, code, w.f.Guarded.Route, home)
	}
	for _, hint := range tenantHints {
		if code := signedIn.withHint(hint, other).get(w.guarded()); !refused(code) {
			t.Errorf("naming %s in the %s %s admitted %s, signed into %s, to %s (answered %d): the tenant was taken from the request, which only the session may decide",
				other, hint.name, hint.where, member.ID, home, w.f.Guarded.Route, code)
		}
	}

	// The other tenant's group does reach a subject signed into that tenant,
	// so the refusals above are about the tenant and not about the group.
	w.setMembers(1, otherGroup, []string{member.ID, stranger.ID})
	if code := w.signIn(stranger).get(w.guarded()); !succeeded(code) {
		t.Fatalf("a member of %s's group carrying %s, signed into %s, was answered %d by %s: the application does not read the actions of a subject of its second tenant",
			other, action, other, code, w.f.Guarded.Route)
	}

	if w.f.Join == nil {
		return
	}
	joined := w.f.Join(t, member, other)
	if joined.ID == "" || joined.Tenant != other {
		t.Fatalf("conformance.Fixture.Join returned %q of tenant %q for %s joining %s: it has to return who the account signs in as there",
			joined.ID, joined.Tenant, member.ID, other)
	}
	joined = auth.Subject{ID: joined.ID, Tenant: joined.Tenant}
	w.setMembers(1, otherGroup, []string{member.ID, joined.ID, stranger.ID})
	if code := w.signIn(joined).get(w.guarded()); !succeeded(code) {
		t.Errorf("%s, signed into %s where a group carries %s for them, was answered %d by %s: the organization the session selected is not the one the actions are read for",
			joined.ID, other, action, code, w.f.Guarded.Route)
	}
	if code := signedIn.get(w.guarded()); !refused(code) {
		t.Errorf("%s, signed into %s where no group carries %s for them, was answered %d by %s after signing into %s too: one session's organization leaked into the other",
			member.ID, home, action, code, w.f.Guarded.Route, other)
	}
}

// nobodyEscalates holds the three refusals a delegate who may edit groups
// meets: granting what they do not hold, putting themselves into a group, and
// giving themselves a permission. Each is measured against the same write made
// with what they do hold, and none of them may write anything.
func (w *world) nobodyEscalates() {
	t, action := w.t, w.f.Guarded.Action
	delegate, colleague := w.account(w.f.Tenants[0]), w.account(w.f.Tenants[0])
	w.group(0, delegatesSlug, []security.Action{
		permission.PermissionView, permission.PermissionList, permission.PermissionGrant,
		permission.PermissionAssign, permission.PermissionGrantDirect,
	}, []string{delegate.ID})
	editors := w.group(0, editorsSlug, nil, nil)
	held := permission.PermissionView
	browser := w.signIn(delegate)

	groupPage := w.route("permission.show", editors.ID)
	write := func(target, page string, values ...string) int {
		t.Helper()
		if code := browser.get(page); !succeeded(code) {
			t.Fatalf("%s, who carries permission.view, was answered %d by %s, the screen the write is made from", delegate.ID, code, page)
		}
		return browser.put(target, values)
	}
	control := func(code int, what string) {
		t.Helper()
		if !wrote(code) {
			t.Fatalf("%s was answered %d when %s, which they may: every refusal below is compared against this write. A CSRF refusal means the screen drew no _token for its form; a 403 means the module's routes do not see what Roles resolved",
				delegate.ID, code, what)
		}
	}

	// Granting.
	control(write(w.route("permission.grant", editors.ID), groupPage, string(held)),
		"making "+editorsSlug+" carry "+string(held))
	version := w.version()
	if code := write(w.route("permission.grant", editors.ID), groupPage, string(held), string(action)); code != http.StatusForbidden {
		t.Errorf("%s, who does not hold %s, was answered %d when making %s carry it, want 403: nobody hands out what they do not hold",
			delegate.ID, action, code, editorsSlug)
	}
	w.unchanged(version, "the refused grant")
	if got := w.actionsOf(editors); !slices.Equal(got, []security.Action{held}) {
		t.Errorf("after the refused grant %s carries %v, want [%s]: a refused grant wrote", editorsSlug, got, held)
	}

	// Assigning.
	control(write(w.route("permission.assign", editors.ID), groupPage, colleague.ID),
		"putting "+colleague.ID+" into "+editorsSlug)
	version = w.version()
	if code := write(w.route("permission.assign", editors.ID), groupPage, colleague.ID, delegate.ID); code != http.StatusForbidden {
		t.Errorf("%s was answered %d when putting themselves into %s, want 403: nobody puts themselves into a group",
			delegate.ID, code, editorsSlug)
	}
	w.unchanged(version, "the refused assignment")
	if got := w.membersOf(editors); !slices.Equal(got, []string{colleague.ID}) {
		t.Errorf("after the refused assignment %s holds %v, want [%s]: a refused assignment wrote", editorsSlug, got, colleague.ID)
	}

	// Giving in one's own right.
	colleaguePage := w.route("permission.member", colleague.ID)
	control(write(w.route("permission.member.grant", colleague.ID), colleaguePage, string(held)),
		"giving "+colleague.ID+" "+string(held))
	version = w.version()
	if code := write(w.route("permission.member.grant", delegate.ID), w.route("permission.member", delegate.ID), string(held)); code != http.StatusForbidden {
		t.Errorf("%s was answered %d when giving themselves %s, want 403: nobody gives a permission to themselves",
			delegate.ID, code, held)
	}
	if code := write(w.route("permission.member.grant", colleague.ID), colleaguePage, string(held), string(action)); code != http.StatusForbidden {
		t.Errorf("%s, who does not hold %s, was answered %d when giving it to %s, want 403: nobody hands out what they do not hold",
			delegate.ID, action, code, colleague.ID)
	}
	w.unchanged(version, "the refused grants in one's own right")
	if got := w.directOf(delegate.ID); len(got) != 0 {
		t.Errorf("after the refused grant to themselves %s carries %v in their own right, want nothing", delegate.ID, got)
	}
	if got := w.directOf(colleague.ID); !slices.Equal(got, []security.Action{held}) {
		t.Errorf("after the refused grant %s carries %v in their own right, want [%s]", colleague.ID, got, held)
	}

	for _, who := range []auth.Subject{delegate, colleague} {
		if code := w.signIn(who).get(w.guarded()); !refused(code) {
			t.Errorf("%s was answered %d by %s after the refused writes: one of them reached the guard's action", who.ID, code, w.f.Guarded.Route)
		}
	}
}

// revocationIsImmediate takes the action away three ways and asks again on
// the same session each time.
func (w *world) revocationIsImmediate() {
	t, action := w.t, w.f.Guarded.Action
	member := w.account(w.f.Tenants[0])
	carriers := w.group(0, carriersSlug, []security.Action{action}, []string{member.ID})
	browser := w.signIn(member)

	if code := browser.get(w.guarded()); !succeeded(code) {
		t.Fatalf("a member of a group carrying %s was answered %d by %s: run AGroupsActionReachesTheApplicationsGuard first, everything here is measured against that answer",
			action, code, w.f.Guarded.Route)
	}
	remembered := "something between the database and the guard remembers actions: the session, a cache, or a resolver that does not read the tenant's permission token"

	w.setActions(0, carriers, nil)
	if code := browser.get(w.guarded()); !refused(code) {
		t.Errorf("after %s was taken off %s, the next request of its member was answered %d by %s: %s", action, carriersSlug, code, w.f.Guarded.Route, remembered)
	}

	w.setActions(0, carriers, []security.Action{action})
	if code := browser.get(w.guarded()); !succeeded(code) {
		t.Fatalf("after %s was given back to %s, the next request of its member was answered %d by %s: a grant does not reach a session that is already open", action, carriersSlug, code, w.f.Guarded.Route)
	}
	w.setMembers(0, carriers, nil)
	if code := browser.get(w.guarded()); !refused(code) {
		t.Errorf("after %s left %s, their next request was answered %d by %s: %s", member.ID, carriersSlug, code, w.f.Guarded.Route, remembered)
	}

	w.setDirect(0, member.ID, []security.Action{action})
	if code := browser.get(w.guarded()); !succeeded(code) {
		t.Fatalf("after %s was given %s in their own right, their next request was answered %d by %s: a direct grant does not reach the guard", member.ID, action, code, w.f.Guarded.Route)
	}
	w.setDirect(0, member.ID, nil)
	if code := browser.get(w.guarded()); !refused(code) {
		t.Errorf("after the direct grant of %s to %s was taken back, their next request was answered %d by %s: %s", action, member.ID, code, w.f.Guarded.Route, remembered)
	}
}

// readingNeedsAGrant walks every screen of the module: a visitor and an
// account with no permission are turned away, the administrator of the tenant
// reads them, and the administrator of the other tenant reads nothing of it.
func (w *world) readingNeedsAGrant() {
	t := w.t
	home, other := w.f.Tenants[0], w.f.Tenants[1]
	member := w.account(home)
	homeGroup := w.group(0, carriersSlug, []security.Action{w.f.Guarded.Action}, []string{member.ID})

	screens := []struct{ name, address string }{
		{"permission.index", w.route("permission.index")},
		{"permission.catalogue", w.route("permission.catalogue")},
		{"permission.matrix", w.route("permission.matrix")},
		{"permission.members", w.route("permission.members")},
		{"permission.show", w.route("permission.show", homeGroup.ID)},
		{"permission.member", w.route("permission.member", member.ID)},
	}

	administrator := w.signIn(w.roots[0])
	for _, screen := range screens {
		if code := administrator.get(screen.address); !succeeded(code) {
			t.Fatalf("the administrator of %s was answered %d by %s: every refusal below is compared against this answer, and a screen that refuses everybody proves nothing",
				home, code, screen.name)
		}
	}

	visitor, unprivileged := w.guest(), w.signIn(member)
	for _, screen := range screens {
		if code := visitor.get(screen.address); !turnedAway(code) {
			t.Errorf("a visitor with no session was answered %d by %s: reading the permissions needs a Grant, and a visitor has none", code, screen.name)
		}
		if code := unprivileged.get(screen.address); !refused(code) {
			t.Errorf("%s, whose groups carry neither permission.list nor permission.view, was answered %d by %s: a read was answered without the Grant of the action it needs",
				member.ID, code, screen.name)
		}
	}
	if code := visitor.get(w.guarded()); !turnedAway(code) {
		t.Errorf("a visitor with no session was answered %d by %s, which requires %s", code, w.f.Guarded.Route, w.f.Guarded.Action)
	}

	stranger := w.signIn(w.roots[1])
	if code := stranger.get(w.route("permission.index")); !succeeded(code) {
		t.Fatalf("the administrator of %s was answered %d by permission.index: the refusal below would prove nothing", other, code)
	}
	group := w.route("permission.show", homeGroup.ID)
	if code := stranger.get(group); !refused(code) {
		t.Errorf("the administrator of %s was answered %d by permission.show for a group of %s: a Grant of one tenant read another tenant's rows", other, code, home)
	}
	for _, hint := range tenantHints {
		if code := stranger.withHint(hint, home).get(group); !refused(code) {
			t.Errorf("the administrator of %s, naming %s in the %s %s, was answered %d by permission.show for a group of %s: the tenant was taken from the request",
				other, home, hint.name, hint.where, code, home)
		}
	}
}

// version is the home tenant's permission token, which every write moves.
func (w *world) version() int64 {
	w.t.Helper()
	v, err := w.svc.Version(w.ctx, w.operators[0])
	if err != nil {
		w.t.Fatalf("reading the permission token of %s: %v", w.f.Tenants[0], err)
	}
	return v
}

// actionsOf is what a group of the home tenant carries.
func (w *world) actionsOf(g *permission.Group) []security.Action {
	w.t.Helper()
	got, err := w.svc.ActionsOf(w.ctx, w.operators[0], g.ID)
	if err != nil {
		w.t.Fatalf("reading what %s carries: %v", g.Slug, err)
	}
	return got
}

// membersOf is who a group of the home tenant holds.
func (w *world) membersOf(g *permission.Group) []string {
	w.t.Helper()
	got, err := w.svc.MembersOf(w.ctx, w.operators[0], g.ID)
	if err != nil {
		w.t.Fatalf("reading who %s holds: %v", g.Slug, err)
	}
	return got
}

// directOf is what a person of the home tenant carries in their own right.
func (w *world) directOf(userID string) []security.Action {
	w.t.Helper()
	got, err := w.svc.DirectActionsOf(w.ctx, w.operators[0], userID)
	if err != nil {
		w.t.Fatalf("reading what %s carries in their own right: %v", userID, err)
	}
	return got
}

// succeeded is the answer to a read that went through.
func succeeded(code int) bool { return code >= 200 && code < 300 }

// wrote is the answer to a write that went through: the module redirects to
// the screen it changed.
func wrote(code int) bool { return code >= 200 && code < 400 }

// refused is the answer to a signed-in subject a policy said no to. A 404 is
// how a read scoped to the tenant answers a row of another, and some guards
// answer it so as not to say the route exists.
func refused(code int) bool { return code == http.StatusForbidden || code == http.StatusNotFound }

// turnedAway is the answer to a visitor: refused, asked to sign in, or sent to
// the sign-in screen.
func turnedAway(code int) bool {
	return refused(code) || code == http.StatusUnauthorized || (code >= 300 && code < 400)
}

// unchanged fails when the home tenant's permission token moved since before,
// which is what any write that could change a decision does.
func (w *world) unchanged(before int64, what string) {
	w.t.Helper()
	if now := w.version(); now != before {
		w.t.Errorf("the tenant's permission token moved from %d to %d across %s: something was written for a request the policy refused", before, now, what)
	}
}
