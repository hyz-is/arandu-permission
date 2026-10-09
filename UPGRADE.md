# Upgrade Guide

## v0.5.2

Nothing to change in an application: this release touches skills, not Go code.
`aru skills:sync` now offers `permission-package` to a project that requires
this version.

## v0.5.1

### A group row can be locked again

Nothing to change. `GroupQuery` gains `GetQuery`, the statement under the query,
so a consumer that locked a group through the generic builder of v0.4.2 keeps
doing so:

```go
groups := permission.Groups(db).Where("slug", "=", "owners")
groups.GetQuery().LockForUpdate()
group, err := groups.First(ctx, grant)
```

## v0.5.0

### Each entity is a concrete type, and the group listing request is `GroupListQuery`

Hesape `v0.47.0` removes the generic model layer, and this release moves to it
with Framework `v0.50.2`. The five entities embed the non-generic `model.Model`,
each table is declared once in `model.go`, and the query that starts from it is
generated beside it. No route, migration, action, policy decision or tenant
rule changed.

**The group listing request is `GroupListQuery`.** `GroupQuery` is the name every
generated query takes after its entity, so the struct `(*PermissionService).ListGroups`
and `(*PermissionService).ViewMatrix` take moved to `GroupListQuery`, with the
same three fields. `GroupQuery.Search`, `GroupQuery.Cursor` and `GroupQuery.Limit`
no longer exist; `GroupQuery` is the query on the groups table now.

```go
// Before
page, err := svc.ListGroups(ctx, actor, permission.GroupQuery{Search: slug, Limit: 100})

// After
page, err := svc.ListGroups(ctx, actor, permission.GroupListQuery{Search: slug, Limit: 100})
```

**The constructors return the generated queries.** `Groups`, `GroupActions`,
`GroupUsers`, `UserActions` and `Versions` take a `model.DB` -- a `*data.DB`
passes unchanged -- and return `*GroupQuery`, `*GroupActionQuery`,
`*GroupUserQuery`, `*UserActionQuery` and `*VersionQuery` instead of
`*model.Model[T]`. A chain that started from them keeps its text, minus the
calls that no longer exist:

| before | now |
|---|---|
| `permission.Groups(db).NewQuery().Where(…)` | `permission.Groups(db).Where(…)` |
| `permission.Groups(db).NewInstance(nil, false)` and `.Entity` | `permission.Groups(db).New()`, which returns `*Group` |
| `Get` → `model.Collection[Group]` | `Get` → `permission.GroupCollection` (`[]*Group`) |
| `b.GetQuery()` on the builder | `q.Base().GetQuery()` |

`First`, `Find` and the other row terminals still return `*Group`, and still
take the Grant.

**The entities no longer carry the model's configuration.** `Group`,
`GroupAction`, `GroupUser`, `UserAction` and `Version` embed `model.Model`, so
the fields and methods `model.Model[T]` promoted onto them are gone: the
configuration fields (`PrimaryKey`, `KeyType`, `Incrementing`, `Timestamps`,
`TenantColumn`, `Table` and the rest) live in the table, which this package
keeps unexported, and `Exists` and `WasRecentlyCreated` are methods,
`row.Exists()`. A copied row refuses every write with `model.ErrUnwired`, so keep
the pointers the queries return.

**Upgrade the floor.** The module requires Hesape `v0.48.0` and Framework
`v0.50.2`, and `arandu.mod.toml` declares `framework = ">= 0.50"`. An application
that pins a Hesape below `v0.47.0` cannot compile this release: every generic
model type it would need is gone from Hesape itself. The published views are
unchanged and need no republish.

<details>
<summary>Every incompatible symbol <code>apidiff</code> reports against v0.4.2</summary>

Most of these are the methods and fields `model.Model[T]` promoted onto the five
entities, which left with the generic type.

