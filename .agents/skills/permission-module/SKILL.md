---
name: permission-module
description: Change what this Arandu package registers — its routes, handlers, configuration, response shape or schema. Use when the request is to "add a route", "add an endpoint", "add a handler", "add a config option", "change the prefix", "add a field to the response", "add a column", "write a migration", "return the tenant id too", "add a background job to the package", "make it run something at boot", or when a change touches module.go, config.go or model.go. Covers foundation.Module and the five optional interfaces beside it, why handlers are thin, the migration name that carries the order, and what Resource is for.
license: MIT
---

# The module contract

`foundation.Module` is `Name()` and `Routes()`, and nothing else. That pair is
the whole public contract between a package and the framework: an application
calls `New`, gets something that satisfies it, and registers it by hand.

```go
var (
	_ foundation.Module     = (*Module)(nil)
	_ foundation.Migratable = (*Module)(nil)
)
```

Those two lines are compile-time proof and they belong at the top of
`module.go`. Add one for every contract the module takes on, so the failure lands
here rather than at the registration in somebody else's repository.

## What lives where, now that it is more than routes

```
module.go       registration, routes, handlers and migrations
config.go       what the application passes in
catalogue.go    the closed set of actions, and the selectors that name them
model.go        the entities, and what they may answer with
policy.go       who may do what
service.go      the rules and authorized Model access
resolver.go     what a request carries into every policy, and the route guard
event.go        what the application is told, once it has happened
translation.go  the sentences a screen draws
command.go      the same use cases, from a terminal
views.go        the files the application takes ownership of
```

Four rules cut across them, and each is held by a test that fails in a way that
names itself:

- **A new table means a new name in `modelConstructors`**, in
  `tests/Unit/audit_test.go`, in the same commit. A table the ordering audit does
  not watch is a table something can reach without a decision.
- **A new exported Service method taking a context calls `security.Authorize`
  itself.** It may not take a Grant: an exported method that took one is an
  exported method whose caller decided, and the caller is somebody else's code.
- **A new screen means a new name in `rendered`**, in `tests/Unit/views_test.go`.
  A published view nothing renders is a file an application maintains for
  nothing.
- **A new sentence on a screen means a line in every locale.** The catalogue test
  compares them against each other in both directions, so a line added to one is
  a failure until it is in the others.

A new command is a value in `Commands()` and never a `package main`. The audit
refuses a buildable one, and the reason is in the test: what a program does is
not a capability whoever ran `go get` agreed to.

## Taking on more than routes

Six interfaces sit beside `Module` in `framework/foundation`, one of which this
package already implements, and each is opted into the same way — by
implementing it. Confirm the set for the version in `go.mod` with:

```sh
export GOWORK=off
go doc github.com/arandu-io/framework/foundation
```

| interface | what it is for |
| --- | --- |
| `Bootable` | prepare state once, at boot |
| `Background` | run a loop of the module's own |
| `Schedulable` | declare work for the scheduler |
| `Migratable` | own tables — this package implements it |
| `Health` | report on the storage it depends on |
| `Closable` | give resources back at shutdown |

Two of them change what `arandu.mod.toml` has to say. A `Background` loop that
calls out needs `network = true`; anything that writes a file needs
`filesystem = true`. Declare it in the same commit as the code, or
`TestTheDeclaredCapabilitiesAreWhatTheCodeDoes` fails and names which of the
four it was. Nothing downstream would have caught it: `aru doctor` reads the
application's own tree and never opens an installed package.

## Adding a route

**1. Register it in `Routes`, with a name.**

```go
	r.Action(stdhttp.MethodDelete, m.cfg.Prefix+"/{id}", m.destroy).Name("permission.destroy")
```

The name is what a URL is built from. Two spellings of one address disagree, and
the failure when they do is a link to a 404. The prefix is `m.cfg.Prefix` and
never a literal: the application decides where the package is mounted, and
`TestTheModuleRegistersItsRoutesUnderItsPrefix` at
`tests/Feature/routes_test.go` mounts the module at a configured prefix and fails if any route
came out anywhere else. It also asserts every route is tagged with the module
name, which is what `aru route:list` groups by.

**2. Write the handler thin.** Read the input, ask the service, answer:

```go
func (m *Module) destroy(ctx *fhttp.Context) error {
	if err := m.svc.Delete(ctx.Ctx(), m.subject(ctx.Request), ctx.Param("id")); err != nil {
		return m.answer(ctx, err)
	}
	return ctx.Status(stdhttp.StatusNoContent)
}
```

`ctx.Status` and not `ctx.JSON` for an empty answer: `JSON` calls `ToArray()` on
what it is handed, so a nil resource panics inside the framework rather than
answering 204.

No rule and no Model construction lives in a handler. A handler that held the
database or called `Permissions` would bypass the only place the Policy is
guaranteed to run. Read `permission-policy` before writing the Service method.

**3. Let `answer` translate the refusal.** It knows three: `security.ErrForbidden`
becomes 403, `ErrNotFound` becomes 404, and a `validation.Errors` becomes 422
with the rejected field names, which are the client's own and give nothing away.
Anything else is *returned*, not swallowed — the framework turns it into the
error page in development and a 500 in production, which is the honest outcome.
Answering 200 with an empty body is the failure nobody debugs.

