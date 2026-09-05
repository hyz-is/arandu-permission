// Package permission administers who may do what, and decides nothing.
//
// The authority over every action stays where it already was: a policy written
// in Go, reached through a Grant, refusing by default. What this package owns is
// the administration of that -- named groups, the actions each group carries,
// the people in them -- and the middleware that puts the result on the acting
// subject before any policy runs.
//
// There is no table of permissions and there is no way to make one. The set of
// actions a group may carry is the application's own code, handed in as
// Config.Actions, and a write naming anything outside it is refused. A row here
// links a group to a permission that already exists because some policy reads
// it; it cannot bring one into being.
//
// The files are laid out by role rather than by layer, so the whole package
// reads top to bottom:
//
//	module.go      -> registration, routes, handlers and migrations
//	config.go      -> what the application passes in
//	catalogue.go   -> the closed set of actions a group may carry
//	model.go       -> the entities, and what they may answer with
//	policy.go      -> who may do what
//	service.go     -> the rules and Model access, after authorization
//	resolver.go    -> what a request carries into every policy
//	views.go       -> the files the application takes ownership of
//
// An application registers it explicitly. There is no service provider, no
// container and no discovery: the wiring is a few lines somebody wrote, and
// reading them is how they learn what the application is made of.
//
// The first group has to come from outside a request, because nobody can
// administer permissions until somebody carries them. A seed builds the subject
// it acts as -- an identifier, the tenant, and the actions of this package as
// its roles -- and calls the same service every screen calls. There is no second
// write path and no escape hatch: the seed is authorized by the same policies,
// by a subject the operator running it constructed.
package permission

import (
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strings"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/foundation"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/framework/validation"
	"github.com/arandu-io/hesape/auth"
	"github.com/arandu-io/hesape/database/migrations"
	"github.com/arandu-io/hesape/database/schema"
	"github.com/arandu-io/hesape/translation"
	hview "github.com/arandu-io/hesape/view"
)

// The names the published views are rendered by.
//
// They are derived from the paths in the archive, and the archive is what the
// publication writes, so a view that moved cannot keep an old name here without
// the module refusing to boot.
const (
	// ViewGroupsIndex is the listing, with its search and its pages.
	ViewGroupsIndex = "vendor.permission.groups.index"
	// ViewGroupsShow is one group: what it carries and who is in it.
	ViewGroupsShow = "vendor.permission.groups.show"
	// ViewCatalogue is every action the application declares, by domain.
	ViewCatalogue = "vendor.permission.catalogue.index"
	// ViewMatrix is the grid of groups against actions.
	ViewMatrix = "vendor.permission.matrix.index"
	// ViewSummary is the fragment that says what a bulk write would change,
	// before it is applied.
	ViewSummary = "vendor.permission.matrix.summary"
	// ViewMember is one person's effective permissions and where each comes
	// from.
	ViewMember = "vendor.permission.users.show"
)

// Module is what the application registers.
//
// It implements foundation.Module, which is Name and Routes and nothing else --
// that pair is the whole public contract between a package and the framework.
//
// It also implements foundation.Migratable, because it owns tables, and
// foundation.Publishable, because it hands view sources to the project.
type Module struct {
	cfg      Config
	svc      *PermissionService
	roles    *Resolver
	sessions *security.SessionStore
	csrf     *security.CSRF
}

// Compile-time proof that the module honors the contracts it claims.
var (
	_ foundation.Module      = (*Module)(nil)
	_ foundation.Migratable  = (*Module)(nil)
	_ foundation.Bootable    = (*Module)(nil)
	_ foundation.Publishable = (*Module)(nil)
)

// New returns the module, or the reason it cannot be built.
//
// The collaborators are parameters and not fields somebody fills in afterwards:
// a module that could be registered half-wired is a module whose first request
// is the thing that reports the missing half.
//
// It returns an error rather than panicking or carrying on, because everything
// it refuses is a wiring mistake, and a wiring mistake found at boot costs one
// restart. The same mistake found later is a request that reached a nil handle.
func New(cfg Config, db *data.DB, sessions *security.SessionStore, csrf *security.CSRF) (*Module, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("permission: New needs a database handle: this package owns tables, and there is no in-memory mode that would let it start without one")
	}
	if sessions == nil {
		return nil, errors.New("permission: New needs a session store: it is where the subject comes from, and a request with no subject cannot be authorized")
	}
	if csrf == nil {
		return nil, errors.New("permission: New needs the CSRF issuer: every screen here writes, and a form with no token is a form the application refuses")
	}

	catalogue, err := NewCatalogue(cfg.Actions...)
	if err != nil {
		return nil, err
	}
	cfg = cfg.withDefaults()
	service := NewPermissionService(db, catalogue, cfg.Listeners...)
	return &Module{
		cfg:      cfg,
		svc:      service,
		roles:    NewResolver(service, cfg.CacheSize),
		sessions: sessions,
		csrf:     csrf,
	}, nil
}

