//go:build kyse

package users

import (
	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// ShowData is what the handler hands this page.
type ShowData = permission.MemberPageData
@endgo

@extends('layouts.app')

@section('content')
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Effective permissions</h1>
			<p class="text-muted-foreground mt-1 font-mono text-xs">{{ .Effective.UserID }}</p>
		</div>
		<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">Groups</a>
	</div>

	<section class="mt-8">
		<h2 class="text-sm font-semibold tracking-tight uppercase">Groups</h2>
		@if(len(.Effective.Groups) > 0)
			<div class="mt-3 flex flex-wrap gap-1">
				@foreach(.Effective.Groups as group)
					<a href="{{ .Prefix }}/groups/{{ group.ID }}">
						{!! components.Badge(components.BadgeProps{Label: group.Slug, Variant: "secondary"}) !!}
					</a>
				@endforeach
			</div>
		@else
			<p class="text-muted-foreground mt-3 text-sm">In no group, and therefore carrying nothing.</p>
		@endif
	</section>

	{{-- Every permission says where it comes from. That is the question a
	     permissions screen is asked twice: not what somebody has, but why --
	     and a list without the origin sends whoever is reading it through every
	     group by hand. --}}
	<section class="mt-10">
		<h2 class="text-sm font-semibold tracking-tight uppercase">Permissions, and where each comes from</h2>
		@if(len(.Effective.Grants) > 0)
			<ul class="mt-3 grid gap-2">
				@foreach(.Effective.Grants as grant)
					<li class="flex flex-wrap items-center justify-between gap-3 border-b py-2">
						<span class="font-mono text-xs">{{ string(grant.Action) }}</span>
						<span class="flex flex-wrap items-center gap-1">
							@foreach(grant.Groups as group)
								<a href="{{ .Prefix }}/groups/{{ group.ID }}">
									{!! components.Badge(components.BadgeProps{Label: group.Slug}) !!}
								</a>
							@endforeach
						</span>
					</li>
				@endforeach
			</ul>
		@else
			<div class="mt-3">
				{!! components.Empty(components.EmptyProps{
					Title:   "Nothing yet",
					Message: "No group this person belongs to carries a permission.",
				}) !!}
			</div>
		@endif
	</section>
@endsection
