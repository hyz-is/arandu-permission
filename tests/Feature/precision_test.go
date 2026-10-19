package feature_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arandu-io/framework/data"
	hdatabase "github.com/arandu-io/hesape/database"
	pgxconnector "github.com/arandu-io/hesape/database/connectors/pgx"
	"github.com/arandu-io/hesape/database/migrations"

	permission "github.com/hyz-is/arandu-permission"
)

// A database migrated before the framework's default time precision was
// microseconds holds the groups' instants in whole seconds, and the last
// migration widens them in place.

func TestTheLastMigrationWidensTheGroups(t *testing.T) {
	t.Parallel()

	pool, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "permission.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	module, err := build(t, settings(), data.Wrap(pool, data.DialectSQLite))
	if err != nil {
		t.Fatal(err)
	}
	all := module.Migrations()
	widen, ok := all[len(all)-1].(migrations.WidenTimePrecision)
	if !ok || widen.GetName() != "20261019_0003_widen_permission_time_precision" ||
		len(widen.Tables) != 1 || widen.Tables[0] != permission.GroupsTable {
		t.Fatalf("the last migration is %s, want the widening of the groups table", all[len(all)-1].GetName())
	}
	// On SQLite there is nothing to widen, and every migration applies.
	conn := hdatabase.ForMigrations(hdatabase.NewConnection(pool, "main", "", map[string]any{"driver": "sqlite"}))
	for _, migration := range all {
		if err := migration.Up(context.Background(), conn); err != nil {
			t.Fatalf("applying %s: %v", migration.GetName(), err)
		}
	}
}

func TestWideningKeepsTheGroupsOnPostgres(t *testing.T) {
	t.Parallel()

	dsn := os.Getenv("ARANDU_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("no server: set ARANDU_TEST_POSTGRES_DSN")
	}
	ctx := context.Background()
	suffix := make([]byte, 8)
	_, _ = rand.Read(suffix)
	schema := "permission_" + hex.EncodeToString(suffix)

	admin, err := sql.Open(pgxconnector.PostgresConnector{}.DriverName(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
		_ = admin.Close()
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pool, err := sql.Open(pgxconnector.PostgresConnector{}.DriverName(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	module, err := build(t, settings(), data.Wrap(pool, data.DialectPostgres))
	if err != nil {
		t.Fatal(err)
	}
	conn := hdatabase.ForMigrations(hdatabase.NewConnection(pool, schema, "", map[string]any{"driver": "pgsql"}))
	all := module.Migrations()
	for _, migration := range all[:len(all)-1] {
		if err := migration.Up(ctx, conn); err != nil {
			t.Fatalf("applying %s: %v", migration.GetName(), err)
		}
	}
	// The shape a release before the widening left on a database migrated
	// while the default was whole seconds.
	for _, column := range []string{"created_at", "updated_at"} {
		if _, err := pool.ExecContext(ctx, `ALTER TABLE `+permission.GroupsTable+` ALTER COLUMN `+column+` TYPE timestamp(0) without time zone`); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	if _, err := pool.ExecContext(ctx, `INSERT INTO `+permission.GroupsTable+`
		(id, tenant_id, slug, name, description, is_system, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		"group-1", "acme", "editors", "Editors", "", false, at, at); err != nil {
		t.Fatal(err)
	}

	if err := all[len(all)-1].Up(ctx, conn); err != nil {
		t.Fatalf("widening: %v", err)
	}
	var narrow int
	if err := pool.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND data_type LIKE 'timestamp%' AND datetime_precision < 6`).Scan(&narrow); err != nil {
		t.Fatal(err)
	}
	if narrow != 0 {
		t.Fatalf("%d date-time columns are still in seconds after the widening", narrow)
	}
	var created time.Time
	if err := pool.QueryRowContext(ctx, `SELECT created_at FROM `+permission.GroupsTable+` WHERE id = $1`, "group-1").Scan(&created); err != nil {
		t.Fatal(err)
	}
	if !created.UTC().Equal(at) {
		t.Fatalf("the group was created at %s after the widening, want %s", created.UTC(), at)
	}
	if reversible, ok := all[len(all)-1].(migrations.ReversibleMigration); !ok {
		t.Fatal("the widening cannot be rolled back")
	} else if err := reversible.Down(ctx, conn); err != nil {
		t.Fatalf("rolling the widening back: %v", err)
	}
}
