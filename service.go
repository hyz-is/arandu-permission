package permission

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/framework/validation"
	"github.com/arandu-io/hesape/database/model"
)

// Pagination bounds for the listing. A request that asks for everything gets
// the maximum, never everything: an unbounded query is how one page load takes
// a production database down.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// The bounds on what a group may be called.
const (
	// MaxSlugLength is how long a slug may be. It is short because the slug is
	// an identifier somebody types.
	MaxSlugLength = 64
	// MaxNameLength and MaxDescriptionLength bound the free text.
	MaxNameLength        = 120
	MaxDescriptionLength = 500
	// MaxBulkSize is how many actions or members one bulk write may name. A
	// request that named a hundred thousand would be authorized a hundred
	// thousand times before anything was written.
	MaxBulkSize = 500
)

// PermissionService holds the rules of this package.
//
// It receives its collaborators through the constructor. There is no container
// and no resolution by reflection: what this service is made of is written at
// the one place that builds it, and reading that place is how somebody learns
// what the package touches.
//
// Everything a handler is allowed to do goes through here. The service is the
// only owner of the database handle, so the request layer cannot reach a Model
// before a policy has answered.
type PermissionService struct {
	db     *data.DB
	groups GroupPolicy
	// actions and members are the two policies that see what a bulk write is
	// about. They are separate values rather than branches of one policy
	// because each answers about its own entity, and a policy that answered
	// about three would take an argument it could not type.
	actions ActionPolicy
	members MembershipPolicy
	// direct answers about an action one person carries in their own right. It
	// is a fourth value for the reason there are three: each policy answers
	// about its own entity, and this one sees who the row is about, which is
	// what lets it refuse a subject writing itself a permission.
	direct UserActionPolicy
	// catalogue is the closed set of actions a group may carry. A write that
	// named anything else is refused before it is authorized: a permission that
	// no policy reads is not a permission, and storing it would put a promise
	// on a screen that nothing keeps.
	catalogue Catalogue
	// listeners are told what changed once it has. They are a field rather than
	// a registration, so a service built for a test is a service that tells
	// nobody unless the test asked it to.
	listeners []Listener
}

// NewPermissionService wires the service over the application's database handle
// and the catalogue its code declares.
//
// The listeners are variadic and last, so a caller that wants none writes
// nothing rather than nil.
func NewPermissionService(db *data.DB, catalogue Catalogue, listeners ...Listener) *PermissionService {
	return &PermissionService{db: db, catalogue: catalogue, listeners: listeners}
}

// Catalogue returns the closed set of actions a group may carry.
//
// It is the value the service was built with and it never changes: what a
// screen may offer and what a write may store are one set, read from one place.
func (s *PermissionService) Catalogue() Catalogue { return s.catalogue }

// GroupQuery is what a listing asks for.
type GroupQuery struct {
	// Search narrows the listing to groups whose slug or name contains it.
	//
	// The characters the pattern syntax reserves are removed before the term is
	// used, so nothing typed into the box can become a wildcard. The statement
	// carries no escape clause, and one engine of the three reads a backslash
	// there as a literal backslash -- so escaping would mean a search that
	// behaves differently depending on where the application is deployed.
	Search string
	// Cursor is where this page resumes, taken from the previous page's Next.
	Cursor string
	// Limit is how many rows the page holds. Zero means the default, and
	// anything above the maximum is brought down to it.
	Limit int
}

// CreateGroupRequest is the input contract for a new group.
//
// The fields are explicit and there is no mass assignment, so a request body
// cannot write a column nobody meant to expose. There is no TenantID here and
// there must never be one: the tenant comes from the Grant, which comes from
// the session.
type CreateGroupRequest struct {
	// Slug is the stable identifier inside the tenant.
	Slug string
	// Name and Description are what people read.
	Name        string
	Description string

	// System declares a group the installation depends on: one that cannot be
	// deleted, cannot have actions taken off it, and cannot be left without a
	// member.
	//
	// It is here for whatever seeds an installation, and the request the HTTP
	// handler builds never sets it -- the handler reads three named fields and
	// this is not one of them. A form that could declare a group undeletable
	// would let anybody make one, and there would be no way back through this
	// package.
	System bool
}

// Validate reports the errors per field.
func (r CreateGroupRequest) Validate() validation.Errors {
	e := validation.Errors{}
	validation.Required(e, "slug", r.Slug)
	validation.MaxLen(e, "slug", r.Slug, MaxSlugLength)
	if r.Slug != "" && !validSlug(r.Slug) {
		e.Add("slug", "may hold only lowercase letters, digits, - and _")
	}
	validation.Required(e, "name", r.Name)
	validation.MaxLen(e, "name", r.Name, MaxNameLength)
	validation.MaxLen(e, "description", r.Description, MaxDescriptionLength)
	return e
}

// Compile-time proof that the request honors the validation contract.
var _ validation.Validatable = CreateGroupRequest{}

// UpdateGroupRequest is what may be changed about an existing group.
//
// The slug is absent on purpose. It is what a seed, a fixture and an operator
// name a group by, and changing it would leave every one of those pointing at
// nothing while the group carried on working for everybody else.
type UpdateGroupRequest struct {
	// Name and Description are what people read.
	Name        string
	Description string
}

