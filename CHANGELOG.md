# Changelog

Everything worth knowing about a release of Arandu Permission is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

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

[Unreleased]: https://github.com/hyz-is/arandu-permission/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/hyz-is/arandu-permission/releases/tag/v0.1.0
