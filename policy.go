package permission

import (
	"context"
	"fmt"

	"github.com/arandu-io/framework/security"
)

// The actions of this package.
//
// They are constants and never built from a variable: an action assembled at
// run time cannot be compared against the one a Grant was issued for, and it
// cannot be read out of the source by anything that enumerates what an
// application may grant.
//
// They are the administration of the catalogue, and they are themselves part of
// it: whoever administers permissions holds a permission to do so, granted
// through a group like any other.
const (
	// PermissionView is reading one group with the actions and members it
	// carries, and reading the effective permissions of one person.
	PermissionView security.Action = "permission.view"
	// PermissionList is paging through the groups and reading the catalogue.
	PermissionList security.Action = "permission.list"
	// PermissionCreate is adding a group.
	PermissionCreate security.Action = "permission.create"
	// PermissionUpdate is changing a group's name or description.
	PermissionUpdate security.Action = "permission.update"
	// PermissionDelete is removing a group.
	PermissionDelete security.Action = "permission.delete"
	// PermissionGrant is attaching an action to a group.
	PermissionGrant security.Action = "permission.grant"
	// PermissionRevoke is detaching an action from a group.
	PermissionRevoke security.Action = "permission.revoke"
	// PermissionAssign is putting a person into a group.
	PermissionAssign security.Action = "permission.assign"
	// PermissionUnassign is taking a person out of a group.
	PermissionUnassign security.Action = "permission.unassign"
	// PermissionGrantDirect is giving one person an action in their own right,
	// outside every group.
	//
	// It is separate from PermissionGrant because the two are different amounts
	// of power and an installation should be able to hand out one without the
	// other. Editing a group is a change somebody else can read off a screen
	// named after a role; giving one person one permission is a change nobody
	// goes looking for.
	PermissionGrantDirect security.Action = "permission.grant_direct"
	// PermissionRevokeDirect is taking such an action back.
	PermissionRevokeDirect security.Action = "permission.revoke_direct"

	// PermissionResolve is reading the groups one is a member of.
	//
	// It is decided by identity and not by membership: whoever asks may read
	// their own rows and nobody else's, so attaching it to a group would grant
	// nothing. That is why Actions leaves it out, and why leaving it out is not
	// an omission -- a screen offering a permission that changes nothing is a
	// screen nobody can reason about.
	PermissionResolve security.Action = "permission.resolve"
)

// Actions are the actions of this package that a group may carry, sorted.
//
// An application splices them into the catalogue it hands to New, so that the
// administration of permissions can itself be administered instead of being
// reachable only by whoever seeded the first group.
func Actions() []security.Action {
	return []security.Action{
		PermissionAssign,
		PermissionCreate,
		PermissionDelete,
		PermissionGrant,
		PermissionGrantDirect,
		PermissionList,
		PermissionRevoke,
		PermissionRevokeDirect,
		PermissionUnassign,
		PermissionUpdate,
		PermissionView,
	}
}

// GroupPolicy decides who may administer a group.
//
// It is the only authority over a Group, and it decides from what the subject
// carries: the effective actions filled in before any policy runs. A subject
// that carries nothing is refused every action here, which is the state an
// application starts in and the state it returns to when the last group that
// granted these actions is emptied.
type GroupPolicy struct{}

// Compile-time proof that the policy answers about this entity and no other.
var _ security.Policy[Group] = GroupPolicy{}

// Can decides whether the subject may perform the action on the group.
func (GroupPolicy) Can(_ context.Context, s security.Subject, a security.Action, record Group) error {
	// Tenant isolation comes first and applies to every action. Without it a
	// rule written below would hold across customers as soon as two of them
	// have a group with the same identifier.
	//
	// The empty id is the candidate that has not been stored yet, which belongs
	// to nobody until it is written with the tenant off the Grant.
	if record.ID != "" && record.TenantID != s.Tenant {
		return fmt.Errorf("the group belongs to another tenant")
	}

	// A group the installation depends on is not deleted and does not change
	// identity. It is the group that carries the administration of permissions,
	// and an application whose last one is gone has nobody left who can create
	// another.
	if record.IsSystem && (a == PermissionDelete || a == PermissionRevoke) {
		return fmt.Errorf("%s is not open on a system group", a)
	}

	if !s.Can(a) {
		return fmt.Errorf("no group of this subject carries %s", a)
	}
	return nil
}