// Validate reports the errors per field.
func (r UpdateGroupRequest) Validate() validation.Errors {
	e := validation.Errors{}
	validation.Required(e, "name", r.Name)
	validation.MaxLen(e, "name", r.Name, MaxNameLength)
	validation.MaxLen(e, "description", r.Description, MaxDescriptionLength)
	return e
}

// Compile-time proof that the request honors the validation contract.
var _ validation.Validatable = UpdateGroupRequest{}

// ListGroups returns a page of groups.
//
// It authorizes once, on the empty candidate, and the tenant filter in the
// statement is what bounds the rows. A policy call per row would be one call
// per record on a page and would still not narrow the query -- a listing that
// has to read a customer's rows in order to decide it may not read them has
// already read them.
func (s *PermissionService) ListGroups(ctx context.Context, actor security.Subject, q GroupQuery) (GroupPage, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionList, Group{})
	if err != nil {
		return GroupPage{}, err
	}

	limit := q.Limit
	switch {
	case limit <= 0:
		limit = defaultLimit
	case limit > maxLimit:
		limit = maxLimit
	}

	page := Groups(s.db).NewQuery()
	if term := strings.TrimSpace(q.Search); term != "" {
		pattern := "%" + literal(term) + "%"
		page = page.Where(func(match *model.Builder[Group]) {
			match.Where("slug", "like", pattern).OrWhere("name", "like", pattern)
		})
	}
	if q.Cursor != "" {
		page = page.Where("slug", ">", q.Cursor)
	}

	// One row past the page, so that "is there more" is answered by what came
	// back rather than by guessing from a full page -- which is wrong exactly
	// once per result set, on the page whose last row is the last row.
	rows, err := page.OrderBy("slug").Limit(limit+1).Get(ctx, g)
	if err != nil {
		return GroupPage{}, err
	}

	out := GroupPage{Items: make([]GroupRef, 0, limit)}
	for i, row := range rows {
		if row == nil {
			continue
		}
		if i == limit {
			out.Next = rows[limit-1].Slug
			break
		}
		out.Items = append(out.Items, refOf(row))
	}
	return out, nil
}

// FindGroup returns one group, and asks the policy twice.
//
// The first call is on the empty candidate, because there is no way to read the
// row without a Grant and no way to hold a Grant without a decision. What it
// decides is whether this subject may view groups at all.
//
// The second call is on the row that came back, and it is the one a rule about
// the record itself depends on: the first call saw an empty value, so anything
// the policy says about which tenant owns the row never ran. The read is
// already scoped by tenant, so the second call is not what keeps customers
// apart. It is what keeps the policy honest.
func (s *PermissionService) FindGroup(ctx context.Context, actor security.Subject, id string) (*Group, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return nil, err
	}

	record, err := Groups(s.db).NewQuery().WhereKey(id).First(ctx, g)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionView, *record); err != nil {
		return nil, err
	}
	return record, nil
}

// CreateGroup adds a group.
//
// The candidate is authorized before it is stored, and the candidate is what
// the policy sees -- so a rule about what may be created is a rule about the
// group being created, and not about the person alone.
func (s *PermissionService) CreateGroup(ctx context.Context, actor security.Subject, in CreateGroupRequest) (*Group, error) {
	if errs := in.Validate(); errs.Any() {
		return nil, errs
	}

	g, err := security.Authorize(ctx, s.groups, actor, PermissionCreate, Group{Slug: in.Slug, Name: in.Name})
	if err != nil {
		return nil, err
	}

	taken, err := Groups(s.db).NewQuery().Where("slug", "=", in.Slug).Exists(ctx, g)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrSlugTaken
	}

	id, err := data.NewID()
	if err != nil {
		return nil, err
	}
	instance, err := Groups(s.db).NewInstance(nil, false)
	if err != nil {
		return nil, err
	}
	candidate := instance.Entity
	candidate.ID = id
	candidate.TenantID = data.Tenant(g)
	candidate.Slug = in.Slug
	candidate.Name = in.Name
	candidate.Description = in.Description
	candidate.IsSystem = in.System
	if _, err := candidate.Save(ctx, g); err != nil {
		return nil, err
	}
	s.notify(ctx, g, Event{
		Kind:      GroupCreated,
		ActorID:   actor.ID,
		GroupID:   candidate.ID,
		GroupSlug: candidate.Slug,
	})
	return candidate, nil
}

// UpdateGroup changes what a group is called.
//
// It does not change what the group carries. Nothing about the name or the
// description reaches an authorization decision, so this path never touches the
// version token: a rename is not a permission change and invalidating every
// resolved subject for one would be a stampede for nothing.
func (s *PermissionService) UpdateGroup(ctx context.Context, actor security.Subject, id string, in UpdateGroupRequest) (*Group, error) {
	if errs := in.Validate(); errs.Any() {
		return nil, errs
	}

	g, err := security.Authorize(ctx, s.groups, actor, PermissionUpdate, Group{})
	if err != nil {
		return nil, err
	}

	record, err := Groups(s.db).NewQuery().WhereKey(id).First(ctx, g)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionUpdate, *record); err != nil {
		return nil, err
	}

	record.Name = in.Name
	record.Description = in.Description
	if _, err := record.Save(ctx, g); err != nil {
		return nil, err
	}
	s.notify(ctx, g, Event{
		Kind:      GroupUpdated,
		ActorID:   actor.ID,
		GroupID:   record.ID,
		GroupSlug: record.Slug,
	})
	return record, nil
}

