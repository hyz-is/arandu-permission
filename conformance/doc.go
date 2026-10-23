// Package conformance is the suite an application that installs Arandu
// Permission runs against its own wiring, in one call from its
// tests/Conformance:
//
//	func TestPermission(t *testing.T) { conformance.Permissions(t, tests.PermissionFixture) }
//
// The package's own suite proves the policies, the service and the resolver
// against rows. What it cannot prove is the application around them: that the
// Roles middleware runs before the application's guards, that the session the
// application signs people into carries the organization the actions are read
// for, that nothing between the request and the policy takes a tenant the
// request named, and that nothing remembers an action after it was taken away.
// Those are properties of bootstrap/app.go, and this kit is how an application
// proves them with its own boot, its own database engine and its own sign-in.
//
// # What the kit proves
//
// [Permissions] runs five subtests, each over a fresh application the fixture
// serves:
//
//   - a group's action reaches the application's own guard, and an account in
//     no group is refused there;
//   - an action stays inside its organization: a membership written in another
//     tenant for the same account does not reach it, whatever the request
//     names, and the same person signed into the other organization carries
//     what that organization granted;
//   - nobody hands out what they do not hold: a delegate who may edit groups
//     is refused a grant they do not carry, putting themselves into a group and
//     giving themselves a permission, and nothing is written when they are;
//   - a revoked action is gone on the next request of the same session,
//     whether the group lost it, the person left the group or a direct grant
//     was taken back;
//   - reading the permissions needs a Grant of the reader's tenant: a visitor
//     and an account with no permission are refused every screen, and an
//     administrator of one tenant is refused the groups of another.
//
// Every refusal is compared against an answer that went through -- the same
// route answering the person who may, the same write made with what the
// delegate holds -- so a request refused for the wrong reason, a missing CSRF
// token or a guard that refuses everybody, fails the kit instead of passing it.
//
// # What it is handed
//
// A [Fixture] is built by the application from its own composition: the
// server framework/test Serve booted, the handler that boot returned, the
// permission module it registered, two tenants, and a way to create an account
// the application signs in. The kit signs people in with framework/test As,
// through the application's session store and subject resolver, and writes the
// groups through the module's own service, so nothing the kit does is a path
// the application does not have. It never opens a database: the engine is the
// one the fixture's server runs on, which is what lets the same kit run on
// SQLite and Postgres.
package conformance