```text
(*PermissionService).ListGroups
(*PermissionService).ViewMatrix
Group.ConnectionName
Group.CreatedAtColumn
Group.DeletedAtColumn
Group.Entity
Group.Exists
Group.Grammar
Group.Incrementing
Group.KeyType
Group.NamedScopes
Group.PerPage
Group.PrimaryKey
Group.Processor
Group.RelationResolvers
Group.SoftDeletes
Group.Table
Group.TenantColumn
Group.Timestamps
Group.UpdatedAtColumn
Group.WasRecentlyCreated
GroupAction.ConnectionName
GroupAction.CreatedAtColumn
GroupAction.DeletedAtColumn
GroupAction.Entity
GroupAction.Exists
GroupAction.Grammar
GroupAction.Incrementing
GroupAction.KeyType
GroupAction.NamedScopes
GroupAction.PerPage
GroupAction.PrimaryKey
GroupAction.Processor
GroupAction.RelationResolvers
GroupAction.SoftDeletes
GroupAction.Table
GroupAction.TenantColumn
GroupAction.Timestamps
GroupAction.UpdatedAtColumn
GroupAction.WasRecentlyCreated
GroupActions
GroupQuery.Cursor
GroupQuery.Limit
GroupQuery.Search
GroupUser.ConnectionName
GroupUser.CreatedAtColumn
GroupUser.DeletedAtColumn
GroupUser.Entity
GroupUser.Exists
GroupUser.Grammar
GroupUser.Incrementing
GroupUser.KeyType
GroupUser.NamedScopes
GroupUser.PerPage
GroupUser.PrimaryKey
GroupUser.Processor
GroupUser.RelationResolvers
GroupUser.SoftDeletes
GroupUser.Table
GroupUser.TenantColumn
GroupUser.Timestamps
GroupUser.UpdatedAtColumn
GroupUser.WasRecentlyCreated
GroupUsers
Groups
UserAction.ConnectionName
UserAction.CreatedAtColumn
UserAction.DeletedAtColumn
UserAction.Entity
UserAction.Exists
UserAction.Grammar
UserAction.Incrementing
UserAction.KeyType
UserAction.NamedScopes
UserAction.PerPage
UserAction.PrimaryKey
UserAction.Processor
UserAction.RelationResolvers
UserAction.SoftDeletes
UserAction.Table
UserAction.TenantColumn
UserAction.Timestamps
UserAction.UpdatedAtColumn
UserAction.WasRecentlyCreated
UserActions
Version.ConnectionName
Version.CreatedAtColumn
Version.DeletedAtColumn
Version.Entity
Version.Exists
Version.Grammar
Version.Incrementing
Version.KeyType
Version.NamedScopes
Version.PerPage
Version.PrimaryKey
Version.Processor
Version.RelationResolvers
Version.SoftDeletes
Version.Table
Version.TenantColumn
Version.Timestamps
Version.UpdatedAtColumn
Version.WasRecentlyCreated
Versions
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).AddGlobalScope, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).All, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Append, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).AttributesToArray, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).CallNamedScope, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Create, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Destroy, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).DiscardChanges, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Except, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Find, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).FindMany, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).FindOrFail, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).FindOrNew, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).First, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).FirstOrCreate, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).FirstOrNew, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ForceCreate, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ForceDeleteQuietly, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ForceDeleted, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ForceDeleting, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ForceDestroy, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).FreshTimestamp, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetAppends, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetConnectionName, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetCreatedAtColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetDeletedAtColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetForeignKey, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetGlobalScopes, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetHidden, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetIncrementing, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetKeyName, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetKeyType, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetMorphClass, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetPerPage, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetPrevious, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQualifiedCreatedAtColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQualifiedDeletedAtColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQualifiedKeyName, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQualifiedUpdatedAtColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQueueableConnection, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQueueableID, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetQueueableRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetRawOriginal, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetRelation, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetRouteKey, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetRouteKeyName, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetTable, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetTouchedRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetUpdatedAtColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).GetVisible, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).HasAppended, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).HasGlobalScope, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).HasNamedScope, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Is
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).IsForceDeleting, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).IsIgnoringTouch, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).IsNot, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).IsRelation, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).IsSoftDeletable, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadAggregate, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorph, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorphAggregate, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorphAvg, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorphCount, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorphMax, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorphMin, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).LoadMorphSum, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).MakeHidden
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).MakeVisible
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewBaseQueryBuilder, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewCollection, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewFromBuilder, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewInstance, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewModelQuery, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewQuery, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewQueryForRestoration, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewQueryWithoutRelationships, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewQueryWithoutScope, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewQueryWithoutScopes, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).NewTypedBuilder, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).On, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).OnWriteConnection, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Only, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).OnlyTrashed, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).OriginalIsEquivalent, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).PushQuietly, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).QualifyColumn, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).QualifyColumns, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Query, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Ref, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).RegisterGlobalScopes, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).RegisterModelEvent, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ReplicateQuietly, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ResolveRouteBinding, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ResolveRouteBindingQuery, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ResolveSoftDeletableRouteBinding, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).RestoreQuietly, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Restored, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Restoring, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetAppends, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetConnection, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetHidden, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetIncrementing, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetKeyName, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetKeyType, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetPerPage, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetRelation
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetTable, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetTouchedRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SetVisible, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SoftDeleted, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SyncChanges, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SyncOriginal
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SyncOriginalAttribute, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).SyncOriginalAttributes, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).ToPrettyJSON, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Touches, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UnsetAttribute, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UnsetRelation, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UnsetRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UpdateOrCreate, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UpdateOrFail, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UpdateQuietly, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UpdateTimestamps, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).UsesTimestamps, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).Where, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).WhereKey, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).With, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).WithTrashed, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).WithoutRelations, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupAction]).WithoutTimestamps, method set of *GroupAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).AddGlobalScope, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).All, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Append, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).AttributesToArray, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).CallNamedScope, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Create, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Destroy, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).DiscardChanges, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Except, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Find, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).FindMany, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).FindOrFail, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).FindOrNew, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).First, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).FirstOrCreate, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).FirstOrNew, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ForceCreate, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ForceDeleteQuietly, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ForceDeleted, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ForceDeleting, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ForceDestroy, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).FreshTimestamp, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetAppends, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetConnectionName, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetCreatedAtColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetDeletedAtColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetForeignKey, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetGlobalScopes, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetHidden, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetIncrementing, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetKeyName, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetKeyType, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetMorphClass, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetPerPage, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetPrevious, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQualifiedCreatedAtColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQualifiedDeletedAtColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQualifiedKeyName, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQualifiedUpdatedAtColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQueueableConnection, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQueueableID, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetQueueableRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetRawOriginal, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetRelation, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetRouteKey, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetRouteKeyName, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetTable, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetTouchedRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetUpdatedAtColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).GetVisible, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).HasAppended, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).HasGlobalScope, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).HasNamedScope, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Is
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).IsForceDeleting, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).IsIgnoringTouch, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).IsNot, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).IsRelation, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).IsSoftDeletable, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadAggregate, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorph, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorphAggregate, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorphAvg, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorphCount, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorphMax, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorphMin, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).LoadMorphSum, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).MakeHidden
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).MakeVisible
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewBaseQueryBuilder, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewCollection, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewFromBuilder, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewInstance, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewModelQuery, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewQuery, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewQueryForRestoration, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewQueryWithoutRelationships, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewQueryWithoutScope, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewQueryWithoutScopes, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).NewTypedBuilder, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).On, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).OnWriteConnection, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Only, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).OnlyTrashed, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).OriginalIsEquivalent, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).PushQuietly, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).QualifyColumn, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).QualifyColumns, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Query, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Ref, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).RegisterGlobalScopes, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).RegisterModelEvent, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ReplicateQuietly, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ResolveRouteBinding, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ResolveRouteBindingQuery, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ResolveSoftDeletableRouteBinding, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).RestoreQuietly, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Restored, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Restoring, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetAppends, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetConnection, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetHidden, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetIncrementing, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetKeyName, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetKeyType, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetPerPage, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetRelation
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetTable, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetTouchedRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SetVisible, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SoftDeleted, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SyncChanges, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SyncOriginal
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SyncOriginalAttribute, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).SyncOriginalAttributes, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).ToPrettyJSON, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Touches, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UnsetAttribute, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UnsetRelation, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UnsetRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UpdateOrCreate, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UpdateOrFail, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UpdateQuietly, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UpdateTimestamps, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).UsesTimestamps, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).Where, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).WhereKey, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).With, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).WithTrashed, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).WithoutRelations, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.GroupUser]).WithoutTimestamps, method set of *GroupUser
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).AddGlobalScope, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).All, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Append, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).AttributesToArray, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).CallNamedScope, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Create, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Destroy, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).DiscardChanges, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Except, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Find, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).FindMany, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).FindOrFail, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).FindOrNew, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).First, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).FirstOrCreate, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).FirstOrNew, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ForceCreate, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ForceDeleteQuietly, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ForceDeleted, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ForceDeleting, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ForceDestroy, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).FreshTimestamp, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetAppends, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetConnectionName, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetCreatedAtColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetDeletedAtColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetForeignKey, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetGlobalScopes, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetHidden, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetIncrementing, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetKeyName, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetKeyType, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetMorphClass, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetPerPage, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetPrevious, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQualifiedCreatedAtColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQualifiedDeletedAtColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQualifiedKeyName, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQualifiedUpdatedAtColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQueueableConnection, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQueueableID, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetQueueableRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetRawOriginal, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetRelation, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetRouteKey, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetRouteKeyName, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetTable, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetTouchedRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetUpdatedAtColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).GetVisible, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).HasAppended, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).HasGlobalScope, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).HasNamedScope, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Is
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).IsForceDeleting, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).IsIgnoringTouch, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).IsNot, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).IsRelation, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).IsSoftDeletable, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadAggregate, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorph, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorphAggregate, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorphAvg, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorphCount, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorphMax, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorphMin, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).LoadMorphSum, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).MakeHidden
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).MakeVisible
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewBaseQueryBuilder, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewCollection, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewFromBuilder, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewInstance, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewModelQuery, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewQuery, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewQueryForRestoration, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewQueryWithoutRelationships, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewQueryWithoutScope, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewQueryWithoutScopes, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).NewTypedBuilder, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).On, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).OnWriteConnection, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Only, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).OnlyTrashed, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).OriginalIsEquivalent, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).PushQuietly, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).QualifyColumn, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).QualifyColumns, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Query, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Ref, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).RegisterGlobalScopes, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).RegisterModelEvent, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ReplicateQuietly, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ResolveRouteBinding, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ResolveRouteBindingQuery, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ResolveSoftDeletableRouteBinding, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).RestoreQuietly, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Restored, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Restoring, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetAppends, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetConnection, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetHidden, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetIncrementing, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetKeyName, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetKeyType, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetPerPage, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetRelation
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetTable, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetTouchedRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SetVisible, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SoftDeleted, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SyncChanges, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SyncOriginal
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SyncOriginalAttribute, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).SyncOriginalAttributes, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).ToPrettyJSON, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Touches, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UnsetAttribute, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UnsetRelation, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UnsetRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UpdateOrCreate, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UpdateOrFail, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UpdateQuietly, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UpdateTimestamps, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).UsesTimestamps, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).Where, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).WhereKey, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).With, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).WithTrashed, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).WithoutRelations, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Group]).WithoutTimestamps, method set of *Group
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).AddGlobalScope, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).All, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Append, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).AttributesToArray, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).CallNamedScope, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Create, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Destroy, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).DiscardChanges, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Except, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Find, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).FindMany, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).FindOrFail, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).FindOrNew, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).First, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).FirstOrCreate, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).FirstOrNew, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ForceCreate, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ForceDeleteQuietly, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ForceDeleted, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ForceDeleting, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ForceDestroy, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).FreshTimestamp, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetAppends, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetConnectionName, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetCreatedAtColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetDeletedAtColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetForeignKey, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetGlobalScopes, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetHidden, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetIncrementing, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetKeyName, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetKeyType, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetMorphClass, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetPerPage, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetPrevious, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQualifiedCreatedAtColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQualifiedDeletedAtColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQualifiedKeyName, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQualifiedUpdatedAtColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQueueableConnection, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQueueableID, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetQueueableRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetRawOriginal, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetRelation, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetRouteKey, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetRouteKeyName, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetTable, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetTouchedRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetUpdatedAtColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).GetVisible, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).HasAppended, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).HasGlobalScope, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).HasNamedScope, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Is
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).IsForceDeleting, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).IsIgnoringTouch, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).IsNot, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).IsRelation, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).IsSoftDeletable, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadAggregate, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorph, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorphAggregate, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorphAvg, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorphCount, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorphMax, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorphMin, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).LoadMorphSum, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).MakeHidden
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).MakeVisible
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewBaseQueryBuilder, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewCollection, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewFromBuilder, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewInstance, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewModelQuery, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewQuery, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewQueryForRestoration, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewQueryWithoutRelationships, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewQueryWithoutScope, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewQueryWithoutScopes, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).NewTypedBuilder, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).On, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).OnWriteConnection, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Only, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).OnlyTrashed, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).OriginalIsEquivalent, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).PushQuietly, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).QualifyColumn, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).QualifyColumns, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Query, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Ref, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).RegisterGlobalScopes, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).RegisterModelEvent, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ReplicateQuietly, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ResolveRouteBinding, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ResolveRouteBindingQuery, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ResolveSoftDeletableRouteBinding, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).RestoreQuietly, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Restored, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Restoring, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetAppends, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetConnection, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetHidden, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetIncrementing, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetKeyName, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetKeyType, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetPerPage, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetRelation
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetTable, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetTouchedRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SetVisible, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SoftDeleted, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SyncChanges, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SyncOriginal
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SyncOriginalAttribute, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).SyncOriginalAttributes, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).ToPrettyJSON, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Touches, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UnsetAttribute, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UnsetRelation, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UnsetRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UpdateOrCreate, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UpdateOrFail, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UpdateQuietly, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UpdateTimestamps, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).UsesTimestamps, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).Where, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).WhereKey, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).With, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).WithTrashed, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).WithoutRelations, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.UserAction]).WithoutTimestamps, method set of *UserAction
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).AddGlobalScope, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).All, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Append, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).AttributesToArray, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).CallNamedScope, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Create, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Destroy, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).DiscardChanges, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Except, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Find, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).FindMany, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).FindOrFail, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).FindOrNew, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).First, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).FirstOrCreate, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).FirstOrNew, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ForceCreate, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ForceDeleteQuietly, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ForceDeleted, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ForceDeleting, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ForceDestroy, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).FreshTimestamp, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetAppends, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetConnectionName, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetCreatedAtColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetDeletedAtColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetForeignKey, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetGlobalScopes, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetHidden, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetIncrementing, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetKeyName, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetKeyType, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetMorphClass, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetPerPage, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetPrevious, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQualifiedCreatedAtColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQualifiedDeletedAtColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQualifiedKeyName, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQualifiedUpdatedAtColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQueueableConnection, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQueueableID, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetQueueableRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetRawOriginal, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetRelation, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetRouteKey, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetRouteKeyName, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetTable, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetTouchedRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetUpdatedAtColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).GetVisible, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).HasAppended, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).HasGlobalScope, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).HasNamedScope, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Is
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).IsForceDeleting, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).IsIgnoringTouch, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).IsNot, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).IsRelation, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).IsSoftDeletable, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadAggregate, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorph, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorphAggregate, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorphAvg, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorphCount, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorphMax, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorphMin, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).LoadMorphSum, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).MakeHidden
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).MakeVisible
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewBaseQueryBuilder, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewCollection, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewFromBuilder, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewInstance, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewModelQuery, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewQuery, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewQueryForRestoration, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewQueryWithoutRelationships, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewQueryWithoutScope, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewQueryWithoutScopes, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).NewTypedBuilder, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).On, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).OnWriteConnection, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Only, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).OnlyTrashed, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).OriginalIsEquivalent, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).PushQuietly, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).QualifyColumn, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).QualifyColumns, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Query, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Ref, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).RegisterGlobalScopes, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).RegisterModelEvent, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ReplicateQuietly, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ResolveRouteBinding, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ResolveRouteBindingQuery, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ResolveSoftDeletableRouteBinding, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).RestoreQuietly, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Restored, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Restoring, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetAppends, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetConnection, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetHidden, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetIncrementing, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetKeyName, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetKeyType, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetPerPage, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetRelation
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetTable, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetTouchedRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SetVisible, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SoftDeleted, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SyncChanges, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SyncOriginal
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SyncOriginalAttribute, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).SyncOriginalAttributes, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).ToPrettyJSON, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Touches, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UnsetAttribute, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UnsetRelation, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UnsetRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UpdateOrCreate, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UpdateOrFail, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UpdateQuietly, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UpdateTimestamps, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).UsesTimestamps, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).Where, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).WhereKey, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).With, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).WithTrashed, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).WithoutRelations, method set of *Version
github.com/arandu-io/hesape/database/model.(*Model[github.com/hyz-is/arandu-permission.Version]).WithoutTimestamps, method set of *Version
```

