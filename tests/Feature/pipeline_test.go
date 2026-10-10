package feature_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/http/middleware"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/view"

	permission "github.com/hyz-is/arandu-permission"
)

// These tests serve the screens the way an application does: behind the
// middleware that protects forms, over a migrated database where one person is
// in the administering group. What they hold is the round trip that person
// makes -- load a screen, submit the form it drew -- and the requests that must
// still be turned away.
//
// The renderer below stands in for the application's layout, and it draws what
// that layout draws from the page: the brand link, the sign-in link while nobody
// is signed in and the sign-out link once somebody is, and the hidden _token
// field @csrf writes. It reads them through view.Layout, the interface the
// layout itself is compiled against, so a field the layout would show empty is
// shown empty here.

// layout is the application's layout, reduced to the parts these tests read.
type layout struct{}

func (layout) Render(_ context.Context, w http.ResponseWriter, status int, name string, data any) error {
	page, ok := data.(view.Layout)
	if !ok {
		return fmt.Errorf("%s was handed %T, which the layout cannot draw", name, data)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	var b strings.Builder
	fmt.Fprintf(&b, "<a data-brand href=\"%s\">%s</a>\n", html.EscapeString(page.HomeLink()), html.EscapeString(page.BrandName()))
	if page.SignedIn() {
		fmt.Fprintf(&b, "<a data-logout href=\"%s\">Sign out</a>\n", html.EscapeString(page.LogoutLink()))
	} else {
		fmt.Fprintf(&b, "<a data-login href=\"%s\">Sign in</a>\n", html.EscapeString(page.LoginLink()))
	}
	fmt.Fprintf(&b, "<form method=\"post\"><input type=\"hidden\" name=\"_token\" value=\"%s\"></form>\n", html.EscapeString(page.CSRFToken()))
	_, err := w.Write([]byte(b.String()))
	return err
}

// application is one served instance: the handler the server would run, the
// session store a sign-in writes to, and the service and subject that count
// what the forms wrote.
type application struct {
	handler  http.Handler
	sessions *security.SessionStore
	svc      *permission.PermissionService
	admin    security.Subject
}

// administrator is the person the seed put into the administering group.
const administrator = "user-1"

// serve builds the application. withNavigation registers the three routes the
// layout links to, under the names the application skeleton gives them; without
// it the application has none, which is the case where no link is the right
// answer.
//
// issuer is what the wiring hands New. The skeleton passes nil; a wiring
// written before the parameter stopped being read passes its own, and it must
// make no difference to the token the screens draw.
func serve(t *testing.T, withNavigation bool, issuer *security.CSRF) application {
	t.Helper()

	handle, svc, admin := administered(t, administrator)

	sessions := security.NewSessionStore([]byte(appKey), time.Hour, false, security.NewMemoryBackend())
	module, err := permission.New(settings(), handle, sessions, issuer)
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	router := fhttp.NewRouter().WithRenderer(layout{})
	if withNavigation {
		nothing := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
		router.Get("/{$}", nothing).Name("home")
		router.Get("/auth/login", nothing).Name("auth.login")
		router.Post("/auth/logout", nothing).Name("auth.logout")
	}
	module.Routes(router.ForModule(module.Name()))

	// The middleware the application skeleton mounts, built the way it builds
	// it: over the session store's own reader of the session cookie.
	csrf := security.NewCSRF([]byte(appKey), time.Hour).Secure(false)
	return application{
		handler:  middleware.CSRFProtect(csrf, sessions.IDFromRequest)(router),
		sessions: sessions,
		svc:      svc,
		admin:    admin,
	}
}

// signIn starts a session for userID and returns the cookies a browser would
// keep.
func (a application) signIn(t *testing.T, userID string) []*http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	if _, err := a.sessions.Start(context.Background(), rec, security.Subject{ID: userID, Tenant: "acme", Verified: true}); err != nil {
		t.Fatalf("signing in: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("the session store set no cookie, so the requests below would arrive as a visitor")
	}
	return cookies
}

// visit makes one request carrying the cookies given, and returns the answer.
func (a application) visit(t *testing.T, method, target string, form url.Values, cookies []*http.Cookie, header http.Header) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

// screen loads the listing with the cookies given and answers the token its
// form carries.
func (a application) screen(t *testing.T, cookies []*http.Cookie) string {
	t.Helper()

	rec := a.visit(t, http.MethodGet, permission.DefaultPrefix+"/groups", nil, cookies, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("the listing answered %d: %s", rec.Code, rec.Body)
	}
	return field(t, rec.Body.String(), "_token")
}

// field is the value of the hidden input called name in a drawn screen.
func field(t *testing.T, body, name string) string {
	t.Helper()
	m := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("the screen draws no %s field:\n%s", name, body)
	}
	return html.UnescapeString(m[1])
}