// Name is the module identifier: a lowercase slug, stable, no spaces.
//
// It is what route listings group by and what the route names are prefixed
// with, so changing it changes addresses that other code has already written
// down.
func (m *Module) Name() string { return "permission" }

// routePattern is one address this module answers at, relative to the
// configured prefix.
type routePattern struct {
	method string
	path   string
}

// routePatterns is every address the module registers.
//
// It exists so that the configuration check can parse the exact patterns before
// anything is mounted, and it is compared against what Routes actually
// registers by a test -- because two lists of the same addresses is exactly the
// shape that drifts.
func routePatterns(prefix string) []routePattern {
	return []routePattern{
		{stdhttp.MethodGet, prefix + "/groups"},
		{stdhttp.MethodPost, prefix + "/groups"},
		{stdhttp.MethodGet, prefix + "/groups/{group}"},
		{stdhttp.MethodPut, prefix + "/groups/{group}"},
		{stdhttp.MethodDelete, prefix + "/groups/{group}"},
		{stdhttp.MethodPost, prefix + "/groups/{group}/summary"},
		{stdhttp.MethodPut, prefix + "/groups/{group}/actions"},
		{stdhttp.MethodPut, prefix + "/groups/{group}/members"},
		{stdhttp.MethodGet, prefix + "/catalogue"},
		{stdhttp.MethodGet, prefix + "/matrix"},
		{stdhttp.MethodGet, prefix + "/users/{user}"},
		{stdhttp.MethodPost, prefix + "/users/{user}/summary"},
		{stdhttp.MethodPut, prefix + "/users/{user}/actions"},
	}
}

// Routes registers the module's routes under the configured prefix.
//
// They are named, so a URL is built from a name rather than written out a
// second time somewhere else -- two spellings of one address disagree, and the
// failure when they do is a link to a 404.
//
// Each one declares the action it requires. The declaration decides nothing:
// the handler still asks the service, the service still asks the policy, and
// the policy is still what refuses. What it is for is the catalogue -- a screen
// that lists what may be granted reads the router rather than a list beside it,
// and a list beside it is wrong the first time somebody adds a route.
//
// The whole group runs behind the middleware that fills in what the subject may
// do, so the panel works whether or not the application mounted it globally.
// Mounting it twice costs nothing: the second pass finds the answer the first
// one remembered.
func (m *Module) Routes(r *fhttp.Router) {
	guarded := r.Group("", m.Roles())

	guarded.Action(stdhttp.MethodGet, m.cfg.Prefix+"/groups", m.index).
		Name("permission.index").Can(PermissionList)
	guarded.Action(stdhttp.MethodPost, m.cfg.Prefix+"/groups", m.store).
		Name("permission.store").Can(PermissionCreate)
	guarded.Action(stdhttp.MethodGet, m.cfg.Prefix+"/groups/{group}", m.show).
		Name("permission.show").Can(PermissionView)
	guarded.Action(stdhttp.MethodPut, m.cfg.Prefix+"/groups/{group}", m.update).
		Name("permission.update").Can(PermissionUpdate)
	guarded.Action(stdhttp.MethodDelete, m.cfg.Prefix+"/groups/{group}", m.destroy).
		Name("permission.destroy").Can(PermissionDelete)
	guarded.Action(stdhttp.MethodPost, m.cfg.Prefix+"/groups/{group}/summary", m.summary).
		Name("permission.summary").Can(PermissionView)
	guarded.Action(stdhttp.MethodPut, m.cfg.Prefix+"/groups/{group}/actions", m.grant).
		Name("permission.grant").Can(PermissionGrant)
	guarded.Action(stdhttp.MethodPut, m.cfg.Prefix+"/groups/{group}/members", m.assign).
		Name("permission.assign").Can(PermissionAssign)
	guarded.Action(stdhttp.MethodGet, m.cfg.Prefix+"/catalogue", m.catalogue).
		Name("permission.catalogue").Can(PermissionList)
	guarded.Action(stdhttp.MethodGet, m.cfg.Prefix+"/matrix", m.matrix).
		Name("permission.matrix").Can(PermissionList)
	guarded.Action(stdhttp.MethodGet, m.cfg.Prefix+"/users/{user}", m.member).
		Name("permission.member").Can(PermissionView)
	guarded.Action(stdhttp.MethodPost, m.cfg.Prefix+"/users/{user}/summary", m.memberSummary).
		Name("permission.member.summary").Can(PermissionView)
	guarded.Action(stdhttp.MethodPut, m.cfg.Prefix+"/users/{user}/actions", m.grantDirect).
		Name("permission.member.grant").Can(PermissionGrantDirect)
}

