package permission

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/console"
)

// The commands this package answers to.
//
// They are values in a slice an application splices into its own console, which
// is the same shape as everything else here: nothing is discovered, nothing
// registers itself, and what a binary can do is a list somebody wrote.
//
// None of them is a way around the panel. Every one calls the same use case a
// screen calls, so the same policy answers, the same catalogue is checked and
// the same version token moves. What they add is a terminal, which is where the
// first group of an installation has to come from and where an operator with a
// list of a hundred permissions would rather be.
const (
	// CommandShow prints the groups against the catalogue.
	CommandShow = "permission:show"
	// CommandCreateGroup adds a group, and adds the first one.
	CommandCreateGroup = "permission:create-group"
	// CommandGrant attaches permissions to a group.
	CommandGrant = "permission:grant"
	// CommandRevoke detaches them.
	CommandRevoke = "permission:revoke"
	// CommandAssign puts people in a group.
	CommandAssign = "permission:assign"
	// CommandUnassign takes them out.
	CommandUnassign = "permission:unassign"
	// CommandGive gives one person permissions in their own right.
	CommandGive = "permission:give"
	// CommandForget makes every process re-read what it remembers.
	CommandForget = "permission:cache-reset"
)

// Commands returns this package's commands, sorted by name.
//
// They are added to an application's console explicitly:
//
//	console.NewApplication(os.Stdout, os.Stderr, os.Stdin).Add(module.Commands()...)
//
// Every one of them that writes takes --as, which is the identifier of the
// person it acts as. That person's permissions are read out of the database and
// are what the command may do -- so a command cannot do what the person named
// could not have done from the panel, and an operator who wants more asks for
// more the same way anybody else does.
//
// The one exception is the first group, and it is an exception to whether there
// is anybody to act as rather than to whether the rules apply. See Bootstrap.
func (m *Module) Commands() []console.Command {
	commands := []console.Command{
		{
			Signature: CommandShow +
				" {--as= : Who to act as; the listing is read with their permissions}" +
				" {--tenant= : The customer to read, defaulting to the configured one}",
			Description: "print every group against every permission",
			Run:         m.runShow,
		},
		{
			Signature: CommandCreateGroup +
				" {slug : The stable name, lowercase, letters digits - and _}" +
				" {name : What people read}" +
				" {--description= : Why the group exists}" +
				" {--as= : Who to act as; omitted, this creates the first group of the tenant}" +
				" {--member=* : Somebody to put in it, repeatable}" +
				" {--permission=* : A permission or selector to give it, repeatable}" +
				" {--tenant= : The customer to write in, defaulting to the configured one}",
			Description: "add a group, or the first group of an installation",
			Run:         m.runCreateGroup,
		},
		{
			Signature: CommandGrant +
				" {group : The slug of the group}" +
				" {selector* : A permission or a selector such as invoice.*}" +
				" {--as= : Who to act as}" +
				" {--tenant= : The customer to write in, defaulting to the configured one}",
			Description: "attach permissions to a group",
			Run:         m.runGrant,
		},
		{
			Signature: CommandRevoke +
				" {group : The slug of the group}" +
				" {selector* : A permission or a selector such as invoice.*}" +
				" {--as= : Who to act as}" +
				" {--tenant= : The customer to write in, defaulting to the configured one}",
			Description: "detach permissions from a group",
			Run:         m.runRevoke,
		},
		{
			Signature: CommandAssign +
				" {group : The slug of the group}" +
				" {user* : Somebody to put in it}" +
				" {--as= : Who to act as}" +
				" {--tenant= : The customer to write in, defaulting to the configured one}",
			Description: "put people in a group",
			Run:         m.runAssign,
		},
		{
			Signature: CommandUnassign +
				" {group : The slug of the group}" +
				" {user* : Somebody to take out of it}" +
				" {--as= : Who to act as}" +
				" {--tenant= : The customer to write in, defaulting to the configured one}",
			Description: "take people out of a group",
			Run:         m.runUnassign,
		},
		{
			Signature: CommandGive +
				" {user : Who to give them to}" +
				" {selector* : A permission or a selector such as invoice.*}" +
				" {--as= : Who to act as}" +
				" {--revoke : Take these back instead of giving them}" +
				" {--tenant= : The customer to write in, defaulting to the configured one}",
			Description: "give one person permissions in their own right",
			Run:         m.runGive,
		},
		{
			Signature:   CommandForget + " {--tenant= : The customer to move the token of} {--as= : Who to act as}",
			Description: "make every process re-read what a tenant may do",
			Run:         m.runForget,
		},
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i].Signature < commands[j].Signature })
	return commands
}

