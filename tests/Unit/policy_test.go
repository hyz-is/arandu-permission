package unit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/database/model"

	permission "github.com/hyz-is/arandu-permission"
)

// The properties this package exists to keep, checked against the code rather
// than described in a document:
//
//  1. every policy denies a subject that carries nothing, and denies a record
//     belonging to another tenant;
//  2. nobody grants an action they do not hold;
//  3. nobody puts themselves into a group;
//  4. the service authorizes before it constructs or executes a model query;
//  5. the tenant comes from the Grant.
//
// The fourth is checked here by the handle these tests pass in. It wraps a nil
// *sql.DB, so any statement that were issued would panic and fail the test
// loudly -- which makes "the refusal happened before the model" a fact the suite
// proves rather than a comment. The structural twin in audit_test.go keeps that
// order visible on every service method, including an allowed path. The
// behaviour of all five against real rows is in tests/Feature.

// everyAction is the whole set the policies answer about. A test that listed
// nine of ten would pass while the tenth was open.
var everyAction = []security.Action{
	permission.PermissionView,
	permission.PermissionList,
	permission.PermissionCreate,
	permission.PermissionUpdate,
	permission.PermissionDelete,
	permission.PermissionGrant,
	permission.PermissionRevoke,
	permission.PermissionAssign,
	permission.PermissionUnassign,
	permission.PermissionResolve,
}

// carrier is a subject holding every action of this package, which is the most
// privileged one an application can produce here.
func carrier() security.Subject {
	roles := make([]string, 0, len(everyAction))
	for _, action := range everyAction {
		roles = append(roles, string(action))
	}
	return security.Subject{ID: "user-1", Tenant: "acme", Roles: roles, Verified: true}
}

// bare is a subject that has been through the middleware and came back with
// nothing, which is what everybody is until a group says otherwise.
func bare() security.Subject {
	return security.Subject{ID: "user-2", Tenant: "acme", Verified: true}
}

func TestThePolicyDeniesASubjectThatCarriesNothing(t *testing.T) {
	t.Parallel()

	for _, action := range everyAction {
		if action == permission.PermissionResolve {
			// Reading one's own memberships is decided by identity, and it is
			// the one question a subject carrying nothing has to be able to ask
			// -- it is what fills in everything else.
			continue
		}
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			_, err := security.Authorize(context.Background(), permission.GroupPolicy{},
				bare(), action, permission.Group{})
			if !errors.Is(err, security.ErrForbidden) {
				t.Fatalf("a subject carrying nothing was allowed %s: got %v, want ErrForbidden", action, err)
			}
		})
	}
}

func TestThePolicyDeniesAGroupOfAnotherTenant(t *testing.T) {
	t.Parallel()

	other := permission.Group{ID: "group-1", TenantID: "globex", Slug: "theirs"}

	err := permission.GroupPolicy{}.Can(context.Background(),
		carrier(), permission.PermissionView, other)
	if err == nil {
		t.Fatal("the policy allowed a group belonging to another tenant")
	}
	// The message is asserted because the tenant check is the one refusal that
	// has to survive somebody opening the actions below it.
	if !strings.Contains(err.Error(), "another tenant") {
		t.Fatalf("the refusal did not name the tenant: %v", err)
	}
}

func TestThePolicyDeniesAGuest(t *testing.T) {
	t.Parallel()

	for _, action := range everyAction {
		_, err := security.Authorize(context.Background(), permission.GroupPolicy{},
			security.Guest("acme"), action, permission.Group{})
		if !errors.Is(err, security.ErrForbidden) {
			t.Fatalf("a guest was allowed %s: got %v, want ErrForbidden", action, err)
		}
	}
}

func TestAuthorizeRefusesASubjectThatIsNobody(t *testing.T) {
	t.Parallel()

	// The zero Subject is a session that failed to load, not an anonymous
	// reader, and it is refused before the policy is consulted. A package that
	// answered it as a guest would answer a broken session as a visitor.
	_, err := security.Authorize(context.Background(), permission.GroupPolicy{},
		security.Subject{}, permission.PermissionView, permission.Group{})
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("an empty subject was authorized: got %v, want ErrForbidden", err)
	}
}