// PublishCommand is what an application runs to take ownership of the views
// this package offers.
//
// It is spelled out as a constant so that whatever says it -- the refusal
// below, or a message an application writes for its own operators -- says one
// thing. A person who is told two different commands for one job tries both.
const PublishCommand = "aru vendor:publish --apply"

// Boot refuses to serve when a view this package renders is not in the binary.
//
// A compiled view registers itself from init(), so by the time anything boots
// the question has one answer already: either the application published the
// files, compiled them and imported the package they became, or it did not.
// Asking here turns "did anybody run the install command" into one refusal at
// start-up that names the views and the command, instead of a 500 on the first
// request that reached one of them.
//
// It also holds the destination. Every file the archive offers has to land
// under the vendor directory named after this module: an archive that reached
// resources/views/home.kyse.go would land on a page the application wrote, and
// what publishes the files writes what the archive says.
func (m *Module) Boot(context.Context) error {
	prefix := viewRoot + "/" + vendorDir + "/" + m.Name() + "/"
	var stray []string
	for _, path := range PublishedPaths() {
		if !strings.HasPrefix(path, prefix) {
			stray = append(stray, path)
		}
	}
	if len(stray) > 0 {
		return fmt.Errorf("permission: %s would be published outside %s, where it lands on a file the application wrote",
			strings.Join(stray, ", "), prefix)
	}

	registered := make(map[string]bool)
	for _, name := range hview.Registered() {
		registered[name] = true
	}
	var missing []string
	for _, name := range ViewNames() {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// The compiled packages are named because the import is the half of the
		// install nothing else can do: the command writes the sources, the view
		// compiler turns them into Go, and a package the application does not
		// import is not linked at all.
		imports := make([]string, 0, len(ViewPackages()))
		for _, pkg := range ViewPackages() {
			imports = append(imports, "<module path>/"+pkg)
		}
		return fmt.Errorf("permission: no view is registered as %s. Run `%s`, then `aru view:build`, then import %s in bootstrap/app.go",
			strings.Join(missing, ", "), PublishCommand, strings.Join(imports, ", "))
	}
	return nil
}

// The data a screen is drawn from.
//
// They are declared here rather than in the published markup, and the markup
// declares an alias to each. That is what lets the module render its own
// screens: a compiled view asserts the data back to the type its source
// declared, and a type declared in the project would be a type this package
// cannot construct.
//
// Every one of them embeds the page chrome, which is what the application's
// layout draws around them.

// GroupsPageData is the listing.
type GroupsPageData struct {
	hview.Page

	// Prefix is where this module answers, so the markup composes its own
	// addresses instead of hard-coding one the configuration can change.
	Prefix string
	// Labels are the sentences this screen draws, resolved for the locale the
	// request asked for.
	Labels Labels
	// Search is the term the listing was narrowed by, echoed back into the
	// field so the box still says what is being looked at.
	Search string
	// Groups are the rows, and Next is the cursor of the following page, empty
	// on the last one.
	Groups []GroupRef
	Next   string
	// Rejected holds the field errors of a create that was refused, so the form
	// comes back with the reason on it rather than blank.
	Rejected validation.Errors
}

// ActionChoice is one action on the group screen, and whether the group carries
// it.
type ActionChoice struct {
	Action security.Action
	Held   bool
}

// ActionSection is one domain of the catalogue as the group screen draws it.
type ActionSection struct {
	// Domain is the part before the first dot, shared by every action below.
	Domain  string
	Choices []ActionChoice
}

// GroupPageData is one group: what it carries, and who is in it.
type GroupPageData struct {
	hview.Page

	Prefix string
	Labels Labels
	Group  GroupRef
	// System says the group cannot be deleted and cannot have actions taken
	// off it, so the screen draws those controls as unavailable rather than
	// offering a change the server refuses.
	System bool
	// Description is what the group is for.
	Description string
	// Sections are every action of the catalogue, by domain, each saying
	// whether this group carries it.
	Sections []ActionSection
	// Members are who is in the group.
	Members []string
}

