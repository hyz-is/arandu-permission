//go:build kyse

package groups

import (
	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// ShowData is what the handler hands this page.
//
// It is an alias, for the reason IndexData is one: the shape belongs beside the
// code that fills it.
type ShowData = permission.GroupPageData
@endgo

@extends('layouts.app')

@section('content')
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Group.Name }}</h1>
			<p class="text-muted-foreground mt-1 font-mono text-xs">{{ .Group.Slug }}</p>
			@if(.Description != "")
				<p class="text-muted-foreground mt-2 max-w-prose text-sm">{{ .Description }}</p>
			@endif
		</div>
		<div class="flex items-center gap-2">
			@if(.System)
				{!! components.Badge(components.BadgeProps{Label: "system", Variant: "secondary"}) !!}
			@endif
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">Back</a>
		</div>
	</div>

	@if(.System)
		<div class="mt-6">
			{!! components.Alert(components.AlertProps{
				Title:   "This group holds the installation together",
				Message: "It cannot be deleted, its permissions cannot be taken off it, and it cannot be left without a member.",
			}) !!}
		</div>
	@endif

	<section class="mt-10">
		<h2 class="text-lg font-semibold tracking-tight">Name</h2>
		<form class="mt-4 grid max-w-lg gap-4" hx-put="{{ .Prefix }}/groups/{{ .Group.ID }}">
			@csrf
			<div>
				{!! components.Label(components.LabelProps{For: "name", Text: "Name"}) !!}
				{!! components.Input(components.InputProps{
					Name:     "name",
					ID:       "name",
					Value:    .Group.Name,
					Required: true,
				}) !!}
			</div>
			<div>
				{!! components.Textarea(components.TextareaProps{
					Name:  "description",
					Label: "Description",
					Value: .Description,
					Rows:  3,
				}) !!}
			</div>
			{!! components.Button(components.ButtonProps{Label: "Save", Type: "submit"}) !!}
		</form>
	</section>

	{{-- The permissions of this group.
	     The form does not submit itself. It asks for a summary first, which is
	     swapped in below it, and the button on that summary is what writes --
	     so what somebody approves and what is applied are one computation of the
	     difference rather than two readings of the same intent. --}}
	<section class="mt-12 border-t pt-8">
		<h2 class="text-lg font-semibold tracking-tight">Permissions</h2>
		<p class="text-muted-foreground mt-1 text-sm">
			Every permission this application declares. What is not ticked is not carried.
		</p>

		<form id="actions-form" class="mt-6"
		      hx-post="{{ .Prefix }}/groups/{{ .Group.ID }}/summary"
		      hx-target="#actions-summary"
		      hx-swap="innerHTML">
			@csrf
			<input type="hidden" name="kind" value="actions">

			<div class="grid gap-8">
				@foreach(.Sections as section)
					<fieldset>
						<legend class="text-sm font-semibold tracking-tight">{{ section.Domain }}</legend>
						<div class="mt-3 grid gap-2 sm:grid-cols-2">
							@foreach(section.Choices as choice)
								{!! components.Checkbox(components.CheckboxProps{
									Name:     "value",
									ID:       "action-" + string(choice.Action),
									Label:    string(choice.Action),
									Value:    string(choice.Action),
									Checked:  choice.Held,
									Disabled: .System,
								}) !!}
							@endforeach
						</div>
					</fieldset>
				@endforeach
			</div>

			@if(!.System)
				<div class="mt-6 flex items-center gap-3">
					{!! components.Button(components.ButtonProps{
						Label:   "Review changes",
						Type:    "submit",
						Variant: "outline",
					}) !!}
					<span class="text-muted-foreground text-xs">Nothing is written until the summary is approved.</span>
				</div>
			@endif
		</form>
		<div id="actions-summary" class="mt-6"></div>
	</section>

	<section class="mt-12 border-t pt-8">
		<h2 class="text-lg font-semibold tracking-tight">Members</h2>
		<p class="text-muted-foreground mt-1 text-sm">
			One identifier per line. Removing everybody from a system group is refused.
		</p>

		<form id="members-form" class="mt-4 grid max-w-lg gap-4"
		      hx-post="{{ .Prefix }}/groups/{{ .Group.ID }}/summary"
		      hx-target="#members-summary"
		      hx-swap="innerHTML">
			@csrf
			<input type="hidden" name="kind" value="members">

			<ul class="grid gap-2">
				@foreach(.Members as member)
					<li class="flex items-center justify-between gap-3">
						<a class="font-mono text-xs hover:underline" href="{{ .Prefix }}/users/{{ member }}">{{ member }}</a>
						<label class="text-muted-foreground flex items-center gap-2 text-xs">
							<input type="checkbox" name="value" value="{{ member }}" checked>
							keep
						</label>
					</li>
				@endforeach
			</ul>

			<div>
				{!! components.Label(components.LabelProps{For: "value", Text: "Add somebody"}) !!}
				{!! components.Input(components.InputProps{
					Name:        "value",
					ID:          "value",
					Placeholder: "user identifier",
				}) !!}
			</div>

			{!! components.Button(components.ButtonProps{
				Label:   "Review changes",
				Type:    "submit",
				Variant: "outline",
			}) !!}
		</form>
		<div id="members-summary" class="mt-6"></div>
	</section>

	@if(!.System)
		<section class="mt-12 border-t pt-8">
			<h2 class="text-lg font-semibold tracking-tight">Delete</h2>
			<p class="text-muted-foreground mt-1 text-sm">
				The group, what it carries and who is in it, all at once.
			</p>
			{{-- The verb is on the form and not on the button: a component takes
			     the HTMX attributes it supports as fields of its own, and the
			     delete verb is not one of them. --}}
			<form class="mt-4"
			      hx-delete="{{ .Prefix }}/groups/{{ .Group.ID }}"
			      hx-confirm="Delete this group and every membership on it?">
				@csrf
				{!! components.Button(components.ButtonProps{
					Label:   "Delete this group",
					Type:    "submit",
					Variant: "destructive",
				}) !!}
			</form>
		</section>
	@endif
@endsection
