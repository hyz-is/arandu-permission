package feature_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arandu-io/framework/security"

	permission "github.com/hyz-is/arandu-permission"
)

// What a permission given to one person, outside every group, must not become.
//
// It is the shortest path from "may administer permissions" to "may do
// anything": no group is created, nothing is named after a role, and nobody
// reading a list of groups would see it. So the two refusals that bound the
// group path are asked harder here, and the four properties of the package are
// asked again against these rows rather than assumed to carry over.
//
//	TestNobodyGivesThemselvesAPermission               self-elevation
//	TestNobodyGivesAwayAPermissionTheyDoNotHold        self-elevation
//	TestADirectGrantOfOneTenantIsInvisibleToAnother    cross-tenant access
//	TestARevokedDirectGrantIsNotServedFromTheResolver  stale cache

// TestNobodyGivesThemselvesAPermission is the refusal that has no equivalent in
// the reference, and it is the one that matters most.
//
// The subject may administer permissions in every way this package declares. If
// it could write itself a row, the whole of the escalation defence would be one
// request wide.
func TestNobodyGivesThemselvesAPermission(t *testing.T) {
	t.Parallel()

	svc := service(t)
	actor := operator("acme", "invoice.delete")

	_, err := svc.SetDirectActions(context.Background(), actor, actor.ID, []security.Action{"invoice.delete"})
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("giving oneself a permission = %v, want ErrForbidden", err)
	}

	held, err := svc.DirectActionsOf(context.Background(), actor, actor.ID)
	if err != nil {
		t.Fatalf("reading the direct grants: %v", err)
	}
	if len(held) != 0 {
		t.Errorf("the subject carries %v after a refused write", held)
	}
}

// TestNobodyGivesAwayAPermissionTheyDoNotHold is the group rule, applied to the
// path that has no group in it.
func TestNobodyGivesAwayAPermissionTheyDoNotHold(t *testing.T) {
	t.Parallel()

	svc := service(t)
	// This actor administers permissions and does not hold invoice.delete.
	actor := operator("acme")

	_, err := svc.SetDirectActions(context.Background(), actor, "someone-else", []security.Action{"invoice.delete"})
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("handing out an unheld permission = %v, want ErrForbidden", err)
	}

	// The same actor holding it may hand it out, which is what makes the
	// refusal above a rule rather than a wall.
	holder := operator("acme", "invoice.delete")
	change, err := svc.SetDirectActions(context.Background(), holder, "someone-else", []security.Action{"invoice.delete"})
	if err != nil {
		t.Fatalf("handing out a held permission: %v", err)
	}
	if len(change.Added) != 1 || change.Added[0] != "invoice.delete" {
		t.Errorf("the change added %v, want invoice.delete", change.Added)
	}
}

// TestTakingADirectGrantBackIsNotBoundedByHoldingIt keeps a mistake undoable.
//
// Removing a permission is never an escalation, and requiring the permission in
// order to remove it would leave whoever inherited the mess unable to clean it.
func TestTakingADirectGrantBackIsNotBoundedByHoldingIt(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	holder := operator("acme", "invoice.delete")
	if _, err := svc.SetDirectActions(ctx, holder, "someone-else", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("the setup grant: %v", err)
	}

	// This one administers permissions and does not hold invoice.delete.
	cleaner := operator("acme")
	change, err := svc.SetDirectActions(ctx, cleaner, "someone-else", nil)
	if err != nil {
		t.Fatalf("taking back a permission the actor does not hold: %v", err)
	}
	if len(change.Removed) != 1 || change.Removed[0] != "invoice.delete" {
		t.Errorf("the change removed %v, want invoice.delete", change.Removed)
	}
}

// TestADirectGrantOfOneTenantIsInvisibleToAnother is the cross-tenant refusal,
// asked of the new table rather than assumed from the old ones.
func TestADirectGrantOfOneTenantIsInvisibleToAnother(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()

	if _, err := svc.SetDirectActions(ctx, operator("acme", "invoice.delete"), "shared-id", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting in acme: %v", err)
	}

	// The same person identifier, read as the other customer.
	held, err := svc.DirectActionsOf(ctx, operator("globex", "invoice.delete"), "shared-id")
	if err != nil {
		t.Fatalf("reading in globex: %v", err)
	}
	if len(held) != 0 {
		t.Errorf("globex reads %v of acme's rows", held)
	}

	effective, err := svc.EffectiveFor(ctx, operator("globex", "invoice.delete"), "shared-id")
	if err != nil {
		t.Fatalf("resolving in globex: %v", err)
	}
	if len(effective.Grants) != 0 {
		t.Errorf("globex resolves %v of acme's rows", effective.Roles())
	}
}