// CataloguePageData is every action the application declares, by domain, and
// the groups that carry each.
type CataloguePageData struct {
	hview.Page

	Prefix  string
	Labels  Labels
	Domains []CatalogueDomain
}

// MatrixPageData is the grid of groups against actions.
type MatrixPageData struct {
	hview.Page

	Prefix string
	Labels Labels
	Search string
	Matrix MatrixView
}

// SummaryPageData is what a bulk write would change, drawn before it is
// applied.
//
// It is a fragment: it extends no layout, because it is swapped into a page
// that already has one.
type SummaryPageData struct {
	// Prefix is where this module answers.
	Prefix string
	// Labels are the sentences this fragment draws.
	Labels Labels
	// Target is the address the confirmed write is sent to, composed by the
	// handler that drew the summary.
	//
	// It is here rather than assembled in the markup because two screens are
	// summarised by one fragment, and an address built from a branch inside the
	// template is an address that is wrong for whichever branch nobody tested.
	Target string
	// Group is the group the change is about, empty when the change is about
	// one person's own grants.
	Group GroupRef
	// Kind is "actions" or "members", which is what decides where the
	// confirmation goes and what the counts are called.
	Kind string
	// Change is the difference, from the same computation the write uses.
	Change Change
	// Fields are the values the confirmation resubmits, so that approving the
	// summary sends the same request that produced it.
	Fields []string
}

// MemberPageData is one person's effective permissions and where each of them
// comes from.
type MemberPageData struct {
	hview.Page

	Prefix    string
	Labels    Labels
	Effective Effective
	// Sections are every action of the catalogue, by domain, each saying
	// whether this person carries it in their own right.
	//
	// What a group confers is not ticked here. The box edits one table, and a
	// box that came back ticked because a group granted the action would be a
	// box somebody unticks expecting the permission to go away.
	Sections []ActionSection
}

// Handlers are thin on purpose: read the input, ask the service, answer. No
// rule, database handle or Model construction lives here. A handler that
// reached data directly would skip the service's policy boundary, and the
// layout makes that visible rather than relying on review.

// index answers a page of groups.
func (m *Module) index(ctx *fhttp.Context) error {
	search := ctx.Query("q")
	page, err := m.svc.ListGroups(ctx.Ctx(), m.subject(ctx.Request), GroupQuery{
		Search: search,
		Cursor: ctx.Query("cursor"),
		Limit:  m.cfg.PageSize,
	})
	if err != nil {
		return m.answer(ctx, err)
	}
	labels := m.Labels(m.locale(ctx.Request))
	return ctx.View(ViewGroupsIndex, GroupsPageData{
		Page:   m.page(ctx, labels.T("groups.title")),
		Prefix: m.cfg.Prefix,
		Labels: labels,
		Search: search,
		Groups: page.Items,
		Next:   page.Next,
	})
}

// store creates one group.
//
// It builds the request from named fields, and System is not one of them: a
// group the installation depends on is declared by whatever seeds the
// installation, and a form that could declare one would let anybody make a
// group nobody can delete.
func (m *Module) store(ctx *fhttp.Context) error {
	in := CreateGroupRequest{
		Slug:        strings.TrimSpace(ctx.Input("slug")),
		Name:        strings.TrimSpace(ctx.Input("name")),
		Description: strings.TrimSpace(ctx.Input("description")),
	}

	record, err := m.svc.CreateGroup(ctx.Ctx(), m.subject(ctx.Request), in)
	if err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Redirect(m.cfg.Prefix + "/groups/" + record.ID)
}

// show answers one group with what it carries and who is in it.
func (m *Module) show(ctx *fhttp.Context) error {
	actor := m.subject(ctx.Request)
	id := ctx.Param("group")

	record, err := m.svc.FindGroup(ctx.Ctx(), actor, id)
	if err != nil {
		return m.answer(ctx, err)
	}
	held, err := m.svc.ActionsOf(ctx.Ctx(), actor, id)
	if err != nil {
		return m.answer(ctx, err)
	}
	members, err := m.svc.MembersOf(ctx.Ctx(), actor, id)
	if err != nil {
		return m.answer(ctx, err)
	}

	labels := m.Labels(m.locale(ctx.Request))
	return ctx.View(ViewGroupsShow, GroupPageData{
		Page:        m.page(ctx, record.Name),
		Prefix:      m.cfg.Prefix,
		Labels:      labels,
		Group:       refOf(record),
		System:      record.IsSystem,
		Description: record.Description,
		Sections:    m.sections(held),
		Members:     members,
	})
}

