package feature_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/auth"
	"github.com/arandu-io/hesape/log"

	permission "github.com/hyz-is/arandu-permission"
)

// An application mounts Roles for every route and again behind a route group.
// The second mount on one request must not read the tenant's token again, and
// the next request must still read it, so a revocation holds on the next one.
func TestASecondMountOfRolesAnswersFromTheSameRequest(t *testing.T) {
	t.Parallel()

	handle, svc, _ := administered(t, "user-1")
	admin := operator("acme", "invoice.delete")
	editors := seed(t, svc, admin, permission.CreateGroupRequest{Slug: "editors", Name: "Editors"})
	if _, err := svc.SetActions(context.Background(), admin, editors.ID, []security.Action{"invoice.delete"}); err != nil {
		t.Fatalf("granting: %v", err)
	}
	if _, err := svc.SetMembers(context.Background(), admin, editors.ID, []string{"user-9"}); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	sessions := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	module, err := permission.New(settings(), handle, sessions, nil)
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	carried := map[string][]security.Action{}
	probe := func(name string) fhttp.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				subject, _ := auth.SubjectFrom(r.Context())
				carried[name] = subject.Actions
				next.ServeHTTP(w, r)
			})
		}
	}
	router := fhttp.NewRouter()
	once := router.Group("", module.Roles(), probe("once"))
	once.Get("/once", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	twice := router.Group("", module.Roles(), module.Roles(), probe("twice"))
	twice.Get("/twice", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	rec := httptest.NewRecorder()
	if _, err := sessions.Start(context.Background(), rec, security.Subject{ID: "user-9", Tenant: "acme", Verified: true}); err != nil {
		t.Fatalf("signing in: %v", err)
	}
	cookies := rec.Result().Cookies()

	queries := func(target string) int {
		collector := log.NewCollector("roles")
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req = req.WithContext(log.WithCollector(req.Context(), collector))
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		answer := httptest.NewRecorder()
		router.ServeHTTP(answer, req)
		if answer.Code != http.StatusNoContent {
			t.Fatalf("%s answered %d", target, answer.Code)
		}
		return collector.QueryCount()
	}

	// The first request of a process fills the resolver's memory; the ones
	// compared are made once it is warm.
	queries("/once")
	queries("/twice")
	single, double := queries("/once"), queries("/twice")
	if single == 0 {
		t.Fatal("resolving a person read nothing, so this counts nothing")
	}
	if double != single {
		t.Errorf("Roles mounted twice read %d times, mounted once %d", double, single)
	}
	if len(carried["twice"]) == 0 || len(carried["twice"]) != len(carried["once"]) {
		t.Errorf("the second mount carried %v, the first %v", carried["twice"], carried["once"])
	}

	// Taking the action away holds on the next request.
	if _, err := svc.SetActions(context.Background(), admin, editors.ID, nil); err != nil {
		t.Fatalf("revoking: %v", err)
	}
	queries("/twice")
	if len(carried["twice"]) != 0 {
		t.Errorf("a revoked action was still carried on the next request: %v", carried["twice"])
	}
}