// DeleteGroup removes a group, the actions it carried and the memberships it
// held.
//
// The three deletions and the version bump are one transaction. A group whose
// row is gone while its memberships survive is a group that grants nothing and
// cannot be found to be repaired.
func (s *PermissionService) DeleteGroup(ctx context.Context, actor security.Subject, id string) error {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionDelete, Group{})
	if err != nil {
		return err
	}

	record, err := Groups(s.db).NewQuery().WhereKey(id).First(ctx, g)
	if err != nil {
		return err
	}
	if record == nil {
		return ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionDelete, *record); err != nil {
		return err
	}

	if err := data.Transaction(ctx, s.db, func(ctx context.Context) error {
		if _, err := GroupActions(s.db).NewQuery().Where("group_id", "=", record.ID).Delete(ctx, g); err != nil {
			return err
		}
		if _, err := GroupUsers(s.db).NewQuery().Where("group_id", "=", record.ID).Delete(ctx, g); err != nil {
			return err
		}
		if _, err := record.Delete(ctx, g); err != nil {
			return err
		}
		return s.bump(ctx, g)
	}); err != nil {
		return err
	}

	version, err := s.version(ctx, g)
	if err != nil {
		return err
	}
	s.notify(ctx, g, Event{
		Kind:      GroupDeleted,
		ActorID:   actor.ID,
		GroupID:   record.ID,
		GroupSlug: record.Slug,
		Version:   version,
	})
	return nil
}

// ActionsOf returns the actions one group carries, sorted.
func (s *PermissionService) ActionsOf(ctx context.Context, actor security.Subject, groupID string) ([]security.Action, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return nil, err
	}

	record, err := Groups(s.db).NewQuery().WhereKey(groupID).First(ctx, g)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionView, *record); err != nil {
		return nil, err
	}

	held, err := s.held(ctx, g, record.ID)
	if err != nil {
		return nil, err
	}
	out := make([]security.Action, 0, len(held))
	for _, action := range held {
		out = append(out, security.Action(action))
	}
	return out, nil
}

// PreviewActions reports what SetActions would change, and writes nothing.
//
// It exists so that a screen can show the difference before it is applied, from
// the same computation the write uses. A summary produced by a second reading
// of the same intent is a summary that can disagree with what happens next.
func (s *PermissionService) PreviewActions(ctx context.Context, actor security.Subject, groupID string, wanted []security.Action) (Change, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return Change{}, err
	}
	_, change, err := s.plannedActions(ctx, g, actor, groupID, wanted)
	if err != nil {
		return Change{}, err
	}
	return change, nil
}

// SetActions makes a group carry exactly these actions.
//
// It is the only way an action is attached or detached, and that is deliberate:
// a per-action call beside it would be a second spelling of the same write, and
// the two would need the same authorization, the same catalogue check and the
// same version bump written twice. Attaching one action is this call with one
// more in the list.
//
// Every added action is authorized on its own, so the rule that nobody grants
// what they do not hold is asked once per action rather than once per request.
func (s *PermissionService) SetActions(ctx context.Context, actor security.Subject, groupID string, wanted []security.Action) (Change, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionGrant, Group{})
	if err != nil {
		return Change{}, err
	}

	record, change, err := s.plannedActions(ctx, g, actor, groupID, wanted)
	if err != nil {
		return Change{}, err
	}
	if change.Empty() {
		return change, nil
	}

	if len(change.Added) > 0 {
		if _, err := security.Authorize(ctx, s.groups, actor, PermissionGrant, *record); err != nil {
			return Change{}, err
		}
	}
	if len(change.Removed) > 0 {
		if _, err := security.Authorize(ctx, s.groups, actor, PermissionRevoke, *record); err != nil {
			return Change{}, err
		}
	}
	for _, action := range change.Added {
		link := GroupAction{TenantID: record.TenantID, GroupID: record.ID, Action: action}
		if _, err := security.Authorize(ctx, s.actions, actor, PermissionGrant, link); err != nil {
			return Change{}, err
		}
	}
	for _, action := range change.Removed {
		link := GroupAction{TenantID: record.TenantID, GroupID: record.ID, Action: action}
		if _, err := security.Authorize(ctx, s.actions, actor, PermissionRevoke, link); err != nil {
			return Change{}, err
		}
	}

	err = data.Transaction(ctx, s.db, func(ctx context.Context) error {
		for _, action := range change.Added {
			id, err := data.NewID()
			if err != nil {
				return err
			}
			instance, err := GroupActions(s.db).NewInstance(nil, false)
			if err != nil {
				return err
			}
			row := instance.Entity
			row.ID = id
			row.TenantID = data.Tenant(g)
			row.GroupID = record.ID
			row.Action = action
			if _, err := row.Save(ctx, g); err != nil {
				return err
			}
		}
		if len(change.Removed) > 0 {
			removed := make([]any, 0, len(change.Removed))
			for _, action := range change.Removed {
				removed = append(removed, action)
			}
			if _, err := GroupActions(s.db).NewQuery().
				Where("group_id", "=", record.ID).
				WhereIn("action", removed).
				Delete(ctx, g); err != nil {
				return err
			}
		}
		return s.bump(ctx, g)
	})
	if err != nil {
		return Change{}, err
	}
	base := Event{ActorID: actor.ID, GroupID: record.ID, GroupSlug: record.Slug}
	if err := s.announce(ctx, g, base, ActionsAttached, ActionsDetached, change, true); err != nil {
		return Change{}, err
	}
	return change, nil
}

