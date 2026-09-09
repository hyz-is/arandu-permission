# Arandu Permission

It administers who may do what, and decides nothing.

The authority over every action stays where it already was: a policy written in
Go, reached through a Grant, refusing by default. What this package owns is the
administration of that — named groups, the actions each group carries, the
people in them, and the permissions given to one person in their own right — plus
the middleware that puts the result on the acting subject before any policy runs.

There is no table of permissions and no way to make one.

## Install

```bash
go get github.com/hyz-is/arandu-permission
```

## Wire it

An Arandu application registers a module explicitly. There is no service
provider, no container and no discovery, so these are the lines to paste into
`bootstrap/app.go` and there are no others.

The import, with the other module imports:

```go
import (
	permission "github.com/hyz-is/arandu-permission"
)
```

The construction, in `Build`, after the session store exists and before
`k.Register`:

```go
	permissionModule, err := permission.New(permission.Config{
		Tenant:  cfg.Auth.Tenant,
		Actions: append(myapp.Actions(), permission.Actions()...),
	}, db, sessions, csrf)
	if err != nil {
		return App{}, err
	}
```

And the registration, inside the `k.Register(...)` call already there:

```go
		permissionModule,
```

Then, once, before the application serves:

```bash
aru migrate
```

This package owns four tables, which is why the migration step is not optional
and why `arandu.mod.toml` says `migrations = true`.

## Publish the views

This package carries the markup of its own pages and hands it over instead of
rendering it from the inside, because a page you cannot edit is a page that says
the wrong thing in your product.

Look at what would be written, then write it:

```bash
aru vendor:publish --tag=view
aru vendor:publish --tag=view --apply
```

Nothing is written without `--apply`. The preview lists every file as `create`,
`update`, `unchanged` or `conflict`, and running the command a second time
writes nothing. A file changed outside its `arandu:begin custom` markers is
reported as a conflict and left alone; `--force` publishes over one, and even
then what is inside the markers is carried forward.

The files land under `resources/views/modules/permission/`, and from that point
they are yours. Nothing of this package is compiled beside them, so no view name
is registered twice and no rule has to decide which of two files won — the
consequence being that a view of this package that changes later does not reach
a project that already published it.

Two steps are left to you, and they are left to you because a command that
edited `bootstrap/app.go` behind your back is a command whose output nobody can
explain. Compile what was written:

```bash
aru view:build
```

and import the directory it wrote into, with the other imports:

```go
	_ "your/module/path/storage/framework/views/modules/permission"
```

Without that import the views are not in the binary, and the module refuses to
boot rather than answering the first request that reaches one of them with a
500. The refusal names the view, the command and the import.

## Configuration

| field | required | meaning |
| --- | --- | --- |
| `Tenant` | yes | the customer a visitor with no session is read as. From the application's configuration, never from the request. |
| `Actions` | yes | every action the application may grant. Its own code, spliced with `permission.Actions()`. What is not in it cannot be attached to anything. |
| `Prefix` | no | where the routes are mounted. Defaults to `/permission`. |
| `PageSize` | no | how many records one page answers with. Defaults to 25, refused above 200. |
| `CacheSize` | no | how many resolved subjects one process remembers. Defaults to 4096. It bounds memory and nothing else. |
| `Translator` | no | the application's own catalogue, asked before the sentences this package ships. |
| `Listeners` | no | told what changed, once it has changed. |

`New` returns an error rather than starting half-wired, so a setting that
cannot work fails where it is written instead of on the first request that
needed it.

## Routes

Under the configured prefix, `/permission` by default.

| method | path | name | requires |
| --- | --- | --- | --- |
| `GET` | `/groups` | `permission.index` | `permission.list` |
| `POST` | `/groups` | `permission.store` | `permission.create` |
| `GET` | `/groups/{group}` | `permission.show` | `permission.view` |
| `PUT` | `/groups/{group}` | `permission.update` | `permission.update` |
| `DELETE` | `/groups/{group}` | `permission.destroy` | `permission.delete` |
| `POST` | `/groups/{group}/summary` | `permission.summary` | `permission.view` |
| `PUT` | `/groups/{group}/actions` | `permission.grant` | `permission.grant` |
| `PUT` | `/groups/{group}/members` | `permission.assign` | `permission.assign` |
| `GET` | `/catalogue` | `permission.catalogue` | `permission.list` |
| `GET` | `/matrix` | `permission.matrix` | `permission.list` |
| `GET` | `/users` | `permission.members` | `permission.list` |
| `GET` | `/users/{user}` | `permission.member` | `permission.view` |
| `POST` | `/users/{user}/summary` | `permission.member.summary` | `permission.view` |
| `PUT` | `/users/{user}/actions` | `permission.member.grant` | `permission.grant_direct` |

The declared action decides nothing: the handler still asks the service, the
service still asks the policy, and the policy is still what refuses. It is there
so that a screen listing what may be granted reads the router rather than a list
beside it.

## The first group

Nobody can administer permissions until somebody carries them, and the screen
that creates groups is behind the permission the first group confers. So the
first one comes from outside a request:

```go
	_, err := permissionModule.Service().Bootstrap(ctx, cfg.Auth.Tenant, permission.BootstrapRequest{
		Slug:    "administrators",
		Name:    "Administrators",
		Members: []string{firstUserID},
	})
```

or from a terminal:

```bash
aru permission:create-group administrators Administrators --member=<user id>
```

