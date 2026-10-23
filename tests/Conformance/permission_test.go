package conformance_test

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/framework/data"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/http/middleware"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/framework/test"
	"github.com/arandu-io/hesape/auth"
	hdatabase "github.com/arandu-io/hesape/database"
	hhttp "github.com/arandu-io/hesape/http"
	"github.com/arandu-io/hesape/view"

	permission "github.com/hyz-is/arandu-permission"
	"github.com/hyz-is/arandu-permission/conformance"

	// The drivers Serve opens the test's database with: a temporary SQLite
	// file, or a copy of a Postgres template when ARANDU_TEST_DATABASE_URL
	// names a server.
	_ "github.com/arandu-io/hesape/database/connectors/pgx"
	_ "github.com/arandu-io/hesape/database/connectors/sqlite"
)

// The application below is the smallest one composed the way the README
// teaches: a session store whose resolver reads the accounts table, the
// permission module over the same database, Roles mounted for every route
// behind the CSRF middleware, and one route of the application's own behind
// Require. It is what the kit is run against here, on SQLite always and on
// Postgres when a server is named -- the same call an application makes from
// its tests/Conformance with its own boot.

// TestPermissionOnSQLite runs the kit on a temporary SQLite file, whatever the
// environment names.
func TestPermissionOnSQLite(t *testing.T) {
	conformance.Permissions(t, func(t *testing.T) conformance.Fixture { return fixture(t, test.SQLite()) })
}

// TestPermission runs the kit on the database ARANDU_TEST_DATABASE_URL names.
// With none it is skipped, and in CI it fails instead.
func TestPermission(t *testing.T) {
	conformance.Permissions(t, func(t *testing.T) conformance.Fixture { return fixture(t) })
}

// appKey is the key the session store and the CSRF issuer are built over.
const appKey = "0123456789abcdef0123456789abcdef"

// reportView is the application's own action, and reports.index the route
// that requires it.
const reportView security.Action = "report.view"

// tenants are the two organizations the application serves.
var tenants = [2]string{"acme", "globex"}

// settings is the configuration the application hands permission.New.
func settings() permission.Config {
	return permission.Config{
		Tenant:  tenants[0],
		Actions: append([]security.Action{reportView}, permission.Actions()...),
	}
}

// migrate is the application's schema: its accounts table, one row per
// person and organization they belong to, and the module's migrations.
func migrate(ctx context.Context, env test.Env) error {
	if _, err := env.DB.ExecContext(ctx, `CREATE TABLE accounts (
		id VARCHAR(64) NOT NULL,
		tenant VARCHAR(64) NOT NULL,
		PRIMARY KEY (id, tenant)
	)`); err != nil {
		return fmt.Errorf("creating accounts: %w", err)
	}

	module, err := permission.New(settings(), env.DB, sessions(env.DB), nil)
	if err != nil {
		return err
	}
	connection, err := hdatabase.NewSingleConnectionResolver(env.DB, "").Connection("")
	if err != nil {
		return err
	}
	concrete, ok := connection.(*hdatabase.Connection)
	if !ok {
		return fmt.Errorf("the resolver answered %T, not a connection a migration runs on", connection)
	}
	for _, migration := range module.Migrations() {
		if err := migration.Up(ctx, hdatabase.ForMigrations(concrete)); err != nil {
			return fmt.Errorf("applying %s: %w", migration.GetName(), err)
		}
	}
	return nil
}

// sessions is the application's session store, with the resolver that answers
// who an account is now from the accounts table.
func sessions(db *data.DB) *security.SessionStore {
	store := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	store.ResolveSubjects(func(ctx context.Context, who auth.Subject) (auth.Subject, error) {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM accounts WHERE id = ? AND tenant = ?`, who.ID, who.Tenant).Scan(&count); err != nil {
			return auth.Subject{}, err
		}
		if count == 0 {
			return auth.Subject{}, hhttp.ErrSubjectGone
		}
		return auth.Subject{ID: who.ID, Tenant: who.Tenant, Verified: true}, nil
	})
	return store
}

// fixture serves the application and hands the kit what it needs of it.
func fixture(t *testing.T, opts ...test.Option) conformance.Fixture {
	t.Helper()

	var module *permission.Module
	var handler http.Handler
	boot := func(_ context.Context, env test.Env) (test.Application, error) {
		store := sessions(env.DB)
		built, err := permission.New(settings(), env.DB, store, nil)
		if err != nil {
			return test.Application{}, err
		}

		router := fhttp.NewRouter().WithRenderer(layout{})
		built.Routes(router.ForModule(built.Name()))
		router.Group("", built.Require(reportView)).
			Get("/reports", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }).
			Name("reports.index")

		csrf := security.NewCSRF([]byte(appKey), time.Hour).Secure(false)
		module = built
		handler = middleware.CSRFProtect(csrf, store.IDFromRequest)(built.Roles()(router))
		return test.Application{Handler: handler, Sessions: store, Routes: router.Routes()}, nil
	}

	server := test.Serve(t, boot, append([]test.Option{test.Migrate(migrate)}, opts...)...)
	return conformance.Fixture{
		Server:  server,
		Handler: handler,
		Module:  module,
		Tenants: tenants,
		Account: func(t *testing.T, tenant string) auth.Subject {
			t.Helper()
			id, err := data.NewID()
			if err != nil {
				t.Fatalf("making an account id: %v", err)
			}
			join(t, server.DB(), id, tenant)
			return auth.Subject{ID: id, Tenant: tenant}
		},
		Join: func(t *testing.T, account auth.Subject, tenant string) auth.Subject {
			t.Helper()
			join(t, server.DB(), account.ID, tenant)
			return auth.Subject{ID: account.ID, Tenant: tenant}
		},
		Guarded: conformance.Guarded{Route: "reports.index", Action: reportView},
	}
}

// join makes the account a member of the organization.
func join(t *testing.T, db *data.DB, id, tenant string) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), `INSERT INTO accounts (id, tenant) VALUES (?, ?)`, id, tenant); err != nil {
		t.Fatalf("adding %s to %s: %v", id, tenant, err)
	}
}

// layout is the application's layout, reduced to what a form needs from it:
// the hidden _token field @csrf writes.
type layout struct{}

func (layout) Render(_ context.Context, w http.ResponseWriter, status int, name string, data any) error {
	page, ok := data.(view.Layout)
	if !ok {
		return errors.New(name + " was handed a value the layout cannot draw")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	var b strings.Builder
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(page.PageTitle()))
	fmt.Fprintf(&b, "<form method=\"post\"><input type=\"hidden\" name=\"_token\" value=\"%s\"></form>\n", html.EscapeString(page.CSRFToken()))
	_, err := w.Write([]byte(b.String()))
	return err
}