// update changes what a group is called.
func (m *Module) update(ctx *fhttp.Context) error {
	in := UpdateGroupRequest{
		Name:        strings.TrimSpace(ctx.Input("name")),
		Description: strings.TrimSpace(ctx.Input("description")),
	}

	record, err := m.svc.UpdateGroup(ctx.Ctx(), m.subject(ctx.Request), ctx.Param("group"), in)
	if err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Redirect(m.cfg.Prefix + "/groups/" + record.ID)
}

// destroy removes one group.
func (m *Module) destroy(ctx *fhttp.Context) error {
	if err := m.svc.DeleteGroup(ctx.Ctx(), m.subject(ctx.Request), ctx.Param("group")); err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Redirect(m.cfg.Prefix + "/groups")
}

// summary answers what a bulk write would change, and writes nothing.
//
// It is the screen somebody reads before approving a change to twelve
// permissions at once. It is drawn from the same computation the write uses, so
// what was approved is what happens.
func (m *Module) summary(ctx *fhttp.Context) error {
	actor := m.subject(ctx.Request)
	id := ctx.Param("group")
	kind, fields, err := m.submitted(ctx)
	if err != nil {
		return m.answer(ctx, err)
	}

	record, err := m.svc.FindGroup(ctx.Ctx(), actor, id)
	if err != nil {
		return m.answer(ctx, err)
	}

	var change Change
	if kind == kindActions {
		change, err = m.svc.PreviewActions(ctx.Ctx(), actor, id, actionsOf(fields))
	} else {
		change, err = m.svc.PreviewMembers(ctx.Ctx(), actor, id, fields)
	}
	if err != nil {
		return m.answer(ctx, err)
	}

	return ctx.Fragment(stdhttp.StatusOK, ViewSummary, SummaryPageData{
		Prefix: m.cfg.Prefix,
		Labels: m.Labels(m.locale(ctx.Request)),
		Target: m.cfg.Prefix + "/groups/" + record.ID + "/" + kind,
		Group:  refOf(record),
		Kind:   kind,
		Change: change,
		Fields: fields,
	})
}

// grant makes a group carry exactly the actions that were submitted.
func (m *Module) grant(ctx *fhttp.Context) error {
	_, fields, err := m.submitted(ctx)
	if err != nil {
		return m.answer(ctx, err)
	}
	id := ctx.Param("group")
	if _, err := m.svc.SetActions(ctx.Ctx(), m.subject(ctx.Request), id, actionsOf(fields)); err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Redirect(m.cfg.Prefix + "/groups/" + id)
}

// assign makes a group hold exactly the people who were submitted.
func (m *Module) assign(ctx *fhttp.Context) error {
	_, fields, err := m.submitted(ctx)
	if err != nil {
		return m.answer(ctx, err)
	}
	id := ctx.Param("group")
	if _, err := m.svc.SetMembers(ctx.Ctx(), m.subject(ctx.Request), id, fields); err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Redirect(m.cfg.Prefix + "/groups/" + id)
}

// catalogue answers every action the application declares, by domain.
func (m *Module) catalogue(ctx *fhttp.Context) error {
	view, err := m.svc.ViewCatalogue(ctx.Ctx(), m.subject(ctx.Request))
	if err != nil {
		return m.answer(ctx, err)
	}
	labels := m.Labels(m.locale(ctx.Request))
	return ctx.View(ViewCatalogue, CataloguePageData{
		Page:    m.page(ctx, labels.T("catalogue.title")),
		Prefix:  m.cfg.Prefix,
		Labels:  labels,
		Domains: view.Domains,
	})
}

// matrix answers a page of groups against every action.
func (m *Module) matrix(ctx *fhttp.Context) error {
	search := ctx.Query("q")
	grid, err := m.svc.ViewMatrix(ctx.Ctx(), m.subject(ctx.Request), GroupQuery{
		Search: search,
		Cursor: ctx.Query("cursor"),
		Limit:  m.cfg.PageSize,
	})
	if err != nil {
		return m.answer(ctx, err)
	}
	labels := m.Labels(m.locale(ctx.Request))
	return ctx.View(ViewMatrix, MatrixPageData{
		Page:   m.page(ctx, labels.T("matrix.title")),
		Prefix: m.cfg.Prefix,
		Labels: labels,
		Search: search,
		Matrix: grid,
	})
}