// MembersOf returns who is in one group, sorted.
func (s *PermissionService) MembersOf(ctx context.Context, actor security.Subject, groupID string) ([]string, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return nil, err
	}

	record, err := Groups(s.db).NewQuery().WhereKey(groupID).First(ctx, g)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionView, *record); err != nil {
		return nil, err
	}
	return s.membership(ctx, g, record.ID)
}

// PreviewMembers reports what SetMembers would change, and writes nothing.
func (s *PermissionService) PreviewMembers(ctx context.Context, actor security.Subject, groupID string, wanted []string) (Change, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return Change{}, err
	}
	_, change, err := s.plannedMembers(ctx, g, actor, groupID, wanted)
	if err != nil {
		return Change{}, err
	}
	return change, nil
}

// SetMembers makes a group hold exactly these people.
//
// Emptying a system group is refused, and it is refused here rather than in a
// policy because the policy is handed one row and the question is about how
// many are left. It is not an authorization refusal: the subject was allowed to
// do it, and what is refused is the state it would leave behind -- an
// application whose only group carrying the administration of permissions has
// nobody in it, and therefore nobody who could put anybody back.
func (s *PermissionService) SetMembers(ctx context.Context, actor security.Subject, groupID string, wanted []string) (Change, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionAssign, Group{})
	if err != nil {
		return Change{}, err
	}

	record, change, err := s.plannedMembers(ctx, g, actor, groupID, wanted)
	if err != nil {
		return Change{}, err
	}
	if change.Empty() {
		return change, nil
	}
	if record.IsSystem && len(change.Unchanged)+len(change.Added) == 0 {
		return Change{}, ErrLastMember
	}

	for _, userID := range change.Added {
		link := GroupUser{TenantID: record.TenantID, GroupID: record.ID, UserID: userID}
		if _, err := security.Authorize(ctx, s.members, actor, PermissionAssign, link); err != nil {
			return Change{}, err
		}
	}
	for _, userID := range change.Removed {
		link := GroupUser{TenantID: record.TenantID, GroupID: record.ID, UserID: userID}
		if _, err := security.Authorize(ctx, s.members, actor, PermissionUnassign, link); err != nil {
			return Change{}, err
		}
	}

	err = data.Transaction(ctx, s.db, func(ctx context.Context) error {
		for _, userID := range change.Added {
			id, err := data.NewID()
			if err != nil {
				return err
			}
			instance, err := GroupUsers(s.db).NewInstance(nil, false)
			if err != nil {
				return err
			}
			row := instance.Entity
			row.ID = id
			row.TenantID = data.Tenant(g)
			row.GroupID = record.ID
			row.UserID = userID
			if _, err := row.Save(ctx, g); err != nil {
				return err
			}
		}
		if len(change.Removed) > 0 {
			removed := make([]any, 0, len(change.Removed))
			for _, userID := range change.Removed {
				removed = append(removed, userID)
			}
			if _, err := GroupUsers(s.db).NewQuery().
				Where("group_id", "=", record.ID).
				WhereIn("user_id", removed).
				Delete(ctx, g); err != nil {
				return err
			}
		}
		return s.bump(ctx, g)
	})
	if err != nil {
		return Change{}, err
	}
	base := Event{ActorID: actor.ID, GroupID: record.ID, GroupSlug: record.Slug}
	if err := s.announce(ctx, g, base, MembersAttached, MembersDetached, change, false); err != nil {
		return Change{}, err
	}
	return change, nil
}

// DirectActionsOf returns the actions one person carries in their own right,
// sorted.
//
// They are what the reference calls extra permissions: what somebody holds on
// top of whatever their groups confer. A screen draws them beside the groups
// rather than mixed into them, because the two are undone in different places.
func (s *PermissionService) DirectActionsOf(ctx context.Context, actor security.Subject, userID string) ([]security.Action, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: it is empty", ErrInvalidMember)
	}

	held, err := s.directHeld(ctx, g, userID)
	if err != nil {
		return nil, err
	}
	out := make([]security.Action, 0, len(held))
	for _, action := range held {
		out = append(out, security.Action(action))
	}
	return out, nil
}

// PreviewDirectActions reports what SetDirectActions would change, and writes
// nothing.
func (s *PermissionService) PreviewDirectActions(ctx context.Context, actor security.Subject, userID string, wanted []security.Action) (Change, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return Change{}, err
	}
	_, err = s.directHeld(ctx, g, userID)
	if err != nil {
		return Change{}, err
	}
	return s.plannedDirect(ctx, g, userID, wanted)
}