</details>

### Published views move out of `vendor/`

The views this package publishes land in `resources/views/modules/permission/` and
compile to `storage/framework/views/modules/permission`. It used to be `vendor/` in
both, and that address could not work: the go command refuses to import a
package whose path carries a `vendor` element —

```
bootstrap/app.go:98:2: use of vendored package not allowed
```

— and a published view is compiled into a Go package the application has to
import for its `init()` to register anything. So the last step of the install,
the import `(*Module).Boot` asks for, did not build.

The archive was already under `resources/publish`, which is what keeps the files
in the module zip: a file under a directory named `vendor` is dropped from it at
any depth. That fixed the source side and left the destination carrying the
word, and the destination is the address the application looks the views up at.

The value of every view name constant moved with it. The constant names are
unchanged, so code that renders through them keeps compiling, and each one now
answers `modules.permission.…` where it answered `vendor.permission.…`:

- `ViewGroupsIndex`
- `ViewGroupsShow`
- `ViewCatalogue`
- `ViewMatrix`
- `ViewSummary`
- `ViewMember`
- `ViewMembers`

A string written out by hand instead of through the constant stops matching, and
what that produces is a 500 saying no view is registered under the old name.

**A project that already published the old tree** publishes again and removes
the old one by hand:

```sh
aru vendor:publish --tag=view --apply
aru view:build
rm -rf resources/views/vendor/permission storage/framework/views/vendor/permission
```