// member answers one person's effective permissions and where each comes from.
func (m *Module) member(ctx *fhttp.Context) error {
	actor := m.subject(ctx.Request)
	userID := ctx.Param("user")

	effective, err := m.svc.EffectiveFor(ctx.Ctx(), actor, userID)
	if err != nil {
		return m.answer(ctx, err)
	}
	own, err := m.svc.DirectActionsOf(ctx.Ctx(), actor, userID)
	if err != nil {
		return m.answer(ctx, err)
	}

	labels := m.Labels(m.locale(ctx.Request))
	return ctx.View(ViewMember, MemberPageData{
		Page:      m.page(ctx, labels.T("member.title")),
		Prefix:    m.cfg.Prefix,
		Labels:    labels,
		Effective: effective,
		Sections:  m.sections(own),
	})
}

// memberSummary answers what a change to one person's own grants would do, and
// writes nothing.
func (m *Module) memberSummary(ctx *fhttp.Context) error {
	userID := ctx.Param("user")
	_, fields, err := m.submitted(ctx)
	if err != nil {
		return m.answer(ctx, err)
	}

	change, err := m.svc.PreviewDirectActions(ctx.Ctx(), m.subject(ctx.Request), userID, actionsOf(fields))
	if err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Fragment(stdhttp.StatusOK, ViewSummary, SummaryPageData{
		Prefix: m.cfg.Prefix,
		Labels: m.Labels(m.locale(ctx.Request)),
		Target: m.cfg.Prefix + "/users/" + userID + "/actions",
		Kind:   kindActions,
		Change: change,
		Fields: fields,
	})
}

// grantDirect makes one person carry exactly the actions that were submitted,
// in their own right.
func (m *Module) grantDirect(ctx *fhttp.Context) error {
	_, fields, err := m.submitted(ctx)
	if err != nil {
		return m.answer(ctx, err)
	}
	userID := ctx.Param("user")
	if _, err := m.svc.SetDirectActions(ctx.Ctx(), m.subject(ctx.Request), userID, actionsOf(fields)); err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Redirect(m.cfg.Prefix + "/users/" + userID)
}

// sections is the whole catalogue by domain, each action saying whether it is
// in the set given.
//
// It draws two screens: what a group carries, and what one person carries in
// their own right. They are the same question about different rows, and one
// builder is what keeps the second screen from quietly listing a different set
// of permissions than the first.
func (m *Module) sections(held []security.Action) []ActionSection {
	carries := make(map[security.Action]bool, len(held))
	for _, action := range held {
		carries[action] = true
	}
	sections := make([]ActionSection, 0)
	for _, domain := range m.svc.Catalogue().Domains() {
		section := ActionSection{Domain: domain.Name}
		for _, action := range domain.Actions {
			section.Choices = append(section.Choices, ActionChoice{Action: action, Held: carries[action]})
		}
		sections = append(sections, section)
	}
	return sections
}

// The two kinds of bulk write a screen submits.
const (
	kindActions = "actions"
	kindMembers = "members"
)

// submitted reads a bulk form: which kind of write it is, and the values it
// names.
//
// A form that ticks no box submits no field at all, which is the request that
// takes everything off a group -- so an absent field is an empty list and never
// a reason to skip the write. That distinction is why this reads the parsed
// form rather than asking for one value.
func (m *Module) submitted(ctx *fhttp.Context) (string, []string, error) {
	if err := ctx.Request.ParseForm(); err != nil {
		return "", nil, err
	}
	kind := ctx.Request.PostForm.Get("kind")
	if kind == "" {
		kind = kindActions
	}
	if kind != kindActions && kind != kindMembers {
		return "", nil, validation.Errors{"kind": {"has to be actions or members"}}
	}

	values := ctx.Request.PostForm["value"]
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return kind, out, nil
}

// actionsOf reads submitted strings as actions. Nothing is validated here: the
// service checks every one against the catalogue, which is the only place that
// set is read.
func actionsOf(values []string) []security.Action {
	out := make([]security.Action, 0, len(values))
	for _, value := range values {
		out = append(out, security.Action(value))
	}
	return out
}

// subject reads who is acting, from the request's context and from nowhere
// else.
//
// The middleware that fills in the actions leaves the subject there. A handler
// that loaded the session itself would get the subject as it was stored,
// carrying no actions at all, and every policy would refuse -- which is the
// safe direction, and still the wrong answer.
//
// A request with no subject on it is a declared guest and not an empty subject.
// The difference matters: an empty subject is refused before the policy is
// consulted, because it is almost always a session that failed to load, and a
// policy asked about nobody answers about nobody. A guest reaches the policy and
// is refused there, by a rule somebody wrote.
//
// The tenant of that guest is the application's, from configuration. It is the
// one place a tenant does not come from a Grant, and it is because there is no
// Grant yet: everywhere downstream, data.Tenant is what the statements take.
func (m *Module) subject(r *stdhttp.Request) security.Subject {
	if sub, carried := auth.SubjectFrom(r.Context()); carried && sub.ID != "" {
		return sub
	}
	return security.Guest(m.cfg.Tenant)
}