// SetDirectActions makes one person carry exactly these actions in their own
// right.
//
// It is the only way such a row is written, for the reason SetActions is the
// only way a group carries one: a per-action call beside it would need the same
// authorization, the same catalogue check and the same version bump written
// twice.
//
// Every added action is authorized on its own. That is where the two rules that
// matter are asked -- nobody hands out what they do not hold, and nobody hands
// anything to themselves -- and asking them once per action rather than once per
// request is what makes a request naming twenty permissions no weaker than
// twenty requests naming one.
func (s *PermissionService) SetDirectActions(ctx context.Context, actor security.Subject, userID string, wanted []security.Action) (Change, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionGrantDirect, Group{})
	if err != nil {
		return Change{}, err
	}

	change, err := s.plannedDirect(ctx, g, userID, wanted)
	if err != nil {
		return Change{}, err
	}
	if change.Empty() {
		return change, nil
	}

	tenant := data.Tenant(g)
	for _, action := range change.Added {
		row := UserAction{TenantID: tenant, UserID: userID, Action: action}
		if _, err := security.Authorize(ctx, s.direct, actor, PermissionGrantDirect, row); err != nil {
			return Change{}, err
		}
	}
	for _, action := range change.Removed {
		row := UserAction{TenantID: tenant, UserID: userID, Action: action}
		if _, err := security.Authorize(ctx, s.direct, actor, PermissionRevokeDirect, row); err != nil {
			return Change{}, err
		}
	}

	err = data.Transaction(ctx, s.db, func(ctx context.Context) error {
		for _, action := range change.Added {
			id, err := data.NewID()
			if err != nil {
				return err
			}
			instance, err := UserActions(s.db).NewInstance(nil, false)
			if err != nil {
				return err
			}
			row := instance.Entity
			row.ID = id
			row.TenantID = data.Tenant(g)
			row.UserID = userID
			row.Action = action
			if _, err := row.Save(ctx, g); err != nil {
				return err
			}
		}
		if len(change.Removed) > 0 {
			removed := make([]any, 0, len(change.Removed))
			for _, action := range change.Removed {
				removed = append(removed, action)
			}
			if _, err := UserActions(s.db).NewQuery().
				Where("user_id", "=", userID).
				WhereIn("action", removed).
				Delete(ctx, g); err != nil {
				return err
			}
		}
		return s.bump(ctx, g)
	})
	if err != nil {
		return Change{}, err
	}
	base := Event{ActorID: actor.ID, UserID: userID}
	if err := s.announce(ctx, g, base, ActionsAttached, ActionsDetached, change, true); err != nil {
		return Change{}, err
	}
	return change, nil
}

// EffectiveFor returns what one person may do, and which group gives them each
// of it.
//
// The origin is the answer to the only question anybody asks a permissions
// screen twice: not what somebody has, but why. A list without it sends the
// person reading it through every group by hand.
func (s *PermissionService) EffectiveFor(ctx context.Context, actor security.Subject, userID string) (Effective, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionView, Group{})
	if err != nil {
		return Effective{}, err
	}
	return s.effective(ctx, g, userID)
}

// Resolution is what a request carries into every policy that runs after it:
// the effective actions of the acting subject, and the token they were read at.
type Resolution struct {
	// Version is the tenant's token at the moment the actions were read. It is
	// compared for equality against the current one to decide whether a
	// remembered answer is still the answer.
	Version int64
	// Roles are the effective actions, one entry per distinct action, sorted.
	Roles []string
	// Groups are the groups the subject belongs to, sorted by slug. They name
	// the origin on a screen and are never read as authorization: a group is
	// not a permission, and a decision taken on a group name would be a
	// decision the catalogue never checked.
	Groups []GroupRef
}

// ResolveOwn returns the effective actions of the subject asking.
//
// It is what fills in the actions every other policy reads, so it cannot itself
// depend on them: the policy admits a subject asking about its own rows and
// refuses every other question. That is the whole of the exception, and it is
// an exception to which rule applies rather than to whether one does -- the
// read is authorized, it is scoped by the tenant on the Grant, and it answers
// about one person.
func (s *PermissionService) ResolveOwn(ctx context.Context, actor security.Subject) (Resolution, error) {
	g, err := security.Authorize(ctx, s.members, actor, PermissionResolve, GroupUser{UserID: actor.ID})
	if err != nil {
		return Resolution{}, err
	}

	version, err := s.version(ctx, g)
	if err != nil {
		return Resolution{}, err
	}
	effective, err := s.effective(ctx, g, actor.ID)
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{Version: version, Roles: effective.Roles(), Groups: effective.Groups}, nil
}

// Version returns the tenant's permission token.
//
// It changes whenever anything that could change a decision changes, and it is
// compared for equality: a remembered resolution is still the answer while the
// token it was read at is still the token.
func (s *PermissionService) Version(ctx context.Context, actor security.Subject) (int64, error) {
	g, err := security.Authorize(ctx, s.members, actor, PermissionResolve, GroupUser{UserID: actor.ID})
	if err != nil {
		return 0, err
	}
	return s.version(ctx, g)
}

// CatalogueEntry is one action of the catalogue and the groups that carry it.
type CatalogueEntry struct {
	// Action is the identifier, and it is the identifier that is stored. What a
	// person reads is derived from it when the page is drawn, so a label can be
	// rewritten in any language without a row changing.
	Action security.Action
	// Groups are the groups that carry it, sorted by slug. Empty means the
	// action exists in the code and no group grants it.
	Groups []GroupRef
}

// CatalogueDomain is one section of the catalogue screen.
type CatalogueDomain struct {
	// Name is the part of every action below it that comes before the first
	// dot.
	Name string
	// Entries are the actions of this domain, sorted.
	Entries []CatalogueEntry
}

// CatalogueView is the catalogue as a screen draws it.
type CatalogueView struct {
	// Domains are the sections, sorted by name.
	Domains []CatalogueDomain
}

