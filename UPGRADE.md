# Upgrade Guide

Every release that breaks something names what to replace, here, beside the
version that broke it. CI refuses an incompatible change whose symbols are not
named on this page.

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
carries comes from a group. A seed builds the subject it acts as — an
identifier, the tenant, and the actions of this package as its roles — and calls
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