**4. Add the case to the route test.** `TestAVisitorWithNoSessionReachesNothing`
at `tests/Feature/routes_test.go` is a table of every route, and it asserts
403 for each. It runs against `data.Wrap(nil, data.DialectSQLite)` — a handle
over no database — so a route that got past the policy panics rather than
passes.

## Where the subject comes from

`m.subject(r)` loads the session and returns `security.Guest(m.cfg.Tenant)` when
there is none. Nothing else reads who is acting, and no handler takes a user id
from the request.

The guest's tenant is the one place in this package where a tenant does not come
from a Grant, and it is because there is no Grant yet. It comes from
`Config.Tenant`, which is the application's own configuration — never from the
request.

## Changing the Model

`Group` embeds `model.Model[Group]`, and `Groups(db)` is the one
configured entry point for the table. Keep the application-generated key
settings and tenant default visible there:

```go
func Groups(db *data.DB) *model.Model[Group] {
	m := model.NewModel[Permission]("permissions", db, db.GetQueryGrammar(), db.GetPostProcessor())
	m.KeyType = "string"
	m.Incrementing = false
	return m
}
```

Do not set `TenantColumn` to `""`: this package owns tenant data. Model
terminals require a Grant and apply `tenant_id`; the Service still calls
`security.Authorize` first because the Model does not decide which Policy
action the Grant represents.

Keep rows as pointers after `NewInstance`, `First`, `Find`, or `Get`. The
embedded Model's `Entity` points into that allocation, so copying the row and
then calling a promoted terminal would act on the original.

This table declares `created_at` but not `updated_at`. The Hesape Model stamps a
timestamp only when the entity declares its column, so creation remains correct
without changing the published migration or disabling timestamps globally.

## Adding a configuration field

`Config` is a typed struct, and that is the point: a misspelled key in a map is
a setting that silently keeps its default, and here a field that does not exist
does not compile.

Three things move together for every new field.

- **A doc comment on the field**, saying what it means and where it may come
  from.
- **A rule in `Validate`** if there is a value it cannot be. `New` calls
  `Validate` before anything else, so a setting that cannot work fails where it
  is wired rather than on the first request that needed it.
- **A default in `withDefaults`**, as a named constant beside `DefaultPrefix`
  and `DefaultPageSize`, if zero is meant to mean something. `withDefaults` runs
  *after* `Validate` and never before: filling a default in first hides the
  value somebody actually wrote from the check that would have refused it.

A value out of range is refused rather than clamped. A number somebody wrote and
did not get is worse than a number somebody wrote and was told about — that is
why `PageSize` above `MaxPageSize` is an error and not a silent 200.

Add the case to `TestTheConfigurationRefusesWhatCannotWork` at
`tests/Unit/policy_test.go`, which is a map of named bad configurations, and
to `TestNewRefusesAWiringThatCannotWork` at `tests/Feature/routes_test.go`
if the field can make `New` fail.

## What may leave in a response

`Resource` and `Collection` in `model.go` are declared snapshots, not direct
encoding of the entity. This also defines the safe copy boundary: Model-backed
Service results stay as `*Permission`/`[]*Permission`, because copying an embedded
Model preserves a back-pointer to the original allocation. A response snapshot
reads only the explicit fields and cannot be saved.

An encoder handed the entity would answer with whatever fields it happens to
have, including the embedded Model and anything added later without opening the
handler. `TenantID` names another customer's identifier and belongs in no
response.

So a new column that should be visible is added in two places: the struct in
`model.go`, and the map `ToArray` returns. A column that should not be visible
is added in one.

`With()` is what goes *beside* the fields at the top level. `ctx.JSON` puts what
`ToArray` returns under a fixed `"data"` key and merges what `With` returns next
to it, so `Collection` answers `{"data": {"items": [...]}, "next_cursor": "..."}`.
The cursor describes the answer rather than the things answered with. A resource
with nothing to add returns nil.

The cursor is only offered for a full page:

```go
	if len(records) == m.cfg.PageSize {
		cursor = records[len(records)-1].ID
	}
```

A short page is the last one, and offering a cursor for it offers a next page
that comes back empty.

## Adding a migration

Append it to the slice `Migrations()` returns, and give it a name that sorts
after the last one:

```go
func (createPermissions) GetName() string { return "20260823_0001_create_permissions" }
```

**The name carries the order and nothing else does.** `TestTheModuleDeclaresItsSchema`
at `tests/Feature/routes_test.go` requires the returned names to be sorted,
requires none to be empty, and requires every one to satisfy
`migrations.ReversibleMigration` — the migrator finds `Down` by type assertion,
so a `Down` with the wrong signature is a rollback that silently does nothing.

Three rules the existing migration keeps:

- **A name is fixed once the package is published.** Changing what an applied
  name means leaves the change missing everywhere it already ran, and nothing
  says so.
- **Types that spell the same in SQLite, PostgreSQL and MySQL.** Identifier
  columns are `VARCHAR(255)` rather than `TEXT` because they take part in a key
  and MySQL refuses `TEXT` in one without a prefix length. Timestamps get no
  database default: the value comes from Go, for the same reason ids do —
  `gen_random_uuid`, `UUID()` and `randomblob` are three spellings of one idea.
- **An index that matches the `ORDER BY` of the listing, tenant first.** Without
  it every page is a scan of every customer's rows.

Migrations do not run at boot. `aru migrate` is a step in the installer's
pipeline, and every migration has to be compatible with the previous binary
during a rollout: a new column is nullable or has a default, and removing one
takes two releases.
