package permission

import (
	"context"
	"net/http"
	"sync"

	fhttp "github.com/arandu-io/framework/http"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/auth"
)

// Resolver answers what the acting subject may do, and remembers the answer
// only while the tenant's token says it is still the answer.
//
// The expensive half of the question -- the memberships, the groups, the
// actions those groups carry, and the assembly of the three -- is what is
// remembered. The cheap half, the tenant's token, is read every time, because
// what a remembered answer costs is the window between a permission being taken
// away and the person losing it. A remembered answer that outlives a revocation
// is the defect this whole mechanism exists to have none of, and a time-based
// expiry is exactly that defect with a number attached.
//
// The memory is this process's. Another replica that changes a permission
// changes the token, and every process reads the token before it trusts what it
// remembers, so nothing has to be told about anything.
type Resolver struct {
	service *PermissionService

	mu      sync.RWMutex
	entries map[resolverKey]resolverEntry
	// limit bounds how many subjects are remembered. It is a bound on memory
	// and not a policy: forgetting early costs one query and can never serve a
	// permission that was taken away.
	limit int
}

// resolverKey is one subject of one customer. The tenant is part of it because
// two customers can have a person with the same identifier, and an entry that
// left it out would answer one of them with the other's permissions.
type resolverKey struct {
	tenant string
	user   string
}

// resolverEntry is a remembered answer and the token it was read at.
type resolverEntry struct {
	version int64
	roles   []string
}

// NewResolver wires a resolver over the service, remembering at most limit
// subjects. A limit of zero or less means DefaultCacheSize.
func NewResolver(service *PermissionService, limit int) *Resolver {
	if limit <= 0 {
		limit = DefaultCacheSize
	}
	return &Resolver{service: service, entries: map[resolverKey]resolverEntry{}, limit: limit}
}

// Resolve returns the effective actions of the subject.
//
// It authorizes like everything else here: the subject asks about its own rows
// and the policy admits exactly that. A subject with no identifier is answered
// with nothing rather than with a query, because there is nobody to answer
// about.
func (r *Resolver) Resolve(ctx context.Context, subject security.Subject) ([]string, error) {
	if subject.ID == "" || subject.Tenant == "" {
		return nil, nil
	}

	version, err := r.service.Version(ctx, subject)
	if err != nil {
		return nil, err
	}

	key := resolverKey{tenant: subject.Tenant, user: subject.ID}
	r.mu.RLock()
	entry, remembered := r.entries[key]
	r.mu.RUnlock()
	if remembered && entry.version == version {
		return append([]string(nil), entry.roles...), nil
	}

	resolved, err := r.service.ResolveOwn(ctx, subject)
	if err != nil {
		return nil, err
	}
	r.remember(key, resolverEntry{version: resolved.Version, roles: resolved.Roles})
	return append([]string(nil), resolved.Roles...), nil
}

// Forget drops every remembered answer.
//
// It exists for the process that has just changed permissions and wants the
// next request in the same process to see them without waiting for anything.
// It is not what makes revocation correct -- the token is -- and nothing else
// calls it.
func (r *Resolver) Forget() {
	r.mu.Lock()
	defer r.mu.Unlock()
	clear(r.entries)
}

// remember stores an answer, forgetting everything when the map is full.
//
// Clearing rather than evicting one entry is deliberate: a least-recently-used
// list is a second data structure to keep correct, and what it would buy is a
// few queries on a process holding more subjects than the bound. The bound is
// the setting; this is what happens at it.
func (r *Resolver) remember(key resolverKey, entry resolverEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) >= r.limit {
		clear(r.entries)
	}
	r.entries[key] = entry
}

// Roles returns middleware that fills in what the acting subject may do.
//
// Mount it after whatever establishes the session and before anything that
// consults a policy. Every policy in the application reads the actions it
// leaves on the subject, so a request that skipped it is a request where every
// policy sees a subject carrying nothing -- which refuses rather than admits,
// and is the direction a missing step has to fail in.
//
// It carries the subject on the request's context. A handler reads it with
// auth.SubjectFrom, and one that loads the session itself instead gets the
// subject as it was stored, without the actions -- so the two are not
// interchangeable, and this is the one that has them.
//
// A request with no session passes through untouched: there is nobody to
// resolve, and answering a visitor with a refusal here would refuse them the
// public pages too.
func (m *Module) Roles() fhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, err := m.sessions.Load(r.Context(), r)
			if err != nil || subject.ID == "" {
				next.ServeHTTP(w, r)
				return
			}

			roles, err := m.roles.Resolve(r.Context(), subject)
			if err != nil {
				// The request is refused rather than carried on with an empty
				// set. An unreadable permission store is not a subject with no
				// permissions: answering it as one turns a database that is
				// down into a screen that says somebody's access was removed.
				fhttp.Refuse(w, r, http.StatusInternalServerError, "permissions could not be read")
				return
			}

			subject.Roles = roles
			next.ServeHTTP(w, r.WithContext(auth.WithSubject(r.Context(), subject)))
		})
	}
}

// Require returns middleware that refuses a request unless the acting subject
// carries one of these actions.
//
// It refuses and it never admits. Nothing it lets through has been authorized by
// it: the handler still asks the service, the service still asks the policy, and
// the policy is still what decides. What it buys is that a request which was
// never going to be allowed is answered before the handler runs -- before a row
// is read, before a page is composed, and with one rule written at the route
// instead of the same rule repeated in every handler behind it.
//
// The actions are alternatives. A subject carrying any one of them passes, which
// is what a screen reachable by two different roles needs; a route that wants
// two at once is two calls, and reads as the conjunction it is.
//
// It has to run after Roles, which is what puts the actions on the subject.
// Before it, every subject carries nothing and this refuses everybody -- which
// is the direction a missing step has to fail in, and still the wrong answer.
//
// There is deliberately no counterpart taking group names. A group is not a
// permission: it is where a permission came from, it is renamed by whoever
// administers it, and it is never checked against the catalogue. A rule written
// against one would be a decision nothing in this package validated, taken on a
// string somebody can edit on a screen. What a route wants when it reaches for a
// role name is the permission that role carries, and that is what this takes.
func (m *Module) Require(actions ...security.Action) fhttp.Middleware {
	// The set is built once, at wiring time, rather than per request. It also
	// makes a Require with no action refuse everything, which is what a route
	// that named nothing asked for.
	wanted := make(map[string]bool, len(actions))
	for _, action := range actions {
		wanted[string(action)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, carried := auth.SubjectFrom(r.Context())
			if !carried || subject.ID == "" {
				// A visitor with no session is refused as unauthenticated
				// rather than as forbidden. The two are different instructions:
				// one says sign in, the other says ask somebody for access, and
				// answering the first as the second sends people to the wrong
				// place.
				fhttp.Refuse(w, r, http.StatusUnauthorized, "unauthorized")
				return
			}
			for role := range wanted {
				if subject.HasRole(role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			// No detail. Which permission was wanted is a fact about what
			// exists, and answering with it maps the application one request at
			// a time.
			fhttp.Refuse(w, r, http.StatusForbidden, "forbidden")
		})
	}
}
