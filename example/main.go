//go:build example

// Command example wires this package the way an application does, and then
// exercises it.
//
// Run it:
//
//	go run -tags example ./example
//
// It is behind a build tag so that installing this package never compiles it:
// what a program does is not a capability whoever ran `go get` agreed to, and a
// library that carries a main package carries one anyway.
//
// It uses SQLite in a temporary directory and leaves nothing behind, so it needs
// no configuration and no server. What it does not do is serve the panel: the
// screens are published into a project and compiled there, which takes three
// commands and an application to run them in. The wiring below is the same
// either way -- this stops one step short of Routes, and the README carries the
// rest.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	hdatabase "github.com/arandu-io/hesape/database"

	permission "github.com/hyz-is/arandu-permission"

	// The driver. An application picks its own, and this package never does.
	_ "github.com/arandu-io/hesape/database/connectors/sqlite"
)

// The actions this pretend application declares in its own code.
//
// This is the whole of what may ever be granted. There is no screen that adds to
// it and no row that can: a permission exists because some policy in Go reads
// it, and the panel administers that set rather than defining one.
const (
	InvoiceView   security.Action = "invoice.view"
	InvoiceCreate security.Action = "invoice.create"
	InvoiceUpdate security.Action = "invoice.update"
	InvoiceDelete security.Action = "invoice.delete"
	ReportRead    security.Action = "report.read"
)

// tenant is the customer this example writes as. An application reads its own
// from configuration, and never from a request.
const tenant = "acme"

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "example:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "arandu-permission-example")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	pool, err := sql.Open("sqlite", filepath.Join(dir, "permission.db"))
	if err != nil {
		return err
	}
	defer func() { _ = pool.Close() }()

	// ---- the wiring, which is what an application copies -------------------
	//
	// Every collaborator is a parameter. There is no container, no provider and
	// no discovery: what this module touches is written here and read here.
	key := []byte("0123456789abcdef0123456789abcdef")
	module, err := permission.New(permission.Config{
		Tenant: tenant,
		// The application's own actions, spliced with this package's, so that
		// the administration of permissions can itself be administered.
		Actions: append(
			[]security.Action{InvoiceView, InvoiceCreate, InvoiceUpdate, InvoiceDelete, ReportRead},
			permission.Actions()...,
		),
		// Told what changed, once it has changed.
		Listeners: []permission.Listener{audit},
	},
		data.Wrap(pool, data.DialectSQLite),
		security.NewSessionStore(key, time.Hour, false, security.NewMemoryBackend()),
		security.NewCSRF(key, time.Hour),
	)
	if err != nil {
		return err
	}

	// In an application this is `aru migrate`, run as a pipeline step. Never at
	// boot: with N replicas, N migrations race.
	connection := hdatabase.NewConnection(pool, "main", "", map[string]any{"driver": "sqlite"})
	for _, migration := range module.Migrations() {
		if err := migration.Up(ctx, hdatabase.ForMigrations(connection)); err != nil {
			return fmt.Errorf("applying %s: %w", migration.GetName(), err)
		}
	}

	// The use cases, which are the same ones every screen calls.
	svc := module.Service()

	// ---- the first group, the one write with no session behind it ----------
	say("bootstrap", "the first group of a tenant, refused once there is one")
	admins, err := svc.Bootstrap(ctx, tenant, permission.BootstrapRequest{
		Slug:        "administrators",
		Name:        "Administrators",
		Description: "Carries the administration of permissions",
		Actions:     []security.Action{InvoiceDelete},
		Members:     []string{"alice"},
	})
	if err != nil {
		return err
	}
	fmt.Printf("  created %s, a system group with alice in it\n", admins.Slug)

	if _, err := svc.Bootstrap(ctx, tenant, permission.BootstrapRequest{
		Slug: "second", Name: "Second", Members: []string{"bob"},
	}); err != nil {
		fmt.Printf("  a second bootstrap is refused: %v\n", err)
	}

	// ---- from here on, every write is authorized by a real subject ---------
	alice, err := resolve(ctx, svc, "alice")
	if err != nil {
		return err
	}
	fmt.Printf("  alice carries %d permission(s), read out of the rows above\n", len(alice.Roles))

	say("selector", "naming many permissions at once, bounded by the catalogue")
	editors, err := svc.CreateGroup(ctx, alice, permission.CreateGroupRequest{
		Slug: "editors", Name: "Editors", Description: "Everything about an invoice",
	})
	if err != nil {
		return err
	}
	wanted, err := svc.Catalogue().Match("invoice.*")
	if err != nil {
		return err
	}
	fmt.Printf("  invoice.* names %v\n", wanted)

	// The selector expanded to four actions and alice holds one of them. The
	// rule that nobody hands out what they do not hold is asked once per action,
	// so this is refused -- which is why it is here.
	if _, err := svc.SetActions(ctx, alice, editors.ID, wanted); err != nil {
		fmt.Printf("  alice cannot grant all four: %v\n", err)
	}
	if _, err := svc.SetActions(ctx, alice, editors.ID, []security.Action{InvoiceDelete}); err != nil {
		return err
	}
	if _, err := svc.SetMembers(ctx, alice, editors.ID, []string{"bob"}); err != nil {
		return err
	}

	say("direct", "one permission for one person, outside every group")
	if _, err := svc.SetDirectActions(ctx, alice, "carol", []security.Action{InvoiceDelete}); err != nil {
		return err
	}
	if _, err := svc.SetDirectActions(ctx, alice, "alice", []security.Action{InvoiceDelete}); err != nil {
		fmt.Printf("  alice cannot give one to herself: %v\n", err)
	}

	say("effective", "what each person may do, and why")
	for _, who := range []string{"alice", "bob", "carol", "dave"} {
		effective, err := svc.EffectiveFor(ctx, alice, who)
		if err != nil {
			return err
		}
		fmt.Printf("  %-6s %s\n", who, describe(effective))
	}

	say("revocation", "the token is what makes it take effect everywhere")
	before, err := svc.Version(ctx, alice)
	if err != nil {
		return err
	}
	if _, err := svc.SetDirectActions(ctx, alice, "carol", nil); err != nil {
		return err
	}
	after, err := svc.Version(ctx, alice)
	if err != nil {
		return err
	}
	fmt.Printf("  the token moved from %d to %d, so every process re-reads\n", before, after)

	carol, err := resolve(ctx, svc, "carol")
	if err != nil {
		return err
	}
	fmt.Printf("  carol now carries %v\n", carol.Roles)

	say("labels", "the identifier is stored, the sentence is resolved when drawn")
	labels := module.Labels("pt-BR")
	for _, action := range []security.Action{permission.PermissionGrantDirect, InvoiceDelete} {
		fmt.Printf("  %-26s %s\n", action, labels.Action(action))
	}
	return nil
}

