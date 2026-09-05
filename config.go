package permission

import (
	"fmt"
	"net/http"

	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/translation"
)

// The defaults for the optional settings. They are constants rather than
// literals inside Config.withDefaults, so the value a reader finds here is the
// value the package uses.
const (
	// DefaultPrefix is where the routes are mounted when Config leaves Prefix
	// empty.
	DefaultPrefix = "/permission"
	// DefaultPageSize is how many groups one page answers with when Config
	// leaves PageSize at zero.
	DefaultPageSize = 25
	// MaxPageSize is the ceiling PageSize is refused above. A page nobody
	// bounded is a page that reads the whole table on the day the table is
	// large.
	MaxPageSize = 200
	// DefaultCacheSize is how many resolved subjects one process remembers
	// before it forgets all of them.
	DefaultCacheSize = 4096
)

// Config is what the application passes when it wires this package.
//
// A typed struct rather than a map: a misspelled key in a map is a setting that
// silently keeps its default, and the failure shows up as behaviour nobody
// asked for rather than as an error. Here a field that does not exist does not
// compile.
type Config struct {
	// Tenant is the customer a visitor with no session is read as.
	//
	// It is required, and it comes from the application's own configuration --
	// never from the request. A tenant a visitor could name is a visitor who
	// chooses whose rows they read. Everywhere there is a session, the tenant
	// comes from the Grant instead, and this value is not consulted at all.
	Tenant string

	// Actions is every action the application may grant, and it is required.
	//
	// It is the application's own list, read out of its code, and it is handed
	// in rather than discovered: a set assembled at boot would hold only the
	// modules that happened to be linked into this binary, and the screen
	// administers permissions for the whole application.
	//
	// It has to contain this package's own actions, which Actions returns, or
	// the administration of permissions would be reachable only by whoever
	// seeded the first group. Splice them in:
	//
	//	Actions: append(myapp.Actions(), permission.Actions()...)
	//
	// Repeats cost nothing. What is not in it cannot be attached to a group,
	// which is the whole of why a screen cannot invent a permission.
	Actions []security.Action

	// Prefix is the path the routes are mounted under. Empty means
	// DefaultPrefix.
	Prefix string

	// PageSize is how many groups one page answers with. Zero means
	// DefaultPageSize, and anything above MaxPageSize is refused rather than
	// clamped: a number somebody wrote and did not get is worse than a number
	// somebody wrote and was told about.
	PageSize int

	// Translator is the application's own catalogue, asked before the one this
	// package ships.
	//
	// It is optional. Nil means the screens read the shipped sentences, which
	// is the right default for an application that has not translated anything
	// -- a panel in English beats a panel showing its own keys.
	//
	// An application that sets it overrides a sentence by defining the same key
	// in its own catalogue. Nothing has to be copied and nothing goes stale:
	// what is not overridden keeps coming from here, including lines added by a
	// later release. Lines returns what there is to override.
	Translator *translation.Translator

	// Listeners are told what changed, after it has changed.
	//
	// They are handed in here rather than registered afterwards, so that what
	// an application does when a permission moves is written at the one place
	// the module is wired and read there. An empty list is the ordinary case
	// and costs nothing: there is no flag to turn events on, because a flag
	// beside an empty list is two ways to say the same thing and only one of
	// them would be checked.
	//
	// Each one runs on the path of the request that caused the change. See
	// Listener for what that means.
	Listeners []Listener

	// CacheSize is how many resolved subjects one process remembers. Zero means
	// DefaultCacheSize. It bounds memory and nothing else: the remembered
	// answer is only used while the tenant's token is unchanged, so forgetting
	// early costs a query and can never serve a permission that was revoked.
	CacheSize int
}

// Validate reports what the configuration cannot be used with.
//
// It is called by New, so an application with a setting that cannot work fails
// where it is wired rather than on the first request that needed it.
func (c Config) Validate() error {
	if c.Tenant == "" {
		return fmt.Errorf("permission: Config.Tenant is required: a visitor with no session has to be read as some customer, and it cannot be one the request names")
	}
	// The same rule the framework applies to every tenant it accepts. A tenant
	// is concatenated into a storage path, a cache key and a lock name, so one
	// carrying a separator lands in another tenant's namespace.
	if !security.ValidTenant(c.Tenant) {
		return fmt.Errorf("permission: Config.Tenant is %q, which cannot be a tenant: lowercase letters, digits, - and _, up to 64 characters", c.Tenant)
	}

	catalogue, err := NewCatalogue(c.Actions...)
	if err != nil {
		return err
	}
	var missing []security.Action
	for _, action := range Actions() {
		if !catalogue.Has(action) {
			missing = append(missing, action)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("permission: Config.Actions leaves out %v, which this package's own screens grant: append permission.Actions() to it, or the only people who can administer permissions are the ones a seed put in a group",
			missing)
	}

	if c.Prefix != "" && c.Prefix[0] != '/' {
		return fmt.Errorf("permission: Config.Prefix is %q and has to start with /", c.Prefix)
	}
	if c.Prefix != "" {
		if err := validateRoutePrefix(c.Prefix); err != nil {
			return err
		}
	}
	if c.PageSize < 0 || c.PageSize > MaxPageSize {
		return fmt.Errorf("permission: Config.PageSize is %d, and has to be between 0 and %d, where 0 means %d", c.PageSize, MaxPageSize, DefaultPageSize)
	}
	if c.CacheSize < 0 {
		return fmt.Errorf("permission: Config.CacheSize is %d, and has to be 0 or more, where 0 means %d", c.CacheSize, DefaultCacheSize)
	}
	return nil
}

// validateRoutePrefix asks the standard library to parse the exact patterns the
// module will register. Its parser is not exported and reports invalid patterns
// by panic, so the throwaway mux turns that boot-time panic into the
// configuration error New promises.
func validateRoutePrefix(prefix string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("permission: Config.Prefix %q cannot be registered as a route path", prefix)
		}
	}()

	handler := http.NotFoundHandler()
	mux := http.NewServeMux()
	for _, pattern := range routePatterns(prefix) {
		mux.Handle(pattern.method+" "+pattern.path, handler)
	}
	return nil
}

// withDefaults returns the configuration with the optional fields filled in.
//
// It runs after Validate and never before: filling a default in first would
// hide the value somebody actually wrote from the check that would have refused
// it.
func (c Config) withDefaults() Config {
	if c.Prefix == "" {
		c.Prefix = DefaultPrefix
	}
	if c.PageSize == 0 {
		c.PageSize = DefaultPageSize
	}
	if c.CacheSize == 0 {
		c.CacheSize = DefaultCacheSize
	}
	return c
}
