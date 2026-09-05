# Changelog

Everything worth knowing about a release of Arandu Permission is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

## [0.2.0] - 2026-09-05

Parity with what the reference administers, ported as effect rather than as
form. Nothing was removed: an application on v0.1.0 compiles against this
unchanged, and `aru migrate` is the one step that is not optional.

### Added

- A permission given to one person in their own right, outside every group —
  what the reference calls an extra permission. `UserAction`, `UserActions`,
  `UserActionPolicy`, and `DirectActionsOf`, `PreviewDirectActions` and
  `SetDirectActions` on the service, with a screen and two routes. It is the
  same action from the same closed catalogue, resolved into the same set, and
  `UserActionPolicy` refuses the two moves that matter: handing out what you do
  not hold, and handing anything to yourself.
- `Catalogue.Match`: a selector naming many permissions at once —
  `invoice.*`, `*.delete`, `invoice.create,update`. It is read where it is
  written and never stored, so what lands in a row is always a concrete action
  the catalogue holds. A selector that names nothing is `ErrNoMatch`.
- `PermissionService.Bootstrap` and `BootstrapRequest`: the first group of a
  tenant, refused as soon as there is one. It replaces the subject a seed used
  to build by hand, which had no such check and stayed runnable forever.
- `Event`, `EventKind`, `Listener` and `Config.Listeners`: seven kinds covering
  every write that could change a decision, each dispatched after the write has
  committed.
- `(*Module).Require`: a route guard that refuses and never admits. There is no
  counterpart taking group names, and the omission is deliberate.
- `(*Module).Commands`: eight commands over the same use cases — `show`,
  `create-group`, `grant`, `revoke`, `assign`, `unassign`, `give` and
  `cache-reset`. Each takes `--as`, and what that person may do is read out of
  the database rather than asserted by whoever typed the command.
- `PermissionService.Refresh`, which moves the tenant's token so every process
  re-reads. It is what the reference's cache reset becomes where there is no
  shared cache to clear.
- A catalogue of sentences in English and Brazilian Portuguese, embedded rather
  than published. `Labels`, `(*Module).Labels`, `Lines`, `Locales`,
  `TranslationGroup`, `ActionKeyPrefix`, `DomainKeyPrefix`. The label of an
  action is derived from the action and resolved when the page is drawn; one
  nobody has written a sentence for reads as its identifier.
- `(*Module).Service`, so a seed, a migration or a job reaches the same
  authorized use cases a screen calls.
- `permission.grant_direct` and `permission.revoke_direct`, in `Actions()`.
- `ErrNoMatch`, `ErrTooMany`, `ErrInvalidMember` and `ErrBootstrapped`, each
  answered with its own status. The first two used to be unnamed errors, so a
  client that sent one member too many was told the server had failed.
- One migration, `20260905_0002_create_permission_user_actions`.
- A runnable example: `go run -tags example ./example`.

### Changed

- `NewPermissionService` takes variadic listeners. Existing calls compile
  unchanged.
- `EffectiveGrant` gained `Direct` and `Effective` gained `Direct`, so a screen
  can say that a permission came from nobody's group.
- `SummaryPageData` gained `Target`, and every page carries `Labels`.
- The published views read their sentences from the catalogue instead of
  carrying English in the markup.

### Notes

- There is still no permissions table and still no way to make one. Two sources
  now reach a person, and they meet in the same resolved set before anything
  decides anything.

## [0.1.0] - 2026-09-05

The first release. It administers who may do what, and decides nothing.

### Added

- `Group`, `GroupAction`, `GroupUser` and `Version`, with `Groups`,
  `GroupActions`, `GroupUsers` and `Versions` as the configured, tenant-scoped
  models over the four tables this package owns.
- `Catalogue` and `NewCatalogue`: the closed set of actions a group may carry,
  built from what the application's own code declares and handed to `New` as
  `Config.Actions`. A write naming anything outside it is refused, which is what
  keeps a screen from inventing a permission no rule reads.
- `GroupPolicy`, `ActionPolicy` and `MembershipPolicy`. Each answers about one
  entity, and between them they hold the three refusals that matter: a record of
  another tenant, granting an action the subject does not hold, and putting
  oneself into a group.
- `PermissionService`, the one owner of the database handle. Every use case
  authorizes before it reaches a model, and the tenant of every row it writes
  comes from the Grant.
- `(*Module).Roles`, the middleware that fills in what the acting subject may
  do. It reads the tenant's version token on every request and re-reads the
  memberships only when the token changed, so a revoked permission cannot be
  served from a remembered answer.
- `Actions`, this package's own actions, for an application to splice into its
  catalogue.
- Six published views: the group listing, one group, the catalogue by domain,
  the group-by-permission matrix, the summary a bulk change is approved from,
  and one person's effective permissions with the group each of them comes from.
- One migration, `20260905_0001_create_permission_tables`, creating
  `permission_groups`, `permission_group_actions`, `permission_group_users` and
  `permission_versions`.

### Notes

- The minimum Framework version is `v0.46.0`, with Hesape `v0.25.1`.
- There is no permissions table and no way to make one. A row here links a group
  to a permission that already exists because some policy in Go reads it.

[Unreleased]: https://github.com/hyz-is/arandu-permission/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/hyz-is/arandu-permission/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/hyz-is/arandu-permission/releases/tag/v0.1.0
