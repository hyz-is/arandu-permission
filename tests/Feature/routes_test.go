package feature_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/arandu-io/framework/data"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/database/migrations"

	permission "github.com/hyz-is/arandu-permission"
)

// These tests drive the module the way an application does: build it, register
// its routes on a router, and make a request.
//
// The database handle wraps nothing, and that is the assertion. A request that
// reached a statement would panic, so every answer below is proof that the
// refusal happened in a policy and not after a read. The tests that need rows
// are in protections_test.go, over a real database.

// appKey is the key a session store is built over. Any thirty-two bytes will do
// here; a real application reads its own from the environment.
const appKey = "0123456789abcdef0123456789abcdef"

// reservedPrefix is the namespace the framework keeps for itself: the health
// probe, the reload endpoint, the development console, the addressed assets.
//
// A module that registers under it is refused when the application boots, by
// name -- and that refusal happens in the process of whoever installed this
// package, after it was published. Here the same rule is a failing test, in the
// repository that can still fix it.
const reservedPrefix = "/_arandu"

// mount builds the module and returns a router with its routes registered.
func mount(t *testing.T, cfg permission.Config) *fhttp.Router {
	t.Helper()

	module, err := build(t, cfg, data.Wrap(nil, data.DialectSQLite))
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	router := fhttp.NewRouter()
	module.Routes(router.ForModule(module.Name()))
	return router
}

// build wires the module over a handle, with the collaborators an application
// would hand it.
func build(t *testing.T, cfg permission.Config, db *data.DB) (*permission.Module, error) {
	t.Helper()

	sessions := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	csrf := security.NewCSRF([]byte(appKey), time.Hour)
	return permission.New(cfg, db, sessions, csrf)
}

// settings is a configuration that works, with a catalogue holding this
// package's actions and one an application declares.
func settings() permission.Config {
	return permission.Config{
		Tenant:  "acme",
		Actions: append(permission.Actions(), "invoice.delete"),
	}
}

