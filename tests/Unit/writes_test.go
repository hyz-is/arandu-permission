package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/auth"

	permission "github.com/hyz-is/arandu-permission"
)

// readOnly are the actions a Grant is issued for to read, and nothing else.
var readOnly = map[security.Action]bool{
	permission.PermissionView:    true,
	permission.PermissionList:    true,
	permission.PermissionResolve: true,
}

// TestEveryPolicyDeclaresWhichActionsWrite holds the four policies to one
// answer: the reading actions read, every other action of the package writes.
// A policy that answered nothing would issue Grants that write for
// permission.view, and a service that read a group and then saved it would
// save.
func TestEveryPolicyDeclaresWhichActionsWrite(t *testing.T) {
	t.Parallel()

	policies := map[string]auth.Writes{
		"GroupPolicy":      permission.GroupPolicy{},
		"ActionPolicy":     permission.ActionPolicy{},
		"MembershipPolicy": permission.MembershipPolicy{},
		"UserActionPolicy": permission.UserActionPolicy{},
	}
	for name, policy := range policies {
		for _, action := range everyAction {
			if got, want := policy.Writes(action), !readOnly[action]; got != want {
				t.Errorf("%s.Writes(%s) = %v, want %v", name, action, got, want)
			}
		}
		if policy.Writes("invoice.delete") {
			t.Errorf("%s says an action it does not own writes; an action nobody named reads", name)
		}
	}
}

// TestAGrantIssuedForReadingCannotWrite is the same answer seen from the Grant
// Authorize issues: auth.Writable, which every write of the database asks,
// refuses the Grant of a reading action and admits the Grant of a writing one.
func TestAGrantIssuedForReadingCannotWrite(t *testing.T) {
	t.Parallel()

	for _, action := range everyAction {
		g, err := security.Authorize(context.Background(), permission.GroupPolicy{}, carrier(), action, permission.Group{})
		if err != nil {
			t.Fatalf("authorizing %s for a subject carrying it: %v", action, err)
		}
		err = auth.Writable(g)
		switch {
		case readOnly[action] && !errors.Is(err, auth.ErrReadGrant):
			t.Errorf("the Grant of %s answered %v to a write, want auth.ErrReadGrant", action, err)
		case !readOnly[action] && err != nil:
			t.Errorf("the Grant of %s refused a write: %v", action, err)
		}
	}
}