// locale is what this request asked to be answered in.
//
// It comes from the request's context, where the negotiation middleware left
// it. An application that mounted none leaves it empty and the shipped locale is
// what is drawn -- a panel in the wrong language is still a panel, and refusing
// the request would be worse than answering it in English.
func (m *Module) locale(r *stdhttp.Request) string {
	return translation.Locale(r.Context())
}

// page is the chrome the application's layout draws around a screen.
func (m *Module) page(ctx *fhttp.Context, title string) hview.Page {
	actor := m.subject(ctx.Request)
	token, err := m.csrf.Issue(m.sessions.IDFromRequest(ctx.Request))
	if err != nil {
		// An unissued token is left empty rather than reported. The page still
		// renders and every form on it is refused, which is what a missing
		// session means -- and the alternative, failing the read because the
		// write would fail, is a blank screen where a sign-in prompt belongs.
		token = ""
	}
	return hview.Page{
		Title:         title,
		Token:         token,
		Authenticated: actor.ID != "",
		Path:          ctx.Request.URL.Path,
	}
}

// answer turns what the service refused into something the client can act on.
//
// A refusal is answered with a status and no detail. Telling the client why a
// policy said no is telling them what exists and what does not, one request at
// a time; the reason is in the log, where the person operating the system reads
// it and the person probing it does not.
//
// The refusals that are not authorization are answered as themselves. An action
// outside the catalogue is not a permission somebody lacks, it is a permission
// that does not exist, and answering it as 403 sends them looking for who to
// ask. Emptying a system group is a state that is refused, not a subject who
// was.
//
// Naming each one is what keeps a client-caused refusal out of the 500s. A
// request that names one member too many, or an identifier that cannot be one,
// used to fall through to a server error -- and a person told the server failed
// sends the same request again.
func (m *Module) answer(ctx *fhttp.Context, err error) error {
	switch {
	case errors.Is(err, security.ErrForbidden):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusForbidden, "forbidden")
		return nil
	case errors.Is(err, ErrNotFound):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusNotFound, "not found")
		return nil
	case errors.Is(err, ErrUnknownAction):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusUnprocessableEntity, "the action is not one this application declares")
		return nil
	case errors.Is(err, ErrLastMember):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusConflict, "a system group cannot be left without a member")
		return nil
	case errors.Is(err, ErrSlugTaken):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusConflict, "a group with this slug already exists")
		return nil
	case errors.Is(err, ErrNoMatch):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusUnprocessableEntity, "the selector names no permission this application declares")
		return nil
	case errors.Is(err, ErrTooMany):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusUnprocessableEntity, "the request names more than one write may")
		return nil
	case errors.Is(err, ErrInvalidMember):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusUnprocessableEntity, "somebody was named by an identifier that cannot be one")
		return nil
	case errors.Is(err, ErrBootstrapped):
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusConflict, "this tenant already has a group")
		return nil
	}

	// A rejected input is the answer rather than a failure, and the fields that
	// were rejected are the client's own, so naming them gives nothing away.
	var rejected validation.Errors
	if errors.As(err, &rejected) {
		fhttp.Refuse(ctx.Response, ctx.Request, stdhttp.StatusUnprocessableEntity, rejected.Error())
		return nil
	}
	return err
}

// Migrations declares the schema this module owns.
//
// They are returned in the order their names sort in, which is the order they
// apply in: the name carries the order, and nothing else decides it.
func (m *Module) Migrations() []foundation.Migration {
	return []foundation.Migration{createPermissionTables{}, createPermissionUserActions{}}
}

// The migration is reversible, and the assertion is here rather than discovered
// at rollback: the migrator tests for Down with a type assertion, so a Down with
// the wrong signature would leave a rollback that silently does nothing.
var (
	_ migrations.ReversibleMigration = createPermissionTables{}
	_ migrations.ReversibleMigration = createPermissionUserActions{}
)

// createPermissionTables is the schema this module owns.
//
// Four tables in one migration, because they are one thing: a groups table
// whose links have nowhere to point and a token nothing writes are not a state
// any application should be able to deploy into.
type createPermissionTables struct{ migrations.BaseMigration }

// GetName is the migration's identity, and it carries the order. It is fixed
// once the package is published: changing what an applied name means leaves the
// change missing everywhere it already ran, and nothing says so.
func (createPermissionTables) GetName() string { return "20260905_0001_create_permission_tables" }