It is refused the moment the tenant has a group, so there is no way back to it.
Everything it writes goes through the same use cases a screen calls.

## Two ways a permission reaches a person

Through a group, or given to them in their own right — what the reference calls
an extra permission. Both are the same action from the same closed catalogue,
both land in the same resolved set, and the same policy in Go decides with the
result.

The direct grant is the shortest path from *may administer permissions* to *may
do anything*, so it carries two refusals that the group path also carries and
that no reference implementation has: **nobody hands out what they do not hold**,
and **nobody hands anything to themselves**.

## Selectors

An application with a hundred actions is a hundred checkboxes. A selector names
many at once and is read where it is written:

```go
	wanted, err := svc.Catalogue().Match("invoice.*")
```

| selector | names |
| --- | --- |
| `invoice.create` | one action |
| `invoice.create,update` | two actions of one module |
| `invoice.*` | every action of the invoice module, at any depth |
| `*.delete` | the delete of every module |
| `invoice` | everything below invoice |

Nothing stores a selector. What lands in a row is always a concrete action the
catalogue holds, so no decision anywhere has to match a string against a pattern.
A selector that names nothing is refused rather than answered with an empty list.

## Commands

`module.Commands()` returns them; an application adds them to its own console.

| command | what it does |
| --- | --- |
| `permission:show` | every group against every permission |
| `permission:create-group` | a group, or the first group of an installation |
| `permission:grant` / `permission:revoke` | permissions on a group, by name or selector |
| `permission:assign` / `permission:unassign` | people in a group |
| `permission:give` | permissions to one person in their own right |
| `permission:cache-reset` | move the tenant's token, so every process re-reads |

Every one of them takes `--as`, and what that person may do is read out of the
database rather than asserted by whoever typed the command. A command can do what
the person named could have done from the panel, and nothing more.

## The route guard

```go
	r.Group("", permissionModule.Require(myapp.ReportRead)).
		Get("/reports", reports.Index)
```

It refuses and it never admits. It has to run after the middleware that fills in
what a subject may do. There is deliberately no counterpart taking group names: a
group is where a permission came from, it is renamed by whoever administers it,
and it is never checked against the catalogue.

## Translations

The panel ships English and Brazilian Portuguese. The lines are embedded rather
than published — a view is meant to be edited, a sentence is meant to keep up
with the code that produces it.

An application overrides one by defining the same key in its own catalogue and
handing that translator to `Config.Translator`. Its catalogue is asked first and
this one is the floor under it, so what is not overridden keeps coming from here.
`permission.Lines(locale)` is what there is to override.

The label of an action is derived from the action and resolved when the page is
drawn. An action nobody has written a sentence for reads as its identifier, which
is the better answer for an application's own actions: the identifier is what its
developers named it.

## Events

```go
	Listeners: []permission.Listener{func(ctx context.Context, e permission.Event) {
		log.Printf("%s %s by %s: %v", e.Kind, e.GroupSlug, e.ActorID, e.Actions)
	}},
```

Seven kinds, covering every write that could change a decision. Each runs after
the write has committed, on the path of the request that caused it.

## Run it

```bash
go run -tags example ./example
```

Opens SQLite in a temporary directory and walks the whole of the above: the first
group, a selector, both escalation refusals, a direct grant, who may do what and
why, and the token moving on a revocation.

## Model-first data path

`Group` embeds `model.Model[Group]`, and `Groups(db)` is the one
configured entry point for its table. `PermissionService` owns `*data.DB` and
follows `validate -> security.Authorize -> Grant -> Model terminal`; handlers
never hold the database or construct a Model.

`CreateGroup` writes `TenantID` from `data.Tenant(g)`. `FindGroup` authorizes
before reading and again against the row it found. `ListGroups` authorizes before
building its scoped query. The Model keeps its default `tenant_id` scope on every
terminal.

Terminals return `*Group` and `[]*Group`. Keep those pointers intact: copying an
embedded Model leaves its `Entity` pointer aimed at the original allocation.

There is no CRUD Repository. Add one only for a complex query, read model,
report, export or raw SQL contract that the common Model path cannot express.

## Layout

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

## What is already correct, and has to stay that way

**The policy denies everything.** There is no permit-all branch to delete
later. The Service calls `security.Authorize` before its first `Groups(db)`
reach, and every Model terminal requires the Grant that call produced.

**Authorization precedes the Model.** The package audit checks that order in
every exported Service method. A Model terminal enforces tenant scope; the
preceding Policy call decides whether the action itself is allowed.

**The tenant comes from `data.Tenant(g)`.** Never from the path, the body, the
query string or a header. The value on the Grant came from the session; a value
that arrived with the request is a value the caller chose.

**`arandu.mod.toml` declares what the package does** — network, filesystem,
exec, migrations — and the suite compares the declaration against what the code
*calls*, not against what it imports: `net/http` is imported by everything with
a route and says nothing. A package that says it makes no outbound calls and
then opens one fails its own tests, and that is the only place the comparison
happens. `aru doctor` audits the application it is run inside and never loads a
dependency, so nothing audits an installed package except the package itself.

## Tests

```bash
go test -race ./...
```

The denial suite constructs the Service with a nil database, so even building
`Groups(nil)` would panic. The structural twin reads the allowed path and
rejects any Service method that reaches the Model before `Authorize`.

## Licence

MIT. See [LICENSE.md](LICENSE.md). Copyright Paulo R. Lima.
