# Upgrade Guide

Every release that breaks something names what to replace, here, beside the
version that broke it. CI refuses an incompatible change whose symbols are not
named on this page.

## v0.2.3

Nothing to change. `v0.2.1` and `v0.2.2` had no entry in either release file;
they are recorded now, and three tests hold an action or a migration this
package declares to a version heading rather than to `[Unreleased]`.

## v0.2.2

Reinstall, and rebuild the views. The published `v0.2.1` archive carried what
the view compiler writes beside the sources rather than the sources. Run
`aru vendor:publish --tag=view --apply` and `aru view:build` after upgrading.

## v0.2.1

Nothing to change, and everything to reinstall. The published `v0.2.0` archive
was missing its view sources -- `go mod` drops every path with a segment named
`vendor` when it packs a module. The files land at the same addresses under the
same view names; what changed is where the archive carries them.

## v0.2.0

Nothing was removed and nothing changed meaning, so an application on v0.1.0
compiles against this and behaves the same. What follows is what to do to get
the new capabilities, and one migration that is not optional.

### Run the migration

There is a second table, `permission_user_actions`, for a permission given to one
person outside every group. It arrives as a second migration rather than an edit
to the first, so `aru migrate` applies it and nothing that already ran is
rewritten:

```bash
aru migrate
```

Until it is applied, every screen and command that reads what one person carries
fails on a missing table. The rest of the panel is unaffected.

### `Bootstrap` replaces the hand-rolled seed subject

v0.1.0 told you to build the subject yourself. That still compiles, and it should
be replaced: a subject written by hand in a seed has no check on it and stays
runnable forever.

```go
_, err := permissionModule.Service().Bootstrap(ctx, tenant, permission.BootstrapRequest{
	Slug:    "administrators",
	Name:    "Administrators",
	Members: []string{firstUserID},
})
```

It is refused as soon as the tenant has a group, which is the check the seed
never had. It creates the group as `System`, gives it this package's own actions
and puts somebody in it, all through the same use cases a screen calls.

### `NewPermissionService` takes listeners

The signature gained a variadic parameter, so every existing call still compiles.
An application that wants to be told what changed passes `Config.Listeners` to
`New` instead of building the service itself.

### The refusals a client caused are named

`ErrTooMany` and `ErrInvalidMember` were unnamed errors, so a request that named
one member too many was answered as a server error. They are sentinels now and
the routes answer them as 422. An application matching on the error text should
match on the sentinel.

### Two new actions

`permission.grant_direct` and `permission.revoke_direct` are in `Actions()`, so
they land in the catalogue automatically. Nobody holds them until a group carries
them: an existing installation's administrators cannot give a direct permission
until somebody grants them these, which is deliberate — it is a wider power than
editing a group and an upgrade should not confer it silently.

## v0.1.0

The first release. There is nothing to upgrade from.

Two things are worth reading before installing it, because neither is
recoverable by editing a call site afterwards.

### The catalogue is required, and it has to hold this package's own actions

`Config.Actions` is every action the application may grant. It is the
application's own list, read out of its own code, and `New` refuses a
configuration whose catalogue leaves out what `Actions` returns:

```go
permission.Config{
	Tenant:  cfg.Auth.Tenant,
	Actions: append(myapp.Actions(), permission.Actions()...),
}
```

Without them, the administration of permissions is reachable only by whoever
seeded the first group, and there is no screen that can hand it to anybody else.

### The first group comes from outside a request

Nobody can administer permissions until somebody carries them, and what somebody
carries comes from a group. In v0.1.0 a seed built the subject it acts as — an
identifier, the tenant, and the actions of this package as its roles — and called
the same service every screen calls:

```go
actor := security.Subject{ID: "seed", Tenant: tenant}
for _, action := range permission.Actions() {
	actor.Roles = append(actor.Roles, string(action))
}

group, err := service.CreateGroup(ctx, actor, permission.CreateGroupRequest{
	Slug:   "administrator",
	Name:   "Administrator",
	System: true,
})
```

`System` marks a group the installation depends on: it cannot be deleted, its
actions cannot be taken off it, and it cannot be left without a member. The
handler that answers the create form never sets it — it reads three named fields
and that is not one of them.

From v0.2.0 this is `Bootstrap`, which does the same thing with a guard on it.