// resolve is what the middleware does on every request: read the memberships and
// the direct grants, and put the result on the subject every policy reads.
func resolve(ctx context.Context, svc *permission.PermissionService, id string) (security.Subject, error) {
	subject := security.Subject{ID: id, Tenant: tenant, Verified: true}
	resolved, err := svc.ResolveOwn(ctx, subject)
	if err != nil {
		return security.Subject{}, err
	}
	subject.Roles = resolved.Roles
	return subject, nil
}

// audit is a listener. An application writes a line, busts a cache or tells
// another system; this prints.
func audit(_ context.Context, e permission.Event) {
	target := e.GroupSlug
	if target == "" {
		target = e.UserID
	}
	what := make([]string, 0, len(e.Actions)+len(e.Members))
	for _, action := range e.Actions {
		what = append(what, string(action))
	}
	what = append(what, e.Members...)

	line := fmt.Sprintf("    [event] %s %s by %s", e.Kind, target, e.ActorID)
	if len(what) > 0 {
		line += ": " + strings.Join(what, ", ")
	}
	fmt.Println(line)
}

// describe is one person's permissions and where each came from.
func describe(effective permission.Effective) string {
	if len(effective.Grants) == 0 {
		return "carries nothing"
	}
	parts := make([]string, 0, len(effective.Grants))
	for _, grant := range effective.Grants {
		from := make([]string, 0, len(grant.Groups)+1)
		if grant.Direct {
			from = append(from, "direct")
		}
		for _, group := range grant.Groups {
			from = append(from, group.Slug)
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", grant.Action, strings.Join(from, "+")))
	}
	return strings.Join(parts, "  ")
}

// say prints a heading.
func say(name, lead string) { fmt.Printf("\n== %s: %s\n", name, lead) }
