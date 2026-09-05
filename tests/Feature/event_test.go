package feature_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	hdatabase "github.com/arandu-io/hesape/database"

	permission "github.com/hyz-is/arandu-permission"
)

// What an application is told, and when.
//
// A listener is how an installation writes its own audit trail, busts its own
// caches and tells its own systems. The two properties that make one worth
// having are held here: it hears about everything that could change a decision,
// and it hears about it only once the change is durable.

// recorder collects what it is told, in order.
type recorder struct {
	mu     sync.Mutex
	events []permission.Event
}

func (r *recorder) listen(_ context.Context, e permission.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) kinds() []permission.EventKind {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]permission.EventKind, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Kind)
	}
	return out
}

// watched is a service over an empty, migrated database that reports to the
// recorder.
func watched(t *testing.T) (*permission.PermissionService, *recorder) {
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

	catalogue, err := permission.NewCatalogue(settings().Actions...)
	if err != nil {
		t.Fatalf("building the catalogue: %v", err)
	}
	heard := &recorder{}
	return permission.NewPermissionService(handle, catalogue, heard.listen), heard
}

// TestEveryWriteThatCouldChangeADecisionIsAnnounced walks the whole write
// surface once and requires an event from each step.
//
// It is written as one pass rather than one test per kind because the property
// is about coverage: a write path added without an event is a change an
// application's audit log never records, and only a test that walks all of them
// notices.
func TestEveryWriteThatCouldChangeADecisionIsAnnounced(t *testing.T) {
	t.Parallel()

	svc, heard := watched(t)
	ctx := context.Background()
	actor := operator("acme", "invoice.delete")

	group, err := svc.CreateGroup(ctx, actor, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	if _, err := svc.UpdateGroup(ctx, actor, group.ID, permission.UpdateGroupRequest{Name: "Editorial"}); err != nil {
		t.Fatalf("updating: %v", err)
	}
	if _, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}
	if _, err := svc.SetMembers(ctx, actor, group.ID, []string{"kim"}); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	if _, err := svc.SetActions(ctx, actor, group.ID, nil); err != nil {
		t.Fatalf("revoking: %v", err)
	}
	if _, err := svc.SetMembers(ctx, actor, group.ID, nil); err != nil {
		t.Fatalf("unassigning: %v", err)
	}
	if _, err := svc.SetDirectActions(ctx, actor, "kim", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting directly: %v", err)
	}
	if _, err := svc.SetDirectActions(ctx, actor, "kim", nil); err != nil {
		t.Fatalf("revoking directly: %v", err)
	}
	if err := svc.DeleteGroup(ctx, actor, group.ID); err != nil {
		t.Fatalf("deleting: %v", err)
	}

	want := []permission.EventKind{
		permission.GroupCreated,
		permission.GroupUpdated,
		permission.ActionsAttached,
		permission.MembersAttached,
		permission.ActionsDetached,
		permission.MembersDetached,
		permission.ActionsAttached,
		permission.ActionsDetached,
		permission.GroupDeleted,
	}
	got := heard.kinds()
	if len(got) != len(want) {
		t.Fatalf("heard %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %s, want %s", i, got[i], want[i])
		}
	}
}

// TestAnEventSaysWhoDidWhatToWhom holds the payload, which is the whole of what
// a listener has to write a line somebody can read a year later.
func TestAnEventSaysWhoDidWhatToWhom(t *testing.T) {
	t.Parallel()

	svc, heard := watched(t)
	ctx := context.Background()
	actor := operator("acme", "invoice.delete")

	group, err := svc.CreateGroup(ctx, actor, permission.CreateGroupRequest{Slug: "billing", Name: "Billing"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	if _, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}
	if _, err := svc.SetDirectActions(ctx, actor, "kim", []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting directly: %v", err)
	}

	heard.mu.Lock()
	defer heard.mu.Unlock()
	attached, direct := heard.events[1], heard.events[2]

	for _, e := range heard.events {
		if e.Tenant != "acme" {
			t.Errorf("%s carries the tenant %q", e.Kind, e.Tenant)
		}
		if e.ActorID != actor.ID {
			t.Errorf("%s carries the actor %q", e.Kind, e.ActorID)
		}
		if e.At.IsZero() {
			t.Errorf("%s carries no time", e.Kind)
		}
	}

	if attached.GroupSlug != "billing" || attached.UserID != "" {
		t.Errorf("the group grant reads as %+v, want the group named and no person", attached)
	}
	if len(attached.Actions) != 1 || attached.Actions[0] != "invoice.delete" {
		t.Errorf("the group grant names %v", attached.Actions)
	}
	if attached.Version == 0 {
		t.Error("the group grant carries no version, so a listener has nothing to compare")
	}

	if direct.UserID != "kim" || direct.GroupID != "" {
		t.Errorf("the direct grant reads as %+v, want the person named and no group", direct)
	}
	if len(direct.Actions) != 1 || direct.Actions[0] != "invoice.delete" {
		t.Errorf("the direct grant names %v", direct.Actions)
	}
}

// TestNothingIsAnnouncedForAWriteThatChangedNothing keeps a listener from
// hearing about a form somebody submitted twice.
func TestNothingIsAnnouncedForAWriteThatChangedNothing(t *testing.T) {
	t.Parallel()

	svc, heard := watched(t)
	ctx := context.Background()
	actor := operator("acme", "invoice.delete")

	group, err := svc.CreateGroup(ctx, actor, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	if _, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}
	before := len(heard.kinds())

	if _, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting the same thing again: %v", err)
	}
	if got := len(heard.kinds()); got != before {
		t.Errorf("a write that changed nothing announced %d events", got-before)
	}
}

// TestARefusedWriteIsNeverAnnounced is the half that matters most: a listener
// told about a change that did not happen has told somebody something untrue,
// and there is no message that takes it back.
func TestARefusedWriteIsNeverAnnounced(t *testing.T) {
	t.Parallel()

	svc, heard := watched(t)
	ctx := context.Background()

	// This actor administers permissions and does not hold invoice.delete.
	actor := operator("acme")
	group, err := svc.CreateGroup(ctx, actor, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	before := len(heard.kinds())

	if _, err := svc.SetActions(ctx, actor, group.ID, []security.Action{"invoice.delete"}); err == nil {
		t.Fatal("granting an unheld action was allowed")
	}
	if _, err := svc.SetDirectActions(ctx, actor, actor.ID, []security.Action{"invoice.delete"}); err == nil {
		t.Fatal("granting to oneself was allowed")
	}
	if got := len(heard.kinds()); got != before {
		t.Errorf("a refused write announced %d events", got-before)
	}
}