// TestASystemGroupIsNotDeletedAndIsNotStripped holds the half of the last
// administrator protection that a policy can see: the group in front of it is
// one the installation depends on.
func TestASystemGroupIsNotDeletedAndIsNotStripped(t *testing.T) {
	t.Parallel()

	system := permission.Group{ID: "group-1", TenantID: "acme", Slug: "administrator", IsSystem: true}

	for _, action := range []security.Action{permission.PermissionDelete, permission.PermissionRevoke} {
		if err := (permission.GroupPolicy{}).Can(context.Background(), carrier(), action, system); err == nil {
			t.Errorf("%s was allowed on a system group", action)
		}
	}
	// What it does not stop is renaming one, which changes nothing about who
	// carries what.
	if err := (permission.GroupPolicy{}).Can(context.Background(), carrier(), permission.PermissionUpdate, system); err != nil {
		t.Errorf("renaming a system group was refused: %v", err)
	}
}

// TestNobodyGrantsAnActionTheyDoNotHold is the self-elevation refusal, at the
// one place that can see both sides of it: the subject's own actions, and the
// action being handed out.
func TestNobodyGrantsAnActionTheyDoNotHold(t *testing.T) {
	t.Parallel()

	// This subject may administer permissions and does not hold invoice.delete.
	actor := carrier()
	link := permission.GroupAction{TenantID: "acme", GroupID: "group-1", Action: "invoice.delete"}

	if err := (permission.ActionPolicy{}).Can(context.Background(), actor, permission.PermissionGrant, link); err == nil {
		t.Fatal("a subject granted an action it does not hold")
	}

	// Holding it is the whole difference.
	holder := actor
	holder.Roles = append(append([]string(nil), actor.Roles...), "invoice.delete")
	if err := (permission.ActionPolicy{}).Can(context.Background(), holder, permission.PermissionGrant, link); err != nil {
		t.Fatalf("a subject that holds the action was refused: %v", err)
	}

	// Taking one away is not an escalation, so it is not bounded by what the
	// subject holds: a permission granted by mistake has to be removable by
	// somebody who never had it.
	if err := (permission.ActionPolicy{}).Can(context.Background(), actor, permission.PermissionRevoke, link); err != nil {
		t.Fatalf("revoking an action the subject does not hold was refused: %v", err)
	}
}

// TestNobodyPutsThemselvesIntoAGroup is the other half of self-elevation, and
// the shortest path to more power there is.
func TestNobodyPutsThemselvesIntoAGroup(t *testing.T) {
	t.Parallel()

	actor := carrier()
	own := permission.GroupUser{TenantID: "acme", GroupID: "group-1", UserID: actor.ID}
	other := permission.GroupUser{TenantID: "acme", GroupID: "group-1", UserID: "user-9"}

	if err := (permission.MembershipPolicy{}).Can(context.Background(), actor, permission.PermissionAssign, own); err == nil {
		t.Fatal("a subject put itself into a group")
	}
	if err := (permission.MembershipPolicy{}).Can(context.Background(), actor, permission.PermissionAssign, other); err != nil {
		t.Fatalf("putting somebody else into a group was refused: %v", err)
	}
	// Leaving one is not an escalation.
	if err := (permission.MembershipPolicy{}).Can(context.Background(), actor, permission.PermissionUnassign, own); err != nil {
		t.Fatalf("taking oneself out of a group was refused: %v", err)
	}
}