// runShow prints the matrix.
//
// It takes --as like every other command, and reading is the reason rather than
// the exception to it. A listing is where a permission system leaks: it says
// which groups exist, what each carries and therefore what the application can
// do -- and answering that to whoever runs the binary would be a read with no
// policy behind it, which is the one thing this package has none of.
func (m *Module) runShow(ctx context.Context, o *console.IO) error {
	tenant := m.tenantOf(o)
	actor, err := m.actorOf(ctx, o, tenant)
	if err != nil {
		return err
	}

	view, err := m.svc.ViewMatrix(ctx, actor, GroupQuery{Limit: MaxPageSize})
	if err != nil {
		return err
	}
	if len(view.Rows) == 0 {
		o.Comment("%s has no group. Create the first one with `%s`.", tenant, CommandCreateGroup)
		return nil
	}

	headers := make([]string, 0, len(view.Actions)+1)
	headers = append(headers, "group")
	for _, action := range view.Actions {
		headers = append(headers, string(action))
	}

	rows := make([][]string, 0, len(view.Rows))
	for _, row := range view.Rows {
		line := make([]string, 0, len(headers))
		name := row.Group.Slug
		if row.System {
			name += " (system)"
		}
		line = append(line, name)
		for _, cell := range row.Cells {
			// The two marks are a tick and a middle dot, which is what the
			// reference prints. A column of "yes" and "no" is unreadable at
			// forty permissions across.
			if cell.Held {
				line = append(line, "*")
				continue
			}
			line = append(line, ".")
		}
		rows = append(rows, line)
	}
	o.Table(headers, rows)
	if view.Next != "" {
		o.Comment("more groups follow; this prints the first %d by slug", MaxPageSize)
	}
	return nil
}