// TestADirectGrantReachesTheResolvedSet holds the half that makes the feature
// worth having: what is written here is what a policy in Go later decides with.
func TestADirectGrantReachesTheResolvedSet(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	if _, err := svc.SetDirectActions(ctx, operator("acme", "invoice.delete"), "carol", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}

	carol := security.Subject{ID: "carol", Tenant: "acme", Verified: true}
	resolved, err := svc.ResolveOwn(ctx, carol)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(resolved.Roles) != 1 || resolved.Roles[0] != "invoice.delete" {
		t.Fatalf("carol resolves to %v, want invoice.delete", resolved.Roles)
	}
	if len(resolved.Groups) != 0 {
		t.Errorf("carol is in %v, and was put in no group", resolved.Groups)
	}

	effective, err := svc.EffectiveFor(ctx, operator("acme"), "carol")
	if err != nil {
		t.Fatalf("reading the effective permissions: %v", err)
	}
	if len(effective.Grants) != 1 || !effective.Grants[0].Direct || len(effective.Grants[0].Groups) != 0 {
		t.Fatalf("the grant reads as %+v, want one direct grant with no group behind it", effective.Grants)
	}
	if len(effective.Direct) != 1 {
		t.Errorf("the direct list holds %v", effective.Direct)
	}
}

// TestAGroupAndADirectGrantOfTheSameActionBothShow keeps the origin honest.
//
// Two sources granting one action is the case somebody looks at a screen to
// understand, and an answer that reported only the first would send whoever is
// taking the permission away to the wrong place.
func TestAGroupAndADirectGrantOfTheSameActionBothShow(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	actor := operator("acme", "invoice.delete")

	group := seed(t, svc, actor, permission.CreateGroupRequest{Slug: "billing", Name: "Billing"})
	if _, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting to the group: %v", err)
	}
	if _, err := svc.SetMembers(ctx, actor, group.ID, []string{"dave"}); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	if _, err := svc.SetDirectActions(ctx, actor, "dave", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting directly: %v", err)
	}

	effective, err := svc.EffectiveFor(ctx, actor, "dave")
	if err != nil {
		t.Fatalf("reading the effective permissions: %v", err)
	}
	if len(effective.Grants) != 1 {
		t.Fatalf("dave holds %d distinct actions, want one", len(effective.Grants))
	}
	grant := effective.Grants[0]
	if !grant.Direct {
		t.Error("the grant does not say it was given directly")
	}
	if len(grant.Groups) != 1 || grant.Groups[0].Slug != "billing" {
		t.Errorf("the grant names %v as its groups, want billing", grant.Groups)
	}
	if len(effective.Roles()) != 1 {
		t.Errorf("dave resolves to %v, want one entry per distinct action", effective.Roles())
	}
}

// TestARevokedDirectGrantIsNotServedFromTheResolver is the stale-cache
// protection, asked of the table that was added after the mechanism was built.
//
// The token is what makes revocation take effect, and a write path that forgot
// to move it would serve the permission from memory until something else
// changed.
func TestARevokedDirectGrantIsNotServedFromTheResolver(t *testing.T) {
	t.Parallel()

	svc := service(t)
	ctx := context.Background()
	actor := operator("acme", "invoice.delete")
	erin := security.Subject{ID: "erin", Tenant: "acme", Verified: true}

	if _, err := svc.SetDirectActions(ctx, actor, "erin", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}

	before, err := svc.Version(ctx, erin)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	resolved, err := svc.ResolveOwn(ctx, erin)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(resolved.Roles) != 1 {
		t.Fatalf("erin resolves to %v before the revocation", resolved.Roles)
	}

	if _, err := svc.SetDirectActions(ctx, actor, "erin", nil); err != nil {
		t.Fatalf("revoking: %v", err)
	}

	after, err := svc.Version(ctx, erin)
	if err != nil {
		t.Fatalf("reading the version again: %v", err)
	}
	if after == before {
		t.Fatal("the token did not move on a direct revocation, so a remembered answer outlives it")
	}
	resolved, err = svc.ResolveOwn(ctx, erin)
	if err != nil {
		t.Fatalf("resolving after the revocation: %v", err)
	}
	if len(resolved.Roles) != 0 {
		t.Errorf("erin still resolves to %v", resolved.Roles)
	}
}

// TestADirectGrantIsCheckedAgainstTheCatalogue holds the thesis on the new
// path: a row here links to a permission that already exists because some
// policy reads it, and cannot bring one into being.
func TestADirectGrantIsCheckedAgainstTheCatalogue(t *testing.T) {
	t.Parallel()

	svc := service(t)
	actor := operator("acme", "invoice.delete", "ledger.forge")

	_, err := svc.SetDirectActions(context.Background(), actor, "frank", []security.Action{"ledger.forge"})
	if !errors.Is(err, permission.ErrUnknownAction) {
		t.Fatalf("granting an undeclared action = %v, want ErrUnknownAction", err)
	}
}
