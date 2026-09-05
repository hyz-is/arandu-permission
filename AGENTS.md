# Working on Arandu Permission

This is an Arandu package: entities with embedded Hesape Models, policies that
decide about them, one service that owns the database handle, and the routes,
commands and screens that reach it.
It is a Go module somebody `go get`s and registers by hand in their own
`bootstrap/app.go`, which is the whole difference from working in an
application. There is no service provider, no container and no discovery — if a
line of wiring is not written in the installer's repository, it does not happen.

Read `.agents/skills/` before writing code. Each skill is a procedure, and the
one you need is named by the situation you are in.

## The gates

Nothing is finished until all four exit zero.

```sh
export GOWORK=off
gofmt -l $(find . -name '*.go' -not -path '*/testdata/*' -not -name '*.kyse.go')
go build ./...
go vet ./...
go test -race ./...
```

`GOWORK=off` is not borrowed from somewhere else, and here it is not a
preference either. This checkout may sit beside a Go workspace that lists the
framework repositories and does not list this one; when it does, every command
above fails before it compiles anything:

```
pattern ./...: directory prefix . does not contain modules listed in go.work
or their selected dependencies
```

With the workspace off, the module resolves the framework version in `go.mod` —
which is what CI compiles against, and what somebody's `go get` will get.

Both filters on `gofmt` are load-bearing in the toolchain even where this
repository has nothing for them to skip: `gofmt` is the only tool in the chain
that ignores build tags, and `testdata/` is where a fixture is allowed to be
invalid on purpose.

`aru doctor` is not one of the gates, and running it here costs a minute and
answers nothing:

```
this is not an Arandu project: no go.mod, main.go and arandu.toml together.
Run it from inside a project, or create one with `aru new`
```

It exits 1. It reads applications, and this is a library.

It does not read this one after it is installed either, and that is why
`tests/Unit/audit_test.go` exists. The doctor walks the application's own tree,
skips `vendor/`, and opens the one `arandu.mod.toml` at its root; it never loads
a dependency. So `tenant-from-request`, `system-grant-without-tenant` and
`permission-not-declared` never see a line of an installed package, and whatever
this one must prove about itself it proves in its own suite or nowhere.

## What this repository holds

| | measured with |
| --- | --- |
| 11 Go files, one per role, all in one package at the root | `ls *.go \| wc -l` |
| 13 test files | `find tests -name '*_test.go' \| wc -l` |
| 13 routes | `grep -c 'guarded.Action' module.go` |
| 12 actions the policies answer about | `grep -cE '^\t[A-Za-z]+ security.Action = ' policy.go` |
| 8 commands | `grep -c '^\t\t\tRun:' command.go` |
| 2 locales | `ls resources/lang \| wc -l` |
| 2 direct dependencies, both under `arandu-io` | `go list -m -f '{{if and (not .Indirect) (not .Main)}}{{.Path}} {{.Version}}{{end}}' all` |

The layout is by role rather than by layer, so the package reads top to bottom:

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

`Groups(db)` configures the table, string primary key and default `tenant_id`
scope, and `GroupActions`, `GroupUsers`, `UserActions` and `Versions` do the same
for the other four tables. Their terminals return pointers; keep those intact,
because copying an embedded Model leaves its `Entity` pointer aimed at the
original allocation.

**A new Model constructor has to be added to `modelConstructors` in
`tests/Unit/audit_test.go` in the same commit.** A table the ordering audit does
not watch is a table something can reach without a decision, and nothing else
would say so. The audit fails when the list and `model.go` disagree, in both
directions.

## What does not exist here

Reaching for one of these is the most common way to write a package that is
rejected in review. None of them is missing by accident.

