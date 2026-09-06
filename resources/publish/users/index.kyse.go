//go:build kyse

package users

import (
	"strconv"

	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// IndexData is what the handler hands this page.
//
// It is an alias, for the reason ShowData is one: the shape belongs beside the
// code that fills it.
type IndexData = permission.MembersPageData
@endgo

@extends('layouts.app')

@section('content')
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("members.title") }}</h1>
			<p class="text-muted-foreground mt-1 max-w-prose text-sm">
				{{ .Labels.T("members.lead") }}
			</p>
		</div>
		<nav class="flex items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">{{ .Labels.T("groups.title") }}</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/catalogue">{{ .Labels.T("catalogue.title") }}</a>
		</nav>
	</div>

	{{-- Narrowing by group is a GET form, so a narrowed listing has an address
	     somebody can send to a colleague. --}}
	<form class="mt-8 flex items-end gap-2" method="get" action="{{ .Prefix }}/users">
		<div class="grow">
			{!! components.Label(components.LabelProps{For: "group", Text: .Labels.T("noun.group_one")}) !!}
			<select class="input" name="group" id="group">
				<option value="">{{ .Labels.T("members.all_groups") }}</option>
				@foreach(.Groups as group)
					<option value="{{ group.Slug }}" @if(group.Slug == .Group) selected @endif>{{ group.Name }}</option>
				@endforeach
			</select>
		</div>
		{!! components.Button(components.ButtonProps{Label: .Labels.T("field.search"), Type: "submit", Variant: "outline"}) !!}
	</form>

	@if(len(.Members) > 0)
		<div class="mt-6 overflow-x-auto rounded-md border">
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b">
						<th class="px-3 py-2 font-semibold">{{ .Labels.T("field.identifier") }}</th>
						<th class="px-3 py-2 font-semibold">{{ .Labels.T("noun.group_many") }}</th>
						<th class="px-3 py-2 font-semibold">{{ .Labels.T("member.direct_title") }}</th>
					</tr>
				</thead>
				<tbody>
					@foreach(.Members as person)
						<tr class="border-b">
							<td class="px-3 py-2">
								<a class="font-mono text-xs hover:underline" href="{{ .Prefix }}/users/{{ person.UserID }}">{{ person.UserID }}</a>
							</td>
							<td class="px-3 py-2">
								@if(len(person.Groups) > 0)
									<span class="flex flex-wrap gap-1">
										@foreach(person.Groups as group)
											<a href="{{ .Prefix }}/groups/{{ group.ID }}">
												{!! components.Badge(components.BadgeProps{Label: group.Slug, Variant: "secondary"}) !!}
											</a>
										@endforeach
									</span>
								@else
									<span class="text-muted-foreground text-xs">&middot;</span>
								@endif
							</td>
							<td class="px-3 py-2">
								@if(person.Direct > 0)
									{!! components.Badge(components.BadgeProps{
										Label:   strconv.Itoa(person.Direct) + " " + .Labels.T("members.direct_count"),
										Variant: "outline",
									}) !!}
								@else
									<span class="text-muted-foreground text-xs">&middot;</span>
								@endif
							</td>
						</tr>
					@endforeach
				</tbody>
			</table>
		</div>

		@if(.Next != "")
			<div class="mt-6 flex justify-end">
				<a class="btn" data-variant="outline" data-size="sm"
				   href="{{ .Prefix }}/users?group={{ .Group }}&cursor={{ .Next }}">{{ .Labels.T("control.next_page") }}</a>
			</div>
		@endif
	@else
		<div class="mt-6">
			{!! components.Empty(components.EmptyProps{
				Title:   .Labels.T("members.empty_title"),
				Message: .Labels.T("members.empty_message"),
			}) !!}
		</div>
	@endif
@endsection