// href is the target of the link marked with attr in a drawn screen, and
// whether the screen drew that link at all.
func href(body, attr string) (string, bool) {
	m := regexp.MustCompile(`<a ` + regexp.QuoteMeta(attr) + ` href="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return html.UnescapeString(m[1]), true
}

// groups counts the groups of the tenant the forms write to, the seeded
// administering group included.
func (a application) groups(t *testing.T) int {
	t.Helper()
	page, err := a.svc.ListGroups(context.Background(), a.admin, permission.GroupListQuery{Limit: 100})
	if err != nil {
		t.Fatalf("listing what the forms wrote: %v", err)
	}
	return len(page.Items)
}

// submission is the form that creates a group, with the token given, or none.
func submission(slug, token string) url.Values {
	form := url.Values{"slug": {slug}, "name": {slug}}
	if token != "" {
		form.Set("_token", token)
	}
	return form
}

// accepted reports whether a submission was answered with a success or a
// redirect, which is what a handler that wrote answers.
func accepted(code int) bool { return code >= 200 && code < 400 }

func TestASignedInAdministratorSubmitsTheFormTheScreenDrew(t *testing.T) {
	t.Parallel()

	app := serve(t, true, nil)
	session := app.signIn(t, administrator)

	token := app.screen(t, session)
	if token == "" {
		t.Fatal("the screen drew an empty token for a signed-in subject, so every form on it is refused")
	}

	rec := app.visit(t, http.MethodPost, permission.DefaultPrefix+"/groups", submission("editors", token), session, nil)
	if !accepted(rec.Code) {
		t.Fatalf("the form the screen drew answered %d, want a success or a redirect: %s", rec.Code, rec.Body)
	}
	if got := app.groups(t); got != 2 {
		t.Fatalf("after the accepted form the tenant has %d group(s), want the seeded one and the new one", got)
	}
}

// TestAnIssuerHandedToNewIsNotTheOneTheScreensDrawFrom holds that the parameter
// is not read: an issuer over another key, which the middleware would refuse
// every token of, changes nothing about the form the screen draws.
func TestAnIssuerHandedToNewIsNotTheOneTheScreensDrawFrom(t *testing.T) {
	t.Parallel()

	stale := security.NewCSRF([]byte("fedcba9876543210fedcba9876543210"), time.Hour)
	app := serve(t, true, stale)
	session := app.signIn(t, administrator)

	rec := app.visit(t, http.MethodPost, permission.DefaultPrefix+"/groups", submission("editors", app.screen(t, session)), session, nil)
	if !accepted(rec.Code) {
		t.Fatalf("with an issuer handed to New the form the screen drew answered %d, want a success or a redirect: %s", rec.Code, rec.Body)
	}
	if got := app.groups(t); got != 2 {
		t.Fatalf("after the accepted form the tenant has %d group(s), want 2", got)
	}
}

// TestAGuestReachesNoScreenAndWritesNothing is the visitor's half. Every route
// of this module is guarded by an action, and a visitor carries none, so there
// is no screen to draw a token on and no form a guest can submit: the listing
// is refused before a page is composed, and a write is refused whether or not
// it carries a token.
func TestAGuestReachesNoScreenAndWritesNothing(t *testing.T) {
	t.Parallel()

	app := serve(t, true, nil)

	rec := app.visit(t, http.MethodGet, permission.DefaultPrefix+"/groups", nil, nil, nil)
	if rec.Code != http.StatusForbidden {
		t.Errorf("the listing answered a visitor %d, want %d", rec.Code, http.StatusForbidden)
	}
	if strings.Contains(rec.Body.String(), `name="_token"`) {
		t.Errorf("the refusal drew a form for a visitor:\n%s", rec.Body)
	}

	if rec := app.visit(t, http.MethodPost, permission.DefaultPrefix+"/groups", submission("editors", ""), nil, nil); rec.Code != middleware.StatusCSRFExpired {
		t.Errorf("a visitor's submission with no token answered %d, want %d", rec.Code, middleware.StatusCSRFExpired)
	}

	// A guest that does carry a valid guest token is past the middleware, and is
	// refused where the route asks for the action.
	guest := httptest.NewRecorder()
	csrf := security.NewCSRF([]byte(appKey), time.Hour).Secure(false)
	issued := httptest.NewRequest(http.MethodGet, "/", nil)
	token, err := csrf.Issue(csrf.Binding(guest, issued, ""))
	if err != nil {
		t.Fatalf("issuing a guest token: %v", err)
	}
	if rec := app.visit(t, http.MethodPost, permission.DefaultPrefix+"/groups", submission("editors", token), guest.Result().Cookies(), nil); rec.Code != http.StatusForbidden {
		t.Errorf("a visitor's submission with a valid guest token answered %d, want %d", rec.Code, http.StatusForbidden)
	}

	if got := app.groups(t); got != 1 {
		t.Fatalf("the visitor's requests left %d group(s), want only the seeded one", got)
	}
}

func TestAForgedSubmissionIsStillRefused(t *testing.T) {
	t.Parallel()

	app := serve(t, true, nil)
	session := app.signIn(t, administrator)
	token := app.screen(t, session)
	if token == "" {
		t.Fatal("the screen drew no token, so the refusals below would prove nothing about one")
	}
	// Another browser of the same person, holding a session and a token of its
	// own.
	other := app.signIn(t, administrator)

	for _, forged := range []struct {
		why     string
		form    url.Values
		cookies []*http.Cookie
		header  http.Header
		want    int
	}{
		{"no token", submission("intruders", ""), session, nil, middleware.StatusCSRFExpired},
		{"a token and not the session it was issued to", submission("intruders", token), nil, nil, middleware.StatusCSRFExpired},
		{"a token issued to another session", submission("intruders", token), other, nil, middleware.StatusCSRFExpired},
		{"a token that was altered", submission("intruders", token+"x"), session, nil, middleware.StatusCSRFExpired},
		{"a page of another site", submission("intruders", token), session, http.Header{"Sec-Fetch-Site": {"cross-site"}}, http.StatusForbidden},
	} {
		rec := app.visit(t, http.MethodPost, permission.DefaultPrefix+"/groups", forged.form, forged.cookies, forged.header)
		if rec.Code != forged.want {
			t.Errorf("a submission with %s answered %d, want %d", forged.why, rec.Code, forged.want)
		}
	}
	if got := app.groups(t); got != 1 {
		t.Fatalf("the forged submissions left %d group(s), want only the seeded one", got)
	}
}

func TestTheLayoutLinksWhereTheApplicationRegisteredItsRoutes(t *testing.T) {
	t.Parallel()

	app := serve(t, true, nil)
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
		body := rec.Body.String()
		if got, _ := href(body, "data-brand"); got != "/" {
			t.Errorf("%s links the brand to %q, want the route named home, /", target, got)
		}
		if got, drawn := href(body, "data-logout"); !drawn || got != "/auth/logout" {
			t.Errorf("%s links Sign out to %q (drawn: %t), want the route named auth.logout, /auth/logout", target, got, drawn)
		}
		if _, drawn := href(body, "data-login"); drawn {
			t.Errorf("%s offers a signed-in subject the sign-in link", target)
		}
	}

	// An application that registered none of the routes gets no address made up
	// for it: the module reads the route table and does not guess at paths.
	bare := serve(t, false, nil)
	rec := bare.visit(t, http.MethodGet, permission.DefaultPrefix+"/groups", nil, bare.signIn(t, administrator), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("the listing answered %d: %s", rec.Code, rec.Body)
	}
	if got, _ := href(rec.Body.String(), "data-brand"); got != "" {
		t.Errorf("with no route named home the brand links to %q, want nothing", got)
	}
	if got, _ := href(rec.Body.String(), "data-logout"); got != "" {
		t.Errorf("with no route named auth.logout Sign out links to %q, want nothing", got)
	}
}