// runCreateGroup adds a group, or the first group of the tenant.
func (m *Module) runCreateGroup(ctx context.Context, o *console.IO) error {
	tenant := m.tenantOf(o)
	slug := strings.TrimSpace(o.Argument("slug").String())
	name := strings.TrimSpace(o.Argument("name").String())
	description := strings.TrimSpace(o.Option("description").String())
	members := trimmed(o.Option("member").Slice())

	wanted, err := m.selected(o.Option("permission").Slice())
	if err != nil {
		return err
	}

	// No --as is the first group, and it is refused the moment there is one.
	if strings.TrimSpace(o.Option("as").String()) == "" {
		record, err := m.svc.Bootstrap(ctx, tenant, BootstrapRequest{
			Slug:        slug,
			Name:        name,
			Description: description,
			Actions:     wanted,
			Members:     members,
		})
		if err != nil {
			return err
		}
		o.Info("created %s, the first group of %s, carrying the administration of permissions", record.Slug, tenant)
		o.Comment("this command cannot create another without --as: %s now has a group, so there is somebody to act as", tenant)
		return nil
	}

	actor, err := m.actorOf(ctx, o, tenant)
	if err != nil {
		return err
	}
	record, err := m.svc.CreateGroup(ctx, actor, CreateGroupRequest{
		Slug:        slug,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return err
	}
	o.Info("created %s in %s", record.Slug, tenant)

	if len(wanted) > 0 {
		if _, err := m.svc.SetActions(ctx, actor, record.ID, wanted); err != nil {
			return err
		}
		o.Line("granted %d permission(s)", len(wanted))
	}
	if len(members) > 0 {
		if _, err := m.svc.SetMembers(ctx, actor, record.ID, members); err != nil {
			return err
		}
		o.Line("put %d person(s) in it", len(members))
	}
	return nil
}

// runGrant attaches permissions to a group.
func (m *Module) runGrant(ctx context.Context, o *console.IO) error {
	return m.changeActions(ctx, o, true)
}

// runRevoke detaches them.
func (m *Module) runRevoke(ctx context.Context, o *console.IO) error {
	return m.changeActions(ctx, o, false)
}

// changeActions is both of the above.
//
// One function because they are one write: SetActions takes the whole set, so
// granting is the set plus what the selectors named and revoking is the set
// minus it. Two implementations of that arithmetic would drift, and the one that
// drifted would be the one that takes permissions away.
func (m *Module) changeActions(ctx context.Context, o *console.IO, attach bool) error {
	tenant := m.tenantOf(o)
	actor, err := m.actorOf(ctx, o, tenant)
	if err != nil {
		return err
	}
	named, err := m.selected(o.Argument("selector").Slice())
	if err != nil {
		return err
	}

	record, err := m.groupBySlug(ctx, actor, o.Argument("group").String())
	if err != nil {
		return err
	}
	current, err := m.svc.ActionsOf(ctx, actor, record.ID)
	if err != nil {
		return err
	}

	change, err := m.svc.SetActions(ctx, actor, record.ID, merge(current, named, attach))
	if err != nil {
		return err
	}
	report(o, change)
	return nil
}

// runAssign puts people in a group.
func (m *Module) runAssign(ctx context.Context, o *console.IO) error {
	return m.changeMembers(ctx, o, true)
}

// runUnassign takes them out.
func (m *Module) runUnassign(ctx context.Context, o *console.IO) error {
	return m.changeMembers(ctx, o, false)
}

// changeMembers is both of the above, for the reason changeActions is both of
// its two.
func (m *Module) changeMembers(ctx context.Context, o *console.IO, attach bool) error {
	tenant := m.tenantOf(o)
	actor, err := m.actorOf(ctx, o, tenant)
	if err != nil {
		return err
	}

	record, err := m.groupBySlug(ctx, actor, o.Argument("group").String())
	if err != nil {
		return err
	}
	current, err := m.svc.MembersOf(ctx, actor, record.ID)
	if err != nil {
		return err
	}

	named := trimmed(o.Argument("user").Slice())
	change, err := m.svc.SetMembers(ctx, actor, record.ID, mergeStrings(current, named, attach))
	if err != nil {
		return err
	}
	report(o, change)
	return nil
}

// runGive gives one person permissions in their own right, or takes them back.
func (m *Module) runGive(ctx context.Context, o *console.IO) error {
	tenant := m.tenantOf(o)
	actor, err := m.actorOf(ctx, o, tenant)
	if err != nil {
		return err
	}
	named, err := m.selected(o.Argument("selector").Slice())
	if err != nil {
		return err
	}

	userID := strings.TrimSpace(o.Argument("user").String())
	current, err := m.svc.DirectActionsOf(ctx, actor, userID)
	if err != nil {
		return err
	}

	change, err := m.svc.SetDirectActions(ctx, actor, userID, merge(current, named, !o.Option("revoke").Bool()))
	if err != nil {
		return err
	}
	report(o, change)
	return nil
}

// runForget moves the tenant's token, so every process re-reads what it
// remembers.
//
// The reference clears a shared cache; there is no shared cache here to clear.
// What every process consults is the token, so moving it is the same
// instruction, and it reaches replicas this command never talks to.
//
// It is not what makes revocation correct. Every write moves the token already.
// This is for the operator who changed a row by hand and wants the running
// system to notice.
func (m *Module) runForget(ctx context.Context, o *console.IO) error {
	tenant := m.tenantOf(o)
	actor, err := m.actorOf(ctx, o, tenant)
	if err != nil {
		return err
	}

	before, err := m.svc.Version(ctx, actor)
	if err != nil {
		return err
	}
	after, err := m.svc.Refresh(ctx, actor)
	if err != nil {
		return err
	}

	m.roles.Forget()
	o.Info("%s moved from %d to %d; every process re-reads on its next request", tenant, before, after)
	return nil
}

// tenantOf is the customer a command acts in.
//
// It comes from a flag or from the configuration, and from nowhere else. A
// terminal is not a request, so REGRA-style suspicion of the caller does not
// apply the same way -- but the default is still the configured one, so an
// operator who forgets the flag writes where the application already writes
// rather than somewhere they did not name.
func (m *Module) tenantOf(o *console.IO) string {
	if named := strings.TrimSpace(o.Option("tenant").String()); named != "" {
		return named
	}
	return m.cfg.Tenant
}

// actorOf is who a command acts as.
//
// The identifier names a person, and what that person may do is read out of the
// database rather than asserted by the flag. It is the whole of why these
// commands are not an escape hatch: whoever runs them can do what the person
// named could have done from the panel, and nothing more.
//
// A command run with no --as acts as nobody, which every policy refuses. That is
// the safe direction and it is deliberate: the one thing that has to work
// without a person is the first group, and Bootstrap is where that lives.
func (m *Module) actorOf(ctx context.Context, o *console.IO, tenant string) (security.Subject, error) {
	subject := security.Subject{
		ID:       strings.TrimSpace(o.Option("as").String()),
		Tenant:   tenant,
		Verified: true,
	}
	if subject.ID == "" {
		return subject, fmt.Errorf("permission: --as is required: a command acts as somebody, and what they may do is read out of the database")
	}

	actions, err := m.roles.Resolve(ctx, subject)
	if err != nil {
		return security.Subject{}, err
	}
	subject.Actions = actions
	return subject, nil
}

// groupBySlug finds a group by the name an operator types.
//
// The panel addresses a group by its identifier because a link carries one; a
// person at a terminal has the slug, which is the name they chose. Both reach
// the same row through the same authorized listing.
func (m *Module) groupBySlug(ctx context.Context, actor security.Subject, slug string) (GroupRef, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return GroupRef{}, fmt.Errorf("permission: no group was named")
	}

	page, err := m.svc.ListGroups(ctx, actor, GroupQuery{Search: slug, Limit: MaxPageSize})
	if err != nil {
		return GroupRef{}, err
	}
	for _, item := range page.Items {
		if item.Slug == slug {
			return item, nil
		}
	}
	return GroupRef{}, fmt.Errorf("%w: no group of this tenant is called %s", ErrNotFound, slug)
}