| A model reaches for | What is here instead |
| --- | --- |
| a service provider, a container, a `Register()` that discovers things | `New(cfg, db, sessions)`, called by hand in the installer's `bootstrap/app.go`. Everything the package touches is a parameter |
| a global `DB`, an `init()` that opens a connection | the `*data.DB` handed to `New`. A package that opened its own connection would be a package the application cannot point at a test database |
| a CRUD Repository beside the Model | `Groups(db)`, reached only by `PermissionService` after `security.Authorize` |
| a tenant read from the path, the body, the query or a header | `data.Tenant(g)`, from the Grant, which came from the session |
| a permit-all branch in the policy "for now" | nothing. The policy denies, and an action is opened by writing the rule that opens it |
| an `interface{}` config, a map of options, an env var read at call time | the typed `Config` struct, validated by `New` |
| a `panic` on bad wiring | an `error` from `New`. A wiring mistake found at boot costs one restart |
| a third dependency | an argument, first. This module is imported into other people's builds |
| a command of its own that copies files into a project | `Publishes()`, which declares a tagged tree and nothing more. `aru vendor:publish` asks the application which modules it registered and writes what each one declares, so one command serves every installed package instead of one command per package |
| a `package main` anywhere the compiler reads | `Commands()`, a slice of values an application adds to its own console. The example is a `main` behind `//go:build example`, which the compiler never reads unless somebody asks for it |
| a permission stored as a pattern, matched at decision time | `Catalogue.Match`, which expands a selector where it is written. What lands in a row is always a concrete action the catalogue holds |
| a middleware, a helper or a screen that decides on a group name | nothing. A group is where a permission came from; it is renamed by whoever administers it and is never checked against the catalogue |

## The four properties

These are the reason the package is shaped the way it is. A change that breaks
one of them is not merged, whatever else it improves. `tests/Unit/policy_test.go`
checks all four against the code.

1. **The policy denies by default**, and has no branch that allows an action.
   `TestThePolicyDeniesEveryActionByDefault` walks every action with an
   administrator subject and requires
   `security.ErrForbidden` from each.
2. **Every Service method authorizes before it reaches the Model.**
   `TestEveryServiceMethodAuthorizesBeforeTheModel` checks the source, and
   `TestTheServiceRefusesBeforeReachingTheModel` gives the Service a nil handle
   so even constructing `Permissions` in the wrong order fails.
3. **The tenant comes from `data.Tenant(g)`**, on every path, read and write.
   `TestTheTenantComesFromTheGrant` and
   `TestTheServiceWritesTenantOnlyFromTheGrant` hold both halves.
4. **Nothing reaches the Model without passing the first two.** The denial
   suite constructs the Service with a nil database, so a call to
   `Groups(nil)` would panic. Every refusal it asserts is therefore proof
   that authorization happened before Model construction.

`policy_test.go` holds those four by calling the code. `tests/Unit/audit_test.go`
holds the same shape by *reading* it: every exported Service method must call
`Authorize` before its first `Permissions`, every tenant write in the Service
comes from `data.Tenant(g)`, and no tenant accessor reads request input.

It also compares `arandu.mod.toml` against what the code *calls* —
`os.WriteFile`, `exec.Command`, `http.Get`, a method named `Migrations` — rather
than against what it imports, because `net/http` is imported by everything with
a route and says nothing. Both directions fail: used and not declared, which is
`permission-not-declared` where the doctor runs it, and declared and not used,
which is a warning there and a failure here because `go test` has one outcome.
Adding an outbound call, a file write or a process means declaring it in the
same commit, and the suite is what says so.

The five protections against real rows are in `tests/Feature/protections_test.go`
and `tests/Feature/direct_test.go`, and the second file exists because none of
the four carries over to a new table by itself: self-elevation, cross-tenant
access, the stale cache and the last administrator are asked again of the rows a
direct grant writes.

The fifth structural property is not syntax, so it is held where the routes exist.
`TestNoRouteLandsInTheFrameworkNamespace`, in `tests/Feature/routes_test.go`,
registers the module and reads the table back: a prefix arrives through
configuration, and `/_arandu/` is refused when the application boots — in the
installer's process, after publication.

What the audit does not reach is written at the top of the file it lives in. It
reads syntax, so dynamic dispatch, reflection, and wrappers around the named
seams are invisible to it. A green run means no such thing was found written
down, not that none exists.

## Writing code

Everything in the source is in English: identifiers, doc comments, internal
comments, error messages, log messages, and the names and messages of tests.
`pkg.go.dev` publishes the doc comments and its readers are users of this
package.

Every exported symbol carries a doc comment, and the comment documents the
symbol and nothing else. Why a signature is what it is belongs there when it is
a fact about the code — *"the value is held because Go does not build a type
from a string"* stays. A date, an issue number, a version in progress or the
name of another repository does not.

Tests go under `tests/`, in a capitalized category directory declaring a
lowercase external package: `tests/Unit` holds `package unit_test`,
`tests/Feature` holds `package feature_test`. Both import the package by its
module path, which is what makes them see exactly what a caller sees. A test
that genuinely needs something unexported goes beside the code as
`*_internal_test.go`, and the suffix is how it says so.
