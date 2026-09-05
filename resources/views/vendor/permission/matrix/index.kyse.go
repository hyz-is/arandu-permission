//go:build kyse

package matrix

import (
	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// IndexData is what the handler hands this page.
type IndexData = permission.MatrixPageData
@endgo

@extends('layouts.app')

@section('content')
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("matrix.title") }}</h1>
			<p class="text-muted-foreground mt-1 text-sm">
				{{ .Labels.T("matrix.lead") }}
			</p>
		</div>
		<nav class="flex items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">{{ .Labels.T("groups.title") }}</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/catalogue">{{ .Labels.T("catalogue.title") }}</a>
		</nav>
	</div>

	<form class="mt-8 flex items-end gap-2" method="get" action="{{ .Prefix }}/matrix">
		<div class="grow">
			{!! components.Label(components.LabelProps{For: "q", Text: .Labels.T("field.search")}) !!}
			{!! components.Input(components.InputProps{
				Name:        "q",
				ID:          "q",
				Type:        "search",
				Value:       .Search,
				Placeholder: .Labels.T("control.search_placeholder"),
			}) !!}
		</div>
		{!! components.Button(components.ButtonProps{Label: .Labels.T("field.search"), Type: "submit", Variant: "outline"}) !!}
	</form>

	@if(len(.Matrix.Rows) > 0)
		{{-- The grid is wide by construction: one column per permission. It
		     scrolls inside its own box rather than making the page scroll
		     sideways, which is what makes the first column readable at all. --}}
		<div class="mt-6 overflow-x-auto rounded-md border">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-b">
						<th class="sticky left-0 z-10 bg-background px-3 py-2 font-semibold">{{ .Labels.T("noun.group_one") }}</th>
						@foreach(.Matrix.Actions as action)
							<th class="px-2 py-2 font-normal whitespace-nowrap" title="{{ string(action) }}">{{ .Labels.Action(action) }}</th>
						@endforeach
					</tr>
				</thead>
				<tbody>
					@foreach(.Matrix.Rows as row)
						<tr class="border-b">
							<th class="sticky left-0 z-10 bg-background px-3 py-2 font-medium whitespace-nowrap">
								<a class="hover:underline" href="{{ .Prefix }}/groups/{{ row.Group.ID }}">{{ row.Group.Slug }}</a>
								@if(row.System)
									{!! components.Badge(components.BadgeProps{Label: .Labels.T("group.system"), Variant: "secondary"}) !!}
								@endif
							</th>
							@foreach(row.Cells as cell)
								<td class="px-2 py-2 text-center">
									@if(cell.Held)
										<span aria-label="carried">&check;</span>
									@else
										<span class="text-muted-foreground" aria-label="not carried">&middot;</span>
									@endif
								</td>
							@endforeach
						</tr>
					@endforeach
				</tbody>
			</table>
		</div>

		@if(.Matrix.Next != "")
			<div class="mt-6 flex justify-end">
				<a class="btn" data-variant="outline" data-size="sm"
				   href="{{ .Prefix }}/matrix?q={{ .Search }}&cursor={{ .Matrix.Next }}">{{ .Labels.T("control.next_page") }}</a>
			</div>
		@endif
	@else
		<div class="mt-6">
			{!! components.Empty(components.EmptyProps{
				Title:       .Labels.T("groups.empty_title"),
				Message:     .Labels.T("groups.empty_message"),
				ActionLabel: .Labels.T("control.create_group"),
				ActionURL:   .Prefix + "/groups",
			}) !!}
		</div>
	@endif
@endsection
