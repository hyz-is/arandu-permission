package feature_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/arandu-io/framework/data"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/auth"

	permission "github.com/hyz-is/arandu-permission"
)

// What the route guard is, and what it is careful not to be.
//
// The reference ships three of these and they are the first thing anybody
// installing it writes into a route file. Here there is one, and it can only
// refuse: nothing it lets through has been authorized by it, because the policy
// behind the handler is still what decides.

// guarded mounts one route behind Require and reports what it answered, given a
// subject already on the request.
func guarded(t *testing.T, carried *security.Subject, wanted ...security.Action) int {
	t.Helper()

	module, err := build(t, settings(), data.Wrap(nil, data.DialectSQLite))
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	router := fhttp.NewRouter()
	reached := false
	router.Group("", module.Require(wanted...)).
		Get("/reports", func(w http.ResponseWriter, _ *http.Request) {
			reached = true
			w.WriteHeader(http.StatusNoContent)
		})

	req := httptest.NewRequest(http.MethodGet, "/reports", nil)
	if carried != nil {
		req = req.WithContext(auth.WithSubject(req.Context(), *carried))
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code >= 400 && reached {
		t.Error("the handler ran and the request was refused")
	}
	return rec.Code
}

func TestRequireRefusesAVisitorWithNoSubject(t *testing.T) {
	t.Parallel()

	if got := guarded(t, nil, permission.PermissionList); got != http.StatusUnauthorized {
		t.Errorf("a request with no subject = %d, want %d", got, http.StatusUnauthorized)
	}
}

func TestRequireRefusesASubjectCarryingSomethingElse(t *testing.T) {
	t.Parallel()

	subject := security.Subject{ID: "kim", Tenant: "acme", Roles: []string{"invoice.delete"}}
	if got := guarded(t, &subject, permission.PermissionList); got != http.StatusForbidden {
		t.Errorf("a subject carrying something else = %d, want %d", got, http.StatusForbidden)
	}
}

// TestRequireAdmitsASubjectCarryingAnyOfThem holds the alternative, which is
// what a screen reachable by two different roles needs.
func TestRequireAdmitsASubjectCarryingAnyOfThem(t *testing.T) {
	t.Parallel()

	subject := security.Subject{ID: "kim", Tenant: "acme", Roles: []string{"invoice.delete"}}
	got := guarded(t, &subject, permission.PermissionList, "invoice.delete")
	if got != http.StatusNoContent {
		t.Errorf("a subject carrying the second of two = %d, want %d", got, http.StatusNoContent)
	}
}

// TestRequireWithNoActionRefusesEverybody keeps a guard that names nothing from
// reading as a guard that wants nothing.
func TestRequireWithNoActionRefusesEverybody(t *testing.T) {
	t.Parallel()

	subject := security.Subject{ID: "kim", Tenant: "acme", Roles: []string{"invoice.delete"}}
	if got := guarded(t, &subject); got != http.StatusForbidden {
		t.Errorf("a guard naming no action = %d, want %d", got, http.StatusForbidden)
	}
}

// TestRequireBeforeTheResolverRefusesRatherThanAdmits is the failure direction.
//
// The actions come from the middleware that fills them in. Mounted the wrong way
// round, this sees a subject carrying nothing -- and a guard that fell open in
// that case would be a guard whose protection depends on wiring order nobody
// checks.
func TestRequireBeforeTheResolverRefusesRatherThanAdmits(t *testing.T) {
	t.Parallel()

	unresolved := security.Subject{ID: "kim", Tenant: "acme"}
	if got := guarded(t, &unresolved, permission.PermissionList); got != http.StatusForbidden {
		t.Errorf("a subject whose actions were never filled in = %d, want %d", got, http.StatusForbidden)
	}
}