// ViewCatalogue returns the catalogue grouped by domain, with the groups that
// carry each action.
func (s *PermissionService) ViewCatalogue(ctx context.Context, actor security.Subject) (CatalogueView, error) {
	g, err := security.Authorize(ctx, s.groups, actor, PermissionList, Group{})
	if err != nil {
		return CatalogueView{}, err
	}

	refs, err := s.groupRefs(ctx, g)
	if err != nil {
		return CatalogueView{}, err
	}
	links, err := GroupActions(s.db).NewQuery().OrderBy("action").Get(ctx, g)
	if err != nil {
		return CatalogueView{}, err
	}

	carriers := map[string][]GroupRef{}
	for _, link := range links {
		if link == nil {
			continue
		}
		if ref, known := refs[link.GroupID]; known {
			carriers[link.Action] = append(carriers[link.Action], ref)
		}
	}
	for action := range carriers {
		sortRefs(carriers[action])
	}

	out := CatalogueView{}
	for _, domain := range s.catalogue.Domains() {
		section := CatalogueDomain{Name: domain.Name}
		for _, action := range domain.Actions {
			section.Entries = append(section.Entries, CatalogueEntry{
				Action: action,
				Groups: carriers[string(action)],
			})
		}
		out.Domains = append(out.Domains, section)
	}
	return out, nil
}

// MatrixCell is one square of the matrix: an action, and whether the row's
// group carries it.
type MatrixCell struct {
	Action security.Action
	Held   bool
}

// MatrixRow is one group of the matrix and its squares, in the same order as
// the matrix's actions.
type MatrixRow struct {
	Group GroupRef
	// System marks a group whose actions cannot be detached, so a screen can
	// draw the squares as fixed rather than offering a change that is refused.
	System bool
	Cells  []MatrixCell
}

// MatrixView is the group by permission grid.
type MatrixView struct {
	// Actions are the columns, sorted, and Domains is the same set grouped so
	// that a screen can draw a heading over each run of columns.
	Actions []security.Action
	Domains []Domain
	// Rows are the groups, in slug order, and Next is where the following page
	// resumes.
	Rows []MatrixRow
	Next string
}

// ViewMatrix returns a page of groups against every action of the catalogue.
func (s *PermissionService) ViewMatrix(ctx context.Context, actor security.Subject, q GroupQuery) (MatrixView, error) {
	page, err := s.ListGroups(ctx, actor, q)
	if err != nil {
		return MatrixView{}, err
	}

	g, err := security.Authorize(ctx, s.groups, actor, PermissionList, Group{})
	if err != nil {
		return MatrixView{}, err
	}

	ids := make([]any, 0, len(page.Items))
	byID := make(map[string]bool, len(page.Items))
	for _, item := range page.Items {
		ids = append(ids, item.ID)
		byID[item.ID] = true
	}

	held := map[string]map[string]bool{}
	if len(ids) > 0 {
		links, err := GroupActions(s.db).NewQuery().WhereIn("group_id", ids).Get(ctx, g)
		if err != nil {
			return MatrixView{}, err
		}
		for _, link := range links {
			if link == nil || !byID[link.GroupID] {
				continue
			}
			if held[link.GroupID] == nil {
				held[link.GroupID] = map[string]bool{}
			}
			held[link.GroupID][link.Action] = true
		}
	}

	system, err := s.systemGroups(ctx, g)
	if err != nil {
		return MatrixView{}, err
	}

	out := MatrixView{Actions: s.catalogue.All(), Domains: s.catalogue.Domains(), Next: page.Next}
	for _, item := range page.Items {
		row := MatrixRow{Group: item, System: system[item.ID], Cells: make([]MatrixCell, 0, len(out.Actions))}
		for _, action := range out.Actions {
			row.Cells = append(row.Cells, MatrixCell{Action: action, Held: held[item.ID][string(action)]})
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

// plannedActions loads the group and works out what the wanted set would
// change, without writing anything.
//
// The catalogue check happens here, before any authorization: an action nobody
// declared is not a permission that was refused, it is a permission that does
// not exist, and answering it as a refusal sends somebody looking for who to
// ask.
func (s *PermissionService) plannedActions(ctx context.Context, g security.Grant, actor security.Subject, groupID string, wanted []security.Action) (*Group, Change, error) {
	if len(wanted) > MaxBulkSize {
		return nil, Change{}, fmt.Errorf("%w: %d actions were named and the limit is %d", ErrTooMany, len(wanted), MaxBulkSize)
	}
	asked := make([]string, 0, len(wanted))
	for _, action := range wanted {
		if !s.catalogue.Has(action) {
			return nil, Change{}, fmt.Errorf("%w: %s", ErrUnknownAction, action)
		}
		asked = append(asked, string(action))
	}

	record, err := Groups(s.db).NewQuery().WhereKey(groupID).First(ctx, g)
	if err != nil {
		return nil, Change{}, err
	}
	if record == nil {
		return nil, Change{}, ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionView, *record); err != nil {
		return nil, Change{}, err
	}

	current, err := s.held(ctx, g, record.ID)
	if err != nil {
		return nil, Change{}, err
	}
	return record, difference(current, asked), nil
}

// plannedMembers is the membership half of plannedActions.
func (s *PermissionService) plannedMembers(ctx context.Context, g security.Grant, actor security.Subject, groupID string, wanted []string) (*Group, Change, error) {
	if len(wanted) > MaxBulkSize {
		return nil, Change{}, fmt.Errorf("%w: %d members were named and the limit is %d", ErrTooMany, len(wanted), MaxBulkSize)
	}
	for _, userID := range wanted {
		if strings.TrimSpace(userID) == "" {
			return nil, Change{}, fmt.Errorf("%w: it is empty", ErrInvalidMember)
		}
	}

	record, err := Groups(s.db).NewQuery().WhereKey(groupID).First(ctx, g)
	if err != nil {
		return nil, Change{}, err
	}
	if record == nil {
		return nil, Change{}, ErrNotFound
	}
	if _, err := security.Authorize(ctx, s.groups, actor, PermissionView, *record); err != nil {
		return nil, Change{}, err
	}

	current, err := s.membership(ctx, g, record.ID)
	if err != nil {
		return nil, Change{}, err
	}
	return record, difference(current, wanted), nil
}

// plannedDirect works out what the wanted set would change about one person's
// own grants, without writing anything.
//
// The catalogue check happens here, before any authorization, for the reason it
// happens first on the group path: an action nobody declared is not a permission
// that was refused, it is a permission that does not exist.
func (s *PermissionService) plannedDirect(ctx context.Context, g security.Grant, userID string, wanted []security.Action) (Change, error) {
	if strings.TrimSpace(userID) == "" {
		return Change{}, fmt.Errorf("%w: it is empty", ErrInvalidMember)
	}
	if len(wanted) > MaxBulkSize {
		return Change{}, fmt.Errorf("%w: %d actions were named and the limit is %d", ErrTooMany, len(wanted), MaxBulkSize)
	}
	asked := make([]string, 0, len(wanted))
	for _, action := range wanted {
		if !s.catalogue.Has(action) {
			return Change{}, fmt.Errorf("%w: %s", ErrUnknownAction, action)
		}
		asked = append(asked, string(action))
	}

	current, err := s.directHeld(ctx, g, userID)
	if err != nil {
		return Change{}, err
	}
	return difference(current, asked), nil
}

// directHeld is the actions one person carries in their own right, sorted.
func (s *PermissionService) directHeld(ctx context.Context, g security.Grant, userID string) ([]string, error) {
	rows, err := UserActions(s.db).NewQuery().Where("user_id", "=", userID).OrderBy("action").Get(ctx, g)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, row.Action)
		}
	}
	return out, nil
}