// selected turns what an operator typed into actions.
//
// Every argument is read as a selector, including one that is simply an action:
// "invoice.delete" names itself, so there is one syntax rather than two and
// nothing has to decide which of them a given string is.
func (m *Module) selected(selectors []string) ([]security.Action, error) {
	catalogue := m.svc.Catalogue()
	seen := map[security.Action]bool{}
	var out []security.Action

	for _, selector := range trimmed(selectors) {
		matched, err := catalogue.Match(selector)
		if err != nil {
			return nil, err
		}
		for _, action := range matched {
			if seen[action] {
				continue
			}
			seen[action] = true
			out = append(out, action)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// report prints what a write did.
func report(o *console.IO, change Change) {
	if change.Empty() {
		o.Comment("nothing changed")
		return
	}
	if len(change.Added) > 0 {
		o.Info("added: %s", strings.Join(change.Added, ", "))
	}
	if len(change.Removed) > 0 {
		o.Info("removed: %s", strings.Join(change.Removed, ", "))
	}
}

// merge is the set a write is given: what is there now, plus or minus what was
// named.
func merge(current, named []security.Action, attach bool) []security.Action {
	have := make([]string, 0, len(current))
	for _, action := range current {
		have = append(have, string(action))
	}
	want := make([]string, 0, len(named))
	for _, action := range named {
		want = append(want, string(action))
	}

	merged := mergeStrings(have, want, attach)
	out := make([]security.Action, 0, len(merged))
	for _, value := range merged {
		out = append(out, security.Action(value))
	}
	return out
}

// mergeStrings is merge over identifiers, and what merge is written in terms of.
func mergeStrings(current, named []string, attach bool) []string {
	set := make(map[string]bool, len(current))
	for _, value := range current {
		set[value] = true
	}
	for _, value := range named {
		if attach {
			set[value] = true
			continue
		}
		delete(set, value)
	}

	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// trimmed drops the blanks a shell leaves behind.
func trimmed(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