// Up creates the four tables, the uniqueness that keeps them consistent, and
// the indexes their reads scan.
//
// The Blueprint spells each column for the engine the migration is running on,
// which is what lets one application develop on a file and deploy on Postgres
// without a second schema.
//
// Every unique index names the tenant first. Without it, two customers cannot
// both have a group called "editors" -- and with the tenant anywhere but first,
// the index that enforces it is not the index the listing scans.
//
// The timestamps have no database default: the value comes from Go.
func (createPermissionTables) Up(ctx context.Context, conn migrations.Connection) error {
	if err := conn.Schema().Create(ctx, GroupsTable, func(table *schema.Blueprint) {
		table.String("id").Primary()
		table.String("tenant_id")
		table.String("slug")
		table.String("name")
		table.Text("description")
		table.Boolean("is_system").Default(false)
		table.Timestamp("created_at")
		table.Timestamp("updated_at")

		// One group per slug per customer. It is the constraint the create path
		// checks for before it writes, and the one that holds when two requests
		// check at the same time.
		table.Unique([]string{"tenant_id", "slug"}, "permission_groups_tenant_slug_uq")
	}); err != nil {
		return err
	}

	if err := conn.Schema().Create(ctx, GroupActionsTable, func(table *schema.Blueprint) {
		table.String("id").Primary()
		table.String("tenant_id")
		table.String("group_id")
		table.String("action")

		// A group carries an action once. Without this, a form submitted twice
		// leaves two rows, and the screen that lists what a group carries shows
		// the same permission twice with no way to tell which one to remove.
		table.Unique([]string{"tenant_id", "group_id", "action"}, "permission_group_actions_uq")
		table.Index([]string{"tenant_id", "group_id"}, "permission_group_actions_group_idx")
	}); err != nil {
		return err
	}

	if err := conn.Schema().Create(ctx, GroupUsersTable, func(table *schema.Blueprint) {
		table.String("id").Primary()
		table.String("tenant_id")
		table.String("group_id")
		table.String("user_id")

		table.Unique([]string{"tenant_id", "group_id", "user_id"}, "permission_group_users_uq")
		// The index the resolution reads by: one person, every group they are
		// in. It is the query on the path of every request, so it is the one
		// index that cannot be missing.
		table.Index([]string{"tenant_id", "user_id"}, "permission_group_users_user_idx")
	}); err != nil {
		return err
	}

	return conn.Schema().Create(ctx, VersionsTable, func(table *schema.Blueprint) {
		// The tenant is the key: one row per customer, so the read is a point
		// read and the write cannot produce a second row for the same one.
		table.String("tenant_id").Primary()
		table.BigInteger("version")
	})
}

// Down drops the four tables, which takes their indexes with them.
func (createPermissionTables) Down(ctx context.Context, conn migrations.Connection) error {
	for _, table := range []string{VersionsTable, GroupUsersTable, GroupActionsTable, GroupsTable} {
		if err := conn.Schema().DropIfExists(ctx, table); err != nil {
			return err
		}
	}
	return nil
}

// createPermissionUserActions is the table for what one person carries in their
// own right.
//
// It is a second migration rather than a column added to the first, because the
// first has already been applied somewhere. A name that has run means something
// there, and editing what it means leaves the change missing on every
// installation that had it, with nothing to say so.
type createPermissionUserActions struct{ migrations.BaseMigration }

// GetName is the migration's identity, and it carries the order.
func (createPermissionUserActions) GetName() string {
	return "20260905_0002_create_permission_user_actions"
}

// Up creates the table, the uniqueness that keeps it consistent, and the index
// its one hot read scans.
func (createPermissionUserActions) Up(ctx context.Context, conn migrations.Connection) error {
	return conn.Schema().Create(ctx, UserActionsTable, func(table *schema.Blueprint) {
		table.String("id").Primary()
		table.String("tenant_id")
		table.String("user_id")
		table.String("action")

		// A person carries an action once. Without this, a form submitted twice
		// leaves two rows, and taking the permission away removes one of them.
		table.Unique([]string{"tenant_id", "user_id", "action"}, "permission_user_actions_uq")
		// The index the resolution reads by. It is on the path of every request
		// that has a session, beside the membership index, so it is the second
		// index that cannot be missing.
		table.Index([]string{"tenant_id", "user_id"}, "permission_user_actions_user_idx")
	})
}

// Down drops the table, which takes its indexes with it.
func (createPermissionUserActions) Down(ctx context.Context, conn migrations.Connection) error {
	return conn.Schema().DropIfExists(ctx, UserActionsTable)
}