// held is the actions one group carries, sorted.
func (s *PermissionService) held(ctx context.Context, g security.Grant, groupID string) ([]string, error) {
	rows, err := GroupActions(s.db).NewQuery().Where("group_id", "=", groupID).OrderBy("action").Get(ctx, g)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, row.Action)
		}
	}
	return out, nil
}

// membership is who is in one group, sorted.
func (s *PermissionService) membership(ctx context.Context, g security.Grant, groupID string) ([]string, error) {
	rows, err := GroupUsers(s.db).NewQuery().Where("group_id", "=", groupID).OrderBy("user_id").Get(ctx, g)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, row.UserID)
		}
	}
	return out, nil
}

// groupRefs is every group of the tenant, by identifier.
func (s *PermissionService) groupRefs(ctx context.Context, g security.Grant) (map[string]GroupRef, error) {
	rows, err := Groups(s.db).NewQuery().OrderBy("slug").Get(ctx, g)
	if err != nil {
		return nil, err
	}
	out := make(map[string]GroupRef, len(rows))
	for _, row := range rows {
		if row != nil {
			out[row.ID] = refOf(row)
		}
	}
	return out, nil
}

// systemGroups is the set of group identifiers whose actions cannot be
// detached.
func (s *PermissionService) systemGroups(ctx context.Context, g security.Grant) (map[string]bool, error) {
	rows, err := Groups(s.db).NewQuery().Where("is_system", "=", true).Get(ctx, g)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row != nil {
			out[row.ID] = true
		}
	}
	return out, nil
}

