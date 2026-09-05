package permission

import (
	"context"
	"time"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
)

// EventKind names what happened.
//
// The set is closed and the values are constants, for the reason the actions
// are: a listener switches on one of these, and a kind assembled at run time is
// a branch nobody can find by reading the code.
//
// The reference emits four -- a role attached, a role detached, a permission
// attached, a permission detached. Here a permission is attached to two
// different things, so the pair that carries it says which by what it fills in,
// and the three that report a group appearing, changing and going away are
// added because the reference gets them from its persistence layer and this
// package has no such layer to get them from.
type EventKind string

const (
	// GroupCreated is a group that now exists.
	GroupCreated EventKind = "permission.group.created"
	// GroupUpdated is a change to what a group is called. It never changes what
	// anybody may do.
	GroupUpdated EventKind = "permission.group.updated"
	// GroupDeleted is a group that is gone, with everything it carried and
	// everybody it held.
	GroupDeleted EventKind = "permission.group.deleted"

	// ActionsAttached is permissions gained. GroupID names the group that
	// gained them, or UserID names the person who did.
	ActionsAttached EventKind = "permission.actions.attached"
	// ActionsDetached is permissions lost, addressed the same way.
	ActionsDetached EventKind = "permission.actions.detached"

	// MembersAttached is people who are now in a group.
	MembersAttached EventKind = "permission.members.attached"
	// MembersDetached is people who are no longer in one.
	MembersDetached EventKind = "permission.members.detached"
)

// Event is one thing that happened, told to whoever asked to be told.
//
// It carries what a listener needs to write a line somebody can read a year
// later: who did it, what it was about, and what changed. It does not carry the
// record, because a record handed to a listener is a record a listener can save
// -- and a write nobody authorized is exactly what this package exists to make
// impossible.
type Event struct {
	// Kind is what happened.
	Kind EventKind
	// At is when, in UTC.
	At time.Time
	// Tenant is the customer it happened in. It comes from the Grant, like
	// every other tenant here.
	Tenant string
	// ActorID is who did it. It is empty only where nothing did -- there is no
	// path in this package that writes without a subject.
	ActorID string
	// Version is the tenant's token after the change, for the kinds that move
	// it. A listener that keeps its own copy of anything compares this rather
	// than a timestamp.
	Version int64

	// GroupID and GroupSlug name the group, on the kinds that have one. Both
	// are empty on an event about what one person carries in their own right.
	GroupID   string
	GroupSlug string
	// UserID names the person, on an event about their own grants. It is empty
	// on every kind about a group.
	UserID string

	// Actions are the actions attached or detached, sorted. Empty on every kind
	// that is not about permissions.
	Actions []security.Action
	// Members are the people attached or detached, sorted. Empty on every kind
	// that is not about membership.
	Members []string
}

// Listener is something told what happened.
//
// It is called after the write has committed, in the goroutine that made it, and
// what it does is on the path of the request that caused it. A listener that
// talks to something slow makes the screen slow; one that has to do that hands
// the work to a queue and returns.
//
// It returns nothing, and that is the contract rather than an omission. The
// write is already durable by the time it is called, so there is no failure a
// listener could report that anything could still act on -- and an error that
// travelled back to the caller would report a write that succeeded as one that
// did not.
type Listener func(context.Context, Event)

// notify tells every listener what happened.
//
// It takes the Grant rather than a tenant string, so the tenant on an event is
// the tenant the statements ran under and cannot drift from it. It is called
// after the transaction, never inside: a listener that saw a change which was
// then rolled back would have told somebody about a thing that did not happen,
// and there is no message that takes it back.
func (s *PermissionService) notify(ctx context.Context, g security.Grant, event Event) {
	if len(s.listeners) == 0 {
		return
	}
	event.At = time.Now().UTC()
	event.Tenant = data.Tenant(g)
	for _, listen := range s.listeners {
		listen(ctx, event)
	}
}