// TestResolvingAnswersAboutTheSubjectAndNobodyElse holds the exception that
// makes the rest possible, and holds its edge.
func TestResolvingAnswersAboutTheSubjectAndNobodyElse(t *testing.T) {
	t.Parallel()

	actor := bare()
	own := permission.GroupUser{TenantID: "acme", UserID: actor.ID}
	other := permission.GroupUser{TenantID: "acme", UserID: "user-9"}

	if err := (permission.MembershipPolicy{}).Can(context.Background(), actor, permission.PermissionResolve, own); err != nil {
		t.Fatalf("a subject was refused its own memberships: %v", err)
	}
	if err := (permission.MembershipPolicy{}).Can(context.Background(), actor, permission.PermissionResolve, other); err == nil {
		t.Fatal("a subject read somebody else's memberships through the resolution path")
	}

	// And it is still bounded by the tenant.
	foreign := permission.GroupUser{TenantID: "globex", UserID: actor.ID}
	if err := (permission.MembershipPolicy{}).Can(context.Background(), actor, permission.PermissionResolve, foreign); err == nil {
		t.Fatal("a subject resolved a membership of another tenant")
	}
}

// nilHandle is a handle over no database.
//
// Any statement issued through it panics, which is what makes these tests prove
// that the refusal came first: a service that reached a model before
// authorizing would crash here rather than pass.
func nilHandle() *data.DB { return data.Wrap(nil, data.DialectSQLite) }

// catalogue is the set these tests grant from: this package's own actions plus
// one an application declares.
func catalogue(t *testing.T) permission.Catalogue {
	t.Helper()
	c, err := permission.NewCatalogue(append(permission.Actions(), "invoice.delete")...)
	if err != nil {
		t.Fatalf("building the catalogue: %v", err)
	}
	return c
}

func TestTheServiceRefusesBeforeReachingTheModel(t *testing.T) {
	t.Parallel()

	// A nil handle makes even construction of a model panic at
	// GetQueryGrammar. This catches moving the configured model entry point --
	// not only its terminal -- ahead of authorization.
	service := permission.NewPermissionService(nil, catalogue(t))
	ctx := context.Background()
	actor := bare()

	if _, err := service.FindGroup(ctx, actor, "group-1"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("FindGroup reached the model before the policy refusal: %v", err)
	}
	if _, err := service.ListGroups(ctx, actor, permission.GroupQuery{}); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("ListGroups reached the model before the policy refusal: %v", err)
	}
	if _, err := service.CreateGroup(ctx, actor, permission.CreateGroupRequest{Slug: "one", Name: "One"}); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("CreateGroup reached the model before the policy refusal: %v", err)
	}
	if _, err := service.UpdateGroup(ctx, actor, "group-1", permission.UpdateGroupRequest{Name: "One"}); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("UpdateGroup reached the model before the policy refusal: %v", err)
	}
	if err := service.DeleteGroup(ctx, actor, "group-1"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("DeleteGroup reached the model before the policy refusal: %v", err)
	}
	if _, err := service.ActionsOf(ctx, actor, "group-1"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("ActionsOf reached the model before the policy refusal: %v", err)
	}
	if _, err := service.MembersOf(ctx, actor, "group-1"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("MembersOf reached the model before the policy refusal: %v", err)
	}
	if _, err := service.SetActions(ctx, actor, "group-1", nil); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("SetActions reached the model before the policy refusal: %v", err)
	}
	if _, err := service.SetMembers(ctx, actor, "group-1", nil); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("SetMembers reached the model before the policy refusal: %v", err)
	}
	if _, err := service.PreviewActions(ctx, actor, "group-1", nil); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("PreviewActions reached the model before the policy refusal: %v", err)
	}
	if _, err := service.PreviewMembers(ctx, actor, "group-1", nil); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("PreviewMembers reached the model before the policy refusal: %v", err)
	}
	if _, err := service.EffectiveFor(ctx, actor, "user-9"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("EffectiveFor reached the model before the policy refusal: %v", err)
	}
	if _, err := service.ViewCatalogue(ctx, actor); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("ViewCatalogue reached the model before the policy refusal: %v", err)
	}
	if _, err := service.ViewMatrix(ctx, actor, permission.GroupQuery{}); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("ViewMatrix reached the model before the policy refusal: %v", err)
	}

	// The resolution path is refused for a subject asking about nobody, which
	// is what an unloaded session looks like.
	if _, err := service.ResolveOwn(ctx, security.Subject{}); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("ResolveOwn reached the model for a subject that is nobody: %v", err)
	}
	if _, err := service.Version(ctx, security.Subject{}); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("Version reached the model for a subject that is nobody: %v", err)
	}
}

