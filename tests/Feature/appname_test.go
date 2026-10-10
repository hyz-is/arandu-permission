package feature_test

import (
	"context"
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/arandu-io/framework/foundation"
	"github.com/arandu-io/framework/foundation/bootstrap"
	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/http/middleware"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/config"

	permission "github.com/hyz-is/arandu-permission"
)

// These tests serve the screens through the Application an installer boots,
// rather than through a bare router, because the brand is not something this
// module knows: it is the application's APP_NAME, which the Application puts
// on every request context and view.New reads from there. A screen that drew
// a brand of its own, or none, would show here as a page whose navigation bar
// names something other than the application it is part of.

// shell is the part of an application the screens lean on and this module does
// not bring: the renderer the layout is drawn with, and the routes the layout
// links to, under the names the application skeleton gives them.
type shell struct{}

func (shell) Name() string { return "shell" }

func (shell) Routes(r *fhttp.Router) {
	nothing := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	r.Get("/{$}", nothing).Name("home")
	r.Get("/auth/login", nothing).Name("auth.login")
	r.Post("/auth/logout", nothing).Name("auth.logout")
}

func (shell) Renderer() fhttp.Renderer { return layout{} }

// booted is the application an installer runs, named name, with this module
// and the shell registered and the form protection mounted, booted and ready
// to answer.
//
// The module boots because the views are linked: this binary registers their
// names from init(), the way the compiled views of an application do.
func booted(t *testing.T, name string) application {
	t.Helper()

	handle, svc, admin := administered(t, administrator)
	sessions := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	module, err := permission.New(settings(), handle, sessions, nil)
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	cfg := bootstrap.Configuration{
		App:           config.App{Name: name, Env: config.EnvProd, Key: []byte(appKey)},
		Observability: bootstrap.Observability{LogLevel: slog.LevelError},
	}
	csrf := security.NewCSRF([]byte(appKey), time.Hour).Secure(false)
	app := foundation.New(cfg).
		Register(shell{}, module).
		Use(middleware.CSRFProtect(csrf, sessions.IDFromRequest))
	if err := app.Boot(context.Background()); err != nil {
		t.Fatalf("booting the application: %v", err)
	}
	return application{handler: app.Handler(), sessions: sessions, svc: svc, admin: admin}
}

// brand is the text of the brand link in a drawn screen, and whether the
// screen drew that link at all.
func brand(body string) (string, bool) {
	m := regexp.MustCompile(`<a data-brand href="[^"]*">([^<]*)</a>`).FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return html.UnescapeString(m[1]), true
}

func TestEveryScreenDrawsTheApplicationNameTheRequestCarries(t *testing.T) {
	const name = "Acme Console"

	app := booted(t, name)
	session := app.signIn(t, administrator)
	for _, target := range []string{
		permission.DefaultPrefix + "/groups",
		permission.DefaultPrefix + "/catalogue",
		permission.DefaultPrefix + "/matrix",
		permission.DefaultPrefix + "/users",
		permission.DefaultPrefix + "/users/" + administrator,
	} {
		rec := app.visit(t, http.MethodGet, target, nil, session, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s answered %d: %s", target, rec.Code, rec.Body)
		}
		got, drawn := brand(rec.Body.String())
		if !drawn {
			t.Fatalf("%s drew no brand link:\n%s", target, rec.Body)
		}
		if got != name {
			t.Errorf("%s draws the brand %q, want the application's APP_NAME, %q", target, got, name)
		}
		if got, _ := href(rec.Body.String(), "data-brand"); got != "/" {
			t.Errorf("%s links the brand to %q, want the route named home, /", target, got)
		}
	}
}

// TestAnApplicationWithNoNameGetsNoBrandMadeUp holds the other half: the module
// has no name of its own to fall back to, so an application whose configuration
// carries none draws none, rather than the module's.
func TestAnApplicationWithNoNameGetsNoBrandMadeUp(t *testing.T) {
	app := booted(t, "")
	rec := app.visit(t, http.MethodGet, permission.DefaultPrefix+"/groups", nil, app.signIn(t, administrator), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("the listing answered %d: %s", rec.Code, rec.Body)
	}
	if got, drawn := brand(rec.Body.String()); !drawn || got != "" {
		t.Errorf("with no APP_NAME the brand reads %q (drawn: %t), want an empty brand", got, drawn)
	}
}
