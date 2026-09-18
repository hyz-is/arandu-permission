package permission

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	hdatabase "github.com/arandu-io/hesape/database"
	_ "github.com/arandu-io/hesape/database/connectors/sqlite"
)

func TestMemberSearchNarrowsBothPermissionSourcesBeforeUnion(t *testing.T) {
	pool, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "members-search.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	handle := data.Wrap(pool, data.DialectSQLite)
	sessions := security.NewSessionStore([]byte("0123456789abcdef0123456789abcdef"), time.Hour, false, security.NewMemoryBackend())
	csrf := security.NewCSRF([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	mod, err := New(Config{Tenant: "acme", Actions: append(Actions(), "invoice.delete")}, handle, sessions, csrf)
	if err != nil {
		t.Fatal(err)
	}
	conn := hdatabase.NewConnection(pool, "main", "", map[string]any{"driver": "sqlite"})
	for _, migration := range mod.Migrations() {
		if err := migration.Up(context.Background(), hdatabase.ForMigrations(conn)); err != nil {
			t.Fatalf("applying %s: %v", migration.GetName(), err)
		}
	}

	ctx := context.Background()
	actor := security.Subject{ID: "seed", Tenant: "acme", Verified: true, Actions: append(Actions(), "invoice.delete")}
	group, err := mod.svc.CreateGroup(ctx, actor, CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mod.svc.SetMembers(ctx, actor, group.ID, []string{"alpha", "bravo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := mod.svc.SetDirectActions(ctx, actor, "charlie", []security.Action{"invoice.delete"}); err != nil {
		t.Fatal(err)
	}

	page, err := mod.svc.searchMembers(ctx, actor, MemberQuery{Limit: 50}, "br")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].UserID != "bravo" {
		t.Fatalf("search returned %+v, want only bravo", page.Items)
	}
	page, err = mod.svc.searchMembers(ctx, actor, MemberQuery{Limit: 50}, "char")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].UserID != "charlie" || page.Items[0].Direct != 1 {
		t.Fatalf("direct-grant search returned %+v, want charlie with one direct action", page.Items)
	}
}