func TestTheModelsAreWiredAndTenantScoped(t *testing.T) {
	t.Parallel()

	handle := nilHandle()
	groups := permission.Groups(handle)
	if groups.GetTable() != permission.GroupsTable {
		t.Errorf("Groups table = %q, want %s", groups.GetTable(), permission.GroupsTable)
	}
	if groups.KeyType != "string" || groups.Incrementing {
		t.Errorf("Groups key is type %q, incrementing %t; want application-generated text", groups.KeyType, groups.Incrementing)
	}
	if model.ModelOf(groups.Entity) != groups {
		t.Error("Groups returned an entity whose embedded model is not wired to it")
	}

	// Every one of the four is scoped by tenant. A table declared global here
	// would be a table one customer reads another's rows from, and nothing else
	// in this package would say so.
	for name, column := range map[string]string{
		permission.GroupsTable:       permission.Groups(handle).TenantColumn,
		permission.GroupActionsTable: permission.GroupActions(handle).TenantColumn,
		permission.GroupUsersTable:   permission.GroupUsers(handle).TenantColumn,
		permission.VersionsTable:     permission.Versions(handle).TenantColumn,
	} {
		if column != "tenant_id" {
			t.Errorf("%s is scoped by %q, want tenant_id", name, column)
		}
	}

	// The link tables keep no timestamps, and the version table is keyed by the
	// tenant rather than by an identifier of its own.
	if permission.GroupActions(handle).Timestamps || permission.GroupUsers(handle).Timestamps {
		t.Error("a link table stamps timestamps, which is a column written and never read")
	}
	if key := permission.Versions(handle).PrimaryKey; key != "tenant_id" {
		t.Errorf("Versions is keyed by %q, want tenant_id: one row per customer", key)
	}
}

func TestASystemGrantWithoutATenantReachesNothing(t *testing.T) {
	t.Parallel()

	// A system grant with no tenant names no customer. The model refuses it
	// while preparing the query, before the nil handle can issue a statement.
	_, err := permission.Groups(nilHandle()).NewQuery().WhereKey("group-1").First(
		context.Background(), security.SystemGrant(permission.PermissionView, ""))
	if !errors.Is(err, model.ErrNoTenant) {
		t.Fatalf("a system grant with no tenant returned %v, want ErrNoTenant", err)
	}
}

func TestTheTenantComesFromTheGrant(t *testing.T) {
	t.Parallel()

	g := security.SystemGrant(permission.PermissionView, "acme")
	if got := data.Tenant(g); got != "acme" {
		t.Fatalf("data.Tenant(g) = %q, want %q", got, "acme")
	}

	// And a Grant nobody issued carries no tenant at all, so a statement that
	// took its tenant from anywhere else would be reading rows this Grant does
	// not name.
	if got := data.Tenant(security.Grant{}); got != "" {
		t.Fatalf("the zero Grant carries the tenant %q, want none", got)
	}
}

func TestTheCatalogueIsClosedAndGrouped(t *testing.T) {
	t.Parallel()

	c := catalogue(t)
	if !c.Has(permission.PermissionGrant) {
		t.Error("the catalogue does not hold an action it was built from")
	}
	if c.Has("invoice.forge") {
		t.Error("the catalogue holds an action nobody declared")
	}

	domains := c.Domains()
	if len(domains) != 2 {
		t.Fatalf("the catalogue has %d domains, want 2: invoice and permission", len(domains))
	}
	if domains[0].Name != "invoice" || domains[1].Name != "permission" {
		t.Fatalf("the domains are %q and %q, want invoice then permission", domains[0].Name, domains[1].Name)
	}

	// A copy, so that a screen sorting the list does not change the set every
	// write is checked against.
	all := c.All()
	if len(all) > 0 {
		all[0] = "mutated"
		if c.All()[0] == "mutated" {
			t.Error("All handed out the catalogue's own slice")
		}
	}
}