then deletes the old lines from `vendor-publish.lock` and changes the import in
`bootstrap/app.go` from `storage/framework/views/vendor/permission` to
`storage/framework/views/modules/permission`.

Framework `v0.46.4` and Hesape `v0.37.0` refuse a publication that carries the
reserved name, so this cannot come back quietly.

Every release that breaks something names what to replace, here, beside the
version that broke it. CI refuses an incompatible change whose symbols are not
named on this page.

### Republish the permission views (prepared as v0.4.3, never tagged)

No API, route or migration changes. Republish the permission views so the members group filter uses the native Kyse Select component and no longer carries a template directive inside an HTML attribute:

    aru vendor:publish --tag=view
    aru vendor:publish --tag=view --apply
    aru view:build

## v0.4.2

No package API, route or migration changes. Existing applications keep their published views, by design. To adopt the native DataTable matrix, the DataTable permission-member listing, identifier search and responsive permission-page containers, preview and apply the view publication again, review any reported conflicts, then rebuild the views:

    aru vendor:publish --tag=view
    aru vendor:publish --tag=view --apply
    aru view:build

Application-specific action names remain application translations; pass them through Config.Translator. The module now ships the control.columns label in English and Brazilian Portuguese. The release selects Kyse v0.29.4, whose DataTable keeps search, facets, sort and caller-owned URL parameters together in the native/HTMX query path.