// effective assembles what one person may do out of their memberships, the
// actions those groups carry, the actions they carry in their own right, and
// the groups themselves.
//
// The catalogue filters the result. A row naming an action that no longer exists
// in the code is left where it is and not answered with: deleting rows on a read
// would make a deployment that dropped an action destroy data, and answering
// with it would put a permission into a decision that nothing reads.
//
// A person in no group is still asked about. Their memberships are empty and
// their own grants are not, and a read that stopped at the first empty answer
// would report somebody carrying nothing while a row said otherwise.
func (s *PermissionService) effective(ctx context.Context, g security.Grant, userID string) (Effective, error) {
	out := Effective{UserID: userID}
	if userID == "" {
		return out, nil
	}

	memberships, err := GroupUsers(s.db).NewQuery().Where("user_id", "=", userID).Get(ctx, g)
	if err != nil {
		return Effective{}, err
	}
	ids := make([]any, 0, len(memberships))
	for _, row := range memberships {
		if row != nil {
			ids = append(ids, row.GroupID)
		}
	}

	refs := map[string]GroupRef{}
	if len(ids) > 0 {
		groups, err := Groups(s.db).NewQuery().WhereIn("id", ids).OrderBy("slug").Get(ctx, g)
		if err != nil {
			return Effective{}, err
		}
		for _, row := range groups {
			if row != nil {
				refs[row.ID] = refOf(row)
				out.Groups = append(out.Groups, refOf(row))
			}
		}
	}

	origin := map[string][]GroupRef{}
	if len(refs) > 0 {
		links, err := GroupActions(s.db).NewQuery().WhereIn("group_id", ids).OrderBy("action").Get(ctx, g)
		if err != nil {
			return Effective{}, err
		}
		for _, link := range links {
			if link == nil || !s.catalogue.Has(security.Action(link.Action)) {
				continue
			}
			if ref, known := refs[link.GroupID]; known {
				origin[link.Action] = append(origin[link.Action], ref)
			}
		}
	}

	own, err := s.directHeld(ctx, g, userID)
	if err != nil {
		return Effective{}, err
	}
	direct := make(map[string]bool, len(own))
	for _, action := range own {
		if !s.catalogue.Has(security.Action(action)) {
			continue
		}
		direct[action] = true
		out.Direct = append(out.Direct, security.Action(action))
		if _, carried := origin[action]; !carried {
			origin[action] = nil
		}
	}

	actions := make([]string, 0, len(origin))
	for action := range origin {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	for _, action := range actions {
		carriers := origin[action]
		sortRefs(carriers)
		out.Grants = append(out.Grants, EffectiveGrant{
			Action: security.Action(action),
			Groups: carriers,
			Direct: direct[action],
		})
	}
	return out, nil
}

// version reads the tenant's token, answering zero when the tenant has never
// had one.
func (s *PermissionService) version(ctx context.Context, g security.Grant) (int64, error) {
	record, err := Versions(s.db).NewQuery().First(ctx, g)
	if err != nil {
		return 0, err
	}
	if record == nil {
		return 0, nil
	}
	return record.Version, nil
}

// bump gives the tenant a token it did not have before.
//
// The new value is the larger of the clock and one past the current token, so
// that two writers changing permissions in the same instant cannot land on one
// value -- which is the failure a counter has, and it is the failure that makes
// a remembered answer outlive the change that should have ended it. Nothing
// orders these values: they are compared for equality and for nothing else.
func (s *PermissionService) bump(ctx context.Context, g security.Grant) error {
	record, err := Versions(s.db).NewQuery().First(ctx, g)
	if err != nil {
		return err
	}
	next := time.Now().UTC().UnixNano()
	if record != nil {
		if record.Version >= next {
			next = record.Version + 1
		}
		record.Version = next
		_, err = record.Save(ctx, g)
		return err
	}

	instance, err := Versions(s.db).NewInstance(nil, false)
	if err != nil {
		return err
	}
	row := instance.Entity
	row.TenantID = data.Tenant(g)
	row.Version = next
	_, err = row.Save(ctx, g)
	return err
}

// announce reports a bulk change to the listeners, as one event per direction.
//
// Two events rather than one carrying both lists, because that is the shape a
// listener wants: "these were given" and "these were taken away" are separate
// lines in an audit log and separate reasons to act, and a listener handed one
// event would begin by splitting it back into two.
//
// It reads the token after the write so that every event of the change carries
// the same one. A listener that keeps its own copy of anything compares that
// rather than a clock.
func (s *PermissionService) announce(ctx context.Context, g security.Grant, base Event, attached, detached EventKind, change Change, actions bool) error {
	if len(s.listeners) == 0 {
		return nil
	}
	version, err := s.version(ctx, g)
	if err != nil {
		return err
	}
	base.Version = version

	for _, side := range []struct {
		kind   EventKind
		values []string
	}{{attached, change.Added}, {detached, change.Removed}} {
		if len(side.values) == 0 {
			continue
		}
		event := base
		event.Kind = side.kind
		if actions {
			event.Actions = make([]security.Action, 0, len(side.values))
			for _, value := range side.values {
				event.Actions = append(event.Actions, security.Action(value))
			}
		} else {
			event.Members = append([]string(nil), side.values...)
		}
		s.notify(ctx, g, event)
	}
	return nil
}

// refOf is one group as something else refers to it.
func refOf(record *Group) GroupRef {
	return GroupRef{ID: record.ID, Slug: record.Slug, Name: record.Name}
}

// sortRefs orders groups by slug, which is the order every screen shows them
// in.
func sortRefs(refs []GroupRef) {
	sort.Slice(refs, func(i, j int) bool { return refs[i].Slug < refs[j].Slug })
}

// difference is what it takes to turn current into wanted.
//
// Repeats in wanted are collapsed, because a form that ticks one box twice is a
// form, not a request to write two rows.
func difference(current, wanted []string) Change {
	have := make(map[string]bool, len(current))
	for _, value := range current {
		have[value] = true
	}
	want := make(map[string]bool, len(wanted))
	for _, value := range wanted {
		want[value] = true
	}

	change := Change{Added: []string{}, Removed: []string{}, Unchanged: []string{}}
	for value := range want {
		if have[value] {
			change.Unchanged = append(change.Unchanged, value)
			continue
		}
		change.Added = append(change.Added, value)
	}
	for value := range have {
		if !want[value] {
			change.Removed = append(change.Removed, value)
		}
	}
	sort.Strings(change.Added)
	sort.Strings(change.Removed)
	sort.Strings(change.Unchanged)
	return change
}

// validSlug reports whether a slug is what a group may be identified by.
func validSlug(slug string) bool {
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// literal strips what the pattern syntax reserves out of a search term.
//
// Without it a person searching for "50%" matches every group, and one
// searching for "_" matches all of them one character at a time -- which reads
// as a broken search rather than as a wildcard nobody asked for.
func literal(term string) string {
	return strings.NewReplacer(`\`, "", "%", "", "_", "").Replace(term)
}