func TestTheCatalogueRefusesWhatIsNotAnAction(t *testing.T) {
	t.Parallel()

	for name, actions := range map[string][]security.Action{
		"nothing at all":  {},
		"an empty action": {""},
		"no domain":       {"delete"},
		"an empty domain": {".delete"},
		"an empty verb":   {"invoice."},
		"whitespace":      {"invoice delete"},
		"uppercase":       {"Invoice.Delete"},
	} {
		if _, err := permission.NewCatalogue(actions...); err == nil {
			t.Errorf("the catalogue accepted %s", name)
		}
	}

	// Repeats are collapsed rather than refused: splicing two lists together is
	// what an application does, and refusing that would make the caller
	// deduplicate a set this returns deduplicated anyway.
	c, err := permission.NewCatalogue("invoice.delete", "invoice.delete")
	if err != nil {
		t.Fatalf("the catalogue refused a repeat: %v", err)
	}
	if c.Len() != 1 {
		t.Fatalf("the catalogue holds %d actions after one repeat, want 1", c.Len())
	}
}

func TestTheRequestsValidateTheirInput(t *testing.T) {
	t.Parallel()

	if errs := (permission.CreateGroupRequest{}).Validate(); !errs.Any() {
		t.Error("an empty create request validated")
	}
	if errs := (permission.CreateGroupRequest{Slug: "Editors", Name: "Editors"}).Validate(); !errs.Any() {
		t.Error("a slug with an uppercase letter validated")
	}
	if errs := (permission.CreateGroupRequest{Slug: "a b", Name: "Editors"}).Validate(); !errs.Any() {
		t.Error("a slug with a space validated")
	}
	if errs := (permission.CreateGroupRequest{
		Slug: strings.Repeat("a", permission.MaxSlugLength+1), Name: "Editors",
	}).Validate(); !errs.Any() {
		t.Error("a slug past the maximum validated")
	}
	if errs := (permission.CreateGroupRequest{Slug: "editors", Name: "Editors"}).Validate(); errs.Any() {
		t.Errorf("a valid create request was rejected: %v", errs)
	}

	if errs := (permission.UpdateGroupRequest{}).Validate(); !errs.Any() {
		t.Error("an empty update request validated")
	}
	if errs := (permission.UpdateGroupRequest{Name: "Editors"}).Validate(); errs.Any() {
		t.Errorf("a valid update request was rejected: %v", errs)
	}
}

func TestTheConfigurationRefusesWhatCannotWork(t *testing.T) {
	t.Parallel()

	full := permission.Actions()

	for name, cfg := range map[string]permission.Config{
		"no tenant":          {Actions: full},
		"tenant with a /":    {Tenant: "acme/reports", Actions: full},
		"tenant uppercase":   {Tenant: "Acme", Actions: full},
		"no catalogue":       {Tenant: "acme"},
		"relative prefix":    {Tenant: "acme", Actions: full, Prefix: "permission"},
		"page size too big":  {Tenant: "acme", Actions: full, PageSize: permission.MaxPageSize + 1},
		"negative page size": {Tenant: "acme", Actions: full, PageSize: -1},
		"negative cache":     {Tenant: "acme", Actions: full, CacheSize: -1},
		"catalogue without this package": {
			Tenant:  "acme",
			Actions: []security.Action{"invoice.delete"},
		},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("the configuration with %s was accepted", name)
		}
	}

	if err := (permission.Config{Tenant: "acme", Actions: full}).Validate(); err != nil {
		t.Fatalf("a valid configuration was refused: %v", err)
	}
}

// TestTheResolvableActionIsNotGrantable holds the one action the catalogue
// deliberately leaves out.
//
// It is decided by identity, so attaching it to a group would change nothing --
// and a screen offering a permission that changes nothing is a screen nobody
// can reason about.
func TestTheResolvableActionIsNotGrantable(t *testing.T) {
	t.Parallel()

	for _, action := range permission.Actions() {
		if action == permission.PermissionResolve {
			t.Fatal("permission.resolve is offered as grantable, and granting it grants nothing")
		}
	}
}