## v0.4.1

No package API, route or migration changes. Update the module normally to select Framework v0.47.1, Hesape v0.41.1 and Kyse v0.29.1. Existing authorization and tenant policies are unchanged. Application-owned published views are not overwritten by this dependency update.

## v0.3.0

### Read the permissions from `Actions`, and leave `Roles` meaning role

This is the release to take before wiring the module into an application that
decides by role, and it is why the previous ones could not be.

The middleware wrote the resolved actions into `Subject.Roles`, because that was
the only list `auth.Subject` had. From the moment it ran, every policy asking
`HasRole("admin")` answered false -- silently, since both sides were `[]string`.

Hesape `v0.28.0` adds a second list, and this fills that one:

```go
// Before: the middleware overwrote this, and HasRole stopped meaning role.
subject.Roles = []string{"user.view", "invoice.create"}

// After: roles are the application's, actions are the module's.
subject.Roles   = []string{"admin"}          // untouched by this module
subject.Actions = []security.Action{"user.view", "invoice.create"}
```

An application that decides by role needs **no change** and keeps working with
the module mounted. One that decides by action asks `subject.Can(action)`.

**An application that was reading actions out of `Roles` has to change**, and
this is the only thing that breaks:

```go
// Before.
if subject.HasRole("invoice.create") { ... }

// After.
if subject.Can(permission.ActionInvoiceCreate) { ... }
```

Three exported names changed with it. All three carried the flat list of
actions, and all three were called Roles:

| before | after |
| --- | --- |
| `Effective.Roles() []string` | `Effective.Actions() []security.Action` |
| `Resolution.Roles []string` | `Resolution.Actions []security.Action` |
| `(*Resolver).Resolve(...) ([]string, error)` | `(...) ([]security.Action, error)` |

### Check your catalogue for a slug this package also declares

`NewCatalogue` refuses an action declared by both. It used to collapse them,
which granted each through the other: somebody given the screen that
administers groups was given the application's `permission.create` as well.

```
permission: "permission.create" is declared by this package and by the
application: ... Rename one of them
```

If your application has its own `permissions` table with `permission.view`,
`permission.create`, `permission.update` or `permission.delete`, boot will now
refuse rather than quietly merge them. Rename yours, or rename nothing and take
this package's screen out -- but the choice is now yours to make rather than
one made for you.

A repeat of your own action across your own lists is still free.

### Upgrade the floor

```sh
go get github.com/arandu-io/hesape@v0.28.0
```

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
