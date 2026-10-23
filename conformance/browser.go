package conformance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/arandu-io/framework/test"
	"github.com/arandu-io/hesape/auth"
)

// browser is one visitor of the application: the cookies the application set
// for it, the CSRF token of the last page it loaded, and what it sends beside
// every request.
//
// The requests go through the fixture's handler in process, and the status is
// read off the answer. The session is the one framework/test As started, so
// every request is read by the application's own guards and resolver.
type browser struct {
	w       *world
	cookies []*http.Cookie
	token   string
	query   url.Values
	header  http.Header
}

// signIn is a browser carrying a session for who, started by framework/test
// As on the application's session store.
func (w *world) signIn(who auth.Subject) *browser {
	w.t.Helper()
	client := test.As(w.t, w.f.Server, auth.Subject{ID: who.ID, Tenant: who.Tenant})
	return &browser{w: w, cookies: client.Cookies()}
}

// guest is a browser with no session.
func (w *world) guest() *browser { return &browser{w: w} }

// tenantHint is one place a request could name a tenant.
type tenantHint struct {
	where string // "query" or "header"
	name  string
}

// tenantHints are the places an application that took the tenant from the
// request would most likely have read it from.
var tenantHints = []tenantHint{
	{"query", "tenant"},
	{"query", "tenant_id"},
	{"query", "organization"},
	{"query", "organization_id"},
	{"query", "workspace"},
	{"header", "X-Tenant"},
	{"header", "X-Tenant-ID"},
	{"header", "X-Organization"},
	{"header", "X-Organization-ID"},
	{"header", "X-Workspace"},
}

// withHint is the same browser naming value at hint on every request.
func (b *browser) withHint(hint tenantHint, value string) *browser {
	named := *b
	named.query, named.header = url.Values{}, http.Header{}
	if hint.where == "header" {
		named.header.Set(hint.name, value)
	} else {
		named.query.Set(hint.name, value)
	}
	return &named
}

// get loads a page and keeps the CSRF token it drew.
func (b *browser) get(address string) int {
	answer := b.send(http.MethodGet, address, nil)
	b.token = tokenOf(answer.Body.String())
	return answer.Code
}

// put submits the bulk form the module's screens send: one value field per
// value, and the token of the last page loaded.
func (b *browser) put(address string, values []string) int {
	form := url.Values{"value": values}
	if b.token != "" {
		form.Set("_token", b.token)
	}
	return b.send(http.MethodPut, address, form).Code
}

// send makes one request through the application's handler.
func (b *browser) send(method, address string, form url.Values) *httptest.ResponseRecorder {
	if len(b.query) > 0 {
		separator := "?"
		if strings.Contains(address, "?") {
			separator = "&"
		}
		address += separator + b.query.Encode()
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequestWithContext(b.w.ctx, method, address, body)
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if b.token != "" {
			request.Header.Set("X-CSRF-Token", b.token)
		}
	}
	for name, values := range b.header {
		request.Header[name] = values
	}
	for _, cookie := range b.cookies {
		request.AddCookie(cookie)
	}

	answer := httptest.NewRecorder()
	b.w.f.Handler.ServeHTTP(answer, request)
	for _, cookie := range answer.Result().Cookies() {
		b.keep(cookie)
	}
	return answer
}

// keep stores a cookie the application set, replacing one of the same name
// and path, and drops one it told the browser to forget.
func (b *browser) keep(cookie *http.Cookie) {
	kept := b.cookies[:0:0]
	for _, have := range b.cookies {
		if have.Name != cookie.Name || have.Path != cookie.Path {
			kept = append(kept, have)
		}
	}
	if cookie.MaxAge >= 0 && (cookie.Expires.IsZero() || cookie.Expires.After(time.Now())) {
		kept = append(kept, cookie)
	}
	b.cookies = kept
}

// tokenOf is the value of the first _token field a page drew, or empty.
func tokenOf(page string) string {
	const marker = `name="_token" value="`
	at := strings.Index(page, marker)
	if at < 0 {
		return ""
	}
	rest := page[at+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}