// answer makes one request against the router and returns the recorder.
func answer(t *testing.T, router *fhttp.Router, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// everyRequest is one request per registered address, with a body where the
// address takes one.
func everyRequest(prefix string) []struct{ method, target, body string } {
	return []struct{ method, target, body string }{
		{http.MethodGet, prefix + "/groups", ""},
		{http.MethodPost, prefix + "/groups", "slug=editors&name=Editors"},
		{http.MethodGet, prefix + "/groups/group-1", ""},
		{http.MethodPut, prefix + "/groups/group-1", "name=Editors"},
		{http.MethodDelete, prefix + "/groups/group-1", ""},
		{http.MethodPost, prefix + "/groups/group-1/summary", "kind=actions&value=invoice.delete"},
		{http.MethodPut, prefix + "/groups/group-1/actions", "kind=actions&value=invoice.delete"},
		{http.MethodPut, prefix + "/groups/group-1/members", "kind=members&value=user-9"},
		{http.MethodGet, prefix + "/catalogue", ""},
		{http.MethodGet, prefix + "/matrix", ""},
		{http.MethodGet, prefix + "/users/user-9", ""},
		{http.MethodPost, prefix + "/users/user-9/summary", "value=invoice.delete"},
		{http.MethodPut, prefix + "/users/user-9/actions", "value=invoice.delete"},
	}
}

func TestAVisitorWithNoSessionReachesNothing(t *testing.T) {
	t.Parallel()

	router := mount(t, settings())

	for _, request := range everyRequest(permission.DefaultPrefix) {
		rec := answer(t, router, request.method, request.target, request.body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s answered %d, want %d", request.method, request.target, rec.Code, http.StatusForbidden)
		}
	}
}

// TestEveryRegisteredAddressIsOneTheConfigurationParsed keeps the two lists of
// this module's addresses from drifting.
//
// The configuration check parses the patterns before anything is mounted, so
// that a prefix which cannot be registered is refused where it is wired. It
// reads its own list, and a route registered outside that list is a route the
// check never saw.
func TestEveryRegisteredAddressIsOneTheConfigurationParsed(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{permission.DefaultPrefix, "/admin/access"} {
		cfg := settings()
		cfg.Prefix = prefix
		router := mount(t, cfg)

		got := make([]string, 0)
		for _, route := range router.Routes() {
			got = append(got, route.Method+" "+route.Pattern)
		}
		want := make([]string, 0)
		for _, request := range everyRequest(prefix) {
			want = append(want, request.method+" "+strings.Replace(
				strings.Replace(request.target, "/group-1", "/{group}", 1), "/user-9", "/{user}", 1))
		}
		sort.Strings(got)
		sort.Strings(want)

		if len(got) != len(want) {
			t.Fatalf("with prefix %s the module registered %v, want %v", prefix, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("with prefix %s the module registered %v, want %v", prefix, got, want)
			}
		}
	}
}

// TestEveryRouteDeclaresTheActionItRequires holds the half of the catalogue the
// router answers.
//
// The declaration decides nothing -- the policy is still what refuses -- and
// that is exactly why it has to be checked here: a route that carries none is
// as open as one that carries the wrong one, and nothing at run time would say
// so.
func TestEveryRouteDeclaresTheActionItRequires(t *testing.T) {
	t.Parallel()

	cfg := settings()
	catalogue := map[security.Action]bool{}
	for _, action := range cfg.Actions {
		catalogue[action] = true
	}

	router := mount(t, cfg)
	checked := 0
	for _, route := range router.Routes() {
		action := route.RequiredAction()
		if action == "" {
			t.Errorf("%s %s declares no required action, so a permissions screen cannot list it",
				route.Method, route.Pattern)
			continue
		}
		if !catalogue[action] {
			t.Errorf("%s %s requires %s, which is not in the catalogue this module was built with",
				route.Method, route.Pattern, action)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no route was checked, so this test proved nothing")
	}
}

func TestTheModuleTagsItsRoutesWithItsName(t *testing.T) {
	t.Parallel()

	router := mount(t, settings())
	for _, route := range router.Routes() {
		if route.Module != "permission" {
			t.Errorf("the route %s %s is not tagged with the module name: %q", route.Method, route.Pattern, route.Module)
		}
	}
}

// TestNoRouteLandsInTheFrameworkNamespace is the one property of this package
// that syntax cannot hold: a prefix arrives through configuration, so the only
// way to know where the routes ended up is to register them and read the table
// back.
func TestNoRouteLandsInTheFrameworkNamespace(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{"", "/admin/access"} {
		cfg := settings()
		cfg.Prefix = prefix
		router := mount(t, cfg)

		registered := 0
		for _, route := range router.Routes() {
			registered++
			if route.Pattern == reservedPrefix || strings.HasPrefix(route.Pattern, reservedPrefix+"/") {
				t.Errorf("the route %s %s is registered under %s/, which the framework keeps for itself and refuses at boot",
					route.Method, route.Pattern, reservedPrefix)
			}
		}
		if registered == 0 {
			t.Fatal("the module registered no route, so this test proved nothing")
		}
	}
}

func TestNewRefusesAWiringThatCannotWork(t *testing.T) {
	t.Parallel()

	sessions := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	csrf := security.NewCSRF([]byte(appKey), time.Hour)
	handle := data.Wrap(nil, data.DialectSQLite)
	valid := settings()

	if _, err := permission.New(permission.Config{}, handle, sessions, csrf); err == nil {
		t.Error("a configuration with no tenant was accepted")
	}
	if _, err := permission.New(valid, nil, sessions, csrf); err == nil {
		t.Error("a nil database handle was accepted")
	}
	if _, err := permission.New(valid, handle, nil, csrf); err == nil {
		t.Error("a nil session store was accepted")
	}
	if _, err := permission.New(valid, handle, sessions, nil); err == nil {
		t.Error("a nil CSRF issuer was accepted")
	}
	if _, err := permission.New(valid, handle, sessions, csrf); err != nil {
		t.Fatalf("a valid wiring was refused: %v", err)
	}
}

func TestNewRefusesARoutePrefixThatCannotBeRegistered(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{"/access{", "/access/{group}"} {
		cfg := settings()
		cfg.Prefix = prefix
		if _, err := build(t, cfg, data.Wrap(nil, data.DialectSQLite)); err == nil {
			t.Errorf("New accepted route prefix %q, which would panic during route registration", prefix)
		}
	}
}

func TestTheModuleDeclaresItsSchema(t *testing.T) {
	t.Parallel()

	module, err := build(t, settings(), data.Wrap(nil, data.DialectSQLite))
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	declared := module.Migrations()
	if len(declared) == 0 {
		t.Fatal("the module declares migrations = true and returns none")
	}

	names := make([]string, 0, len(declared))
	for _, migration := range declared {
		name := migration.GetName()
		if name == "" {
			t.Fatal("a migration has no name, and the name is what carries the order")
		}
		names = append(names, name)

		// A migration that cannot be rolled back is a deploy that cannot be
		// undone. The migrator finds Down by type assertion, so a Down with the
		// wrong signature is a rollback that silently does nothing.
		if _, ok := migration.(migrations.ReversibleMigration); !ok {
			t.Errorf("the migration %s has no Down", name)
		}
	}

	if !sort.StringsAreSorted(names) {
		t.Fatalf("the migrations are not returned in the order their names sort in: %v", names)
	}
}
