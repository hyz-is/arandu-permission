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
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("member.title") }}</h1>
			<p class="text-muted-foreground mt-1 font-mono text-xs">{{ .Effective.UserID }}</p>
		</div>
		<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">{{ .Labels.T("groups.title") }}</a>
	</div>

	<section class="mt-8">
		<h2 class="text-sm font-semibold tracking-tight uppercase">{{ .Labels.T("member.groups") }}</h2>
		@if(len(.Effective.Groups) > 0)
			<div class="mt-3 flex flex-wrap gap-1">
				@foreach(.Effective.Groups as group)
					<a href="{{ .Prefix }}/groups/{{ group.ID }}">
						{!! components.Badge(components.BadgeProps{Label: group.Slug, Variant: "secondary"}) !!}
					</a>
				@endforeach
			</div>
		@else
			<p class="text-muted-foreground mt-3 text-sm">{{ .Labels.T("member.none") }}</p>
		@endif
	</section>

	{{-- Every permission says where it comes from. That is the question a
	     permissions screen is asked twice: not what somebody has, but why --
	     and a list without the origin sends whoever is reading it through every
	     group by hand. --}}
	<section class="mt-10">
		<h2 class="text-sm font-semibold tracking-tight uppercase">{{ .Labels.T("member.origin") }}</h2>
		@if(len(.Effective.Grants) > 0)
			<ul class="mt-3 grid gap-2">
				@foreach(.Effective.Grants as grant)
					<li class="flex flex-wrap items-center justify-between gap-3 border-b py-2">
						<span class="flex flex-col">
							<span class="text-sm">{{ .Labels.Action(grant.Action) }}</span>
							<span class="text-muted-foreground font-mono text-xs">{{ string(grant.Action) }}</span>
						</span>
						<span class="flex flex-wrap items-center gap-1">
							@if(grant.Direct)
								{!! components.Badge(components.BadgeProps{Label: .Labels.T("member.direct"), Variant: "outline"}) !!}
							@endif
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
					Title:   .Labels.T("member.empty_title"),
					Message: .Labels.T("member.empty_message"),
				}) !!}
			</div>
		@endif
	</section>

	{{-- What this person carries in their own right, on top of whatever their
	     groups confer. It edits one table, so a box is ticked only for a direct
	     grant: a box that came back ticked because a group granted the action
	     would be a box somebody unticks expecting the permission to go away. --}}
	<section class="mt-12 border-t pt-8">
		<h2 class="text-lg font-semibold tracking-tight">{{ .Labels.T("member.direct_title") }}</h2>
		<p class="text-muted-foreground mt-1 text-sm">
			{{ .Labels.T("member.direct_lead") }}
		</p>

		<form id="direct-form" class="mt-6"
		      hx-post="{{ .Prefix }}/users/{{ .Effective.UserID }}/summary"
		      hx-target="#direct-summary"
		      hx-swap="innerHTML">
			@csrf
			<input type="hidden" name="kind" value="actions">

			<div class="grid gap-8">
				@foreach(.Sections as section)
					<fieldset>
						<legend class="text-sm font-semibold tracking-tight">{{ .Labels.Domain(section.Domain) }}</legend>
						<div class="mt-3 grid gap-2 sm:grid-cols-2">
							@foreach(section.Choices as choice)
								{!! components.Checkbox(components.CheckboxProps{
									Name:    "value",
									ID:      "direct-" + string(choice.Action),
									Label:   .Labels.Action(choice.Action),
									Hint:    string(choice.Action),
									Value:   string(choice.Action),
									Checked: choice.Held,
								}) !!}
							@endforeach
						</div>
					</fieldset>
				@endforeach
			</div>

			<div class="mt-6 flex items-center gap-3">
				{!! components.Button(components.ButtonProps{
					Label:   .Labels.T("control.review"),
					Type:    "submit",
					Variant: "outline",
				}) !!}
				<span class="text-muted-foreground text-xs">{{ .Labels.T("summary.warning") }}</span>
			</div>
		</form>
		<div id="direct-summary" class="mt-6"></div>
	</section>
@endsection