// ActionPolicy decides who may attach an action to a group and detach it again.
//
// It sees the action being granted, which is what lets it hold the rule that
// matters most here: nobody hands out what they do not hold. Without it,
// whoever may edit any group may write themselves every permission the
// catalogue has, and every other check in this package passes while it happens.
type ActionPolicy struct{}

// Compile-time proof that the policy answers about this entity and no other.
var _ security.Policy[GroupAction] = ActionPolicy{}

// Can decides whether the subject may attach or detach this action.
func (ActionPolicy) Can(_ context.Context, s security.Subject, a security.Action, record GroupAction) error {
	if record.TenantID != "" && record.TenantID != s.Tenant {
		return fmt.Errorf("the group belongs to another tenant")
	}
	if !s.Can(a) {
		return fmt.Errorf("no group of this subject carries %s", a)
	}

	// Granting is bounded by what the subject already holds. Detaching is not:
	// taking a permission away is never an escalation, and requiring the
	// permission in order to remove it would leave a mistake nobody can undo.
	if a == PermissionGrant && !s.Can(security.Action(record.Action)) {
		return fmt.Errorf("%s cannot be granted by a subject that does not hold it", record.Action)
	}
	return nil
}

// MembershipPolicy decides who may put a person into a group, take them out,
// and read the groups they belong to.
//
// It sees who the row is about, which is what lets it refuse the shortest path
// to more power there is: adding yourself to a group. Everything else in this
// package would allow it -- the group is legitimate, the actions on it were
// granted by somebody who held them, and the person doing it may administer
// groups.
type MembershipPolicy struct{}

// Compile-time proof that the policy answers about this entity and no other.
var _ security.Policy[GroupUser] = MembershipPolicy{}

// Can decides whether the subject may change or read this membership.
func (MembershipPolicy) Can(_ context.Context, s security.Subject, a security.Action, record GroupUser) error {
	if record.TenantID != "" && record.TenantID != s.Tenant {
		return fmt.Errorf("the membership belongs to another tenant")
	}

	// Reading one's own memberships is decided by identity, because it is what
	// fills in the actions every other policy reads: a rule that depended on
	// those actions could never be satisfied the first time.
	if a == PermissionResolve {
		if record.UserID != "" && record.UserID == s.ID {
			return nil
		}
		return fmt.Errorf("%s answers about the subject asking and nobody else", a)
	}

	if !s.Can(a) {
		return fmt.Errorf("no group of this subject carries %s", a)
	}
	if a == PermissionAssign && record.UserID == s.ID {
		return fmt.Errorf("%s does not put the subject into a group", a)
	}
	return nil
}

// UserActionPolicy decides who may give one person an action in their own right.
//
// It is the policy with the most to refuse, because a direct grant is the
// shortest path from "may administer permissions" to "may do anything". A group
// has to be created, named and looked at by somebody; a row here is one person
// quietly holding one more permission.
//
// It holds the same two rules the group path holds, and it holds them harder:
// nobody hands out what they do not hold, and nobody hands anything to
// themselves. The second is not a nicety -- without it, whoever may administer
// permissions may write themselves every action in the catalogue in one request,
// and every other check in this package passes while it happens.
type UserActionPolicy struct{}

// Compile-time proof that the policy answers about this entity and no other.
var _ security.Policy[UserAction] = UserActionPolicy{}

// Can decides whether the subject may give this action to this person, take it
// back, or read what they carry.
func (UserActionPolicy) Can(_ context.Context, s security.Subject, a security.Action, record UserAction) error {
	if record.TenantID != "" && record.TenantID != s.Tenant {
		return fmt.Errorf("the grant belongs to another tenant")
	}

	// Reading one's own direct grants is decided by identity, for the reason
	// reading one's own memberships is: it is half of what fills in the actions
	// every other policy reads, so a rule that depended on those actions could
	// never be satisfied the first time.
	if a == PermissionResolve {
		if record.UserID != "" && record.UserID == s.ID {
			return nil
		}
		return fmt.Errorf("%s answers about the subject asking and nobody else", a)
	}

	if !s.Can(a) {
		return fmt.Errorf("no group of this subject carries %s", a)
	}

	if a == PermissionGrantDirect {
		if record.UserID == s.ID {
			return fmt.Errorf("%s does not give the subject a permission", a)
		}
		if !s.Can(security.Action(record.Action)) {
			return fmt.Errorf("%s cannot be granted by a subject that does not hold it", record.Action)
		}
	}

	// Taking a direct grant back is bounded by neither rule. Removing a
	// permission is never an escalation, and requiring the permission in order
	// to remove it would leave a mistake nobody can undo.
	return nil
}
