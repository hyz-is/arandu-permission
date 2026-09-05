//go:build kyse

package catalogue

import (
	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// IndexData is what the handler hands this page.
type IndexData = permission.CataloguePageData
@endgo

@extends('layouts.app')

@section('content')
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Permissions</h1>
			<p class="text-muted-foreground mt-1 max-w-prose text-sm">
				Every permission this application declares, and the groups that carry each one.
				The list is the code: a permission is here because some rule reads it, and there is
				no way to add one from this screen.
			</p>
		</div>
		<nav class="flex items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">Groups</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/matrix">Matrix</a>
		</nav>
	</div>

	@if(len(.Domains) > 0)
		<div class="mt-8 grid gap-10">
			@foreach(.Domains as domain)
				<section>
					<h2 class="text-sm font-semibold tracking-tight uppercase">{{ domain.Name }}</h2>
					<ul class="mt-3 grid gap-2">
						@foreach(domain.Entries as entry)
							<li class="flex flex-wrap items-center justify-between gap-3 border-b py-2">
								<span class="font-mono text-xs">{{ string(entry.Action) }}</span>
								<span class="flex flex-wrap items-center gap-1">
									@if(len(entry.Groups) > 0)
										@foreach(entry.Groups as group)
											<a href="{{ .Prefix }}/groups/{{ group.ID }}">
												{!! components.Badge(components.BadgeProps{
													Label:   group.Slug,
													Variant: "secondary",
												}) !!}
											</a>
										@endforeach
									@else
										<span class="text-muted-foreground text-xs">no group carries it</span>
									@endif
								</span>
							</li>
						@endforeach
					</ul>
				</section>
			@endforeach
		</div>
	@else
		<div class="mt-8">
			{!! components.Empty(components.EmptyProps{
				Title:   "No permission declared",
				Message: "The application handed this module an empty catalogue.",
			}) !!}
		</div>
	@endif
@endsection
