//go:build kyse

package groups

import (
	"github.com/arandu-io/kyse/components"

	permission "github.com/hyz-is/arandu-permission"
)

@go
// IndexData is what the handler hands this page.
//
// It is an alias rather than a struct declared here, so that the shape is
// written once, beside the code that fills it. A second declaration is two
// shapes kept in step by hand, and a field missing from one of them is a blank
// space on a page that answered 200.
type IndexData = permission.GroupsPageData
@endgo

@extends('layouts.app')

@section('content')
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Groups</h1>
			<p class="text-muted-foreground mt-1 text-sm">
				A group carries permissions. A person carries what their groups carry.
			</p>
		</div>
		<nav class="flex items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/catalogue">Permissions</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/matrix">Matrix</a>
		</nav>
	</div>

	{{-- The search is a GET form so the result has an address: a narrowed
	     listing somebody can send to a colleague is worth more than one that
	     only exists in their tab. --}}
	<form class="mt-8 flex items-end gap-2" method="get" action="{{ .Prefix }}/groups">
		<div class="grow">
			{!! components.Label(components.LabelProps{For: "q", Text: "Search"}) !!}
			{!! components.Input(components.InputProps{
				Name:        "q",
				ID:          "q",
				Type:        "search",
				Value:       .Search,
				Placeholder: "slug or name",
			}) !!}
		</div>
		{!! components.Button(components.ButtonProps{Label: "Search", Type: "submit", Variant: "outline"}) !!}
	</form>

	@if(len(.Groups) > 0)
		<ul class="mt-6 grid gap-3">
			@foreach(.Groups as group)
				<li>
					{!! components.Card(components.CardProps{
						Title:       group.Name,
						Href:        .Prefix + "/groups/" + group.ID,
						Description: group.Slug,
					}) !!}
				</li>
			@endforeach
		</ul>

		@if(.Next != "")
			<div class="mt-6 flex justify-end">
				<a class="btn" data-variant="outline" data-size="sm"
				   href="{{ .Prefix }}/groups?q={{ .Search }}&cursor={{ .Next }}">Next page</a>
			</div>
		@endif
	@else
		<div class="mt-6">
			{!! components.Empty(components.EmptyProps{
				Title:   "No group here",
				Message: "Nothing matches, or nothing has been created yet.",
			}) !!}
		</div>
	@endif

	<section class="mt-12 border-t pt-8">
		<h2 class="text-lg font-semibold tracking-tight">New group</h2>
		<p class="text-muted-foreground mt-1 text-sm">
			The slug is the stable name and does not change afterwards.
		</p>

		<form class="mt-4 grid max-w-lg gap-4" method="post" action="{{ .Prefix }}/groups">
			@csrf
			<div>
				{!! components.Label(components.LabelProps{For: "slug", Text: "Slug"}) !!}
				{!! components.Input(components.InputProps{
					Name:        "slug",
					ID:          "slug",
					Placeholder: "editors",
					Required:    true,
				}) !!}
			</div>
			<div>
				{!! components.Label(components.LabelProps{For: "name", Text: "Name"}) !!}
				{!! components.Input(components.InputProps{Name: "name", ID: "name", Required: true}) !!}
			</div>
			<div>
				{!! components.Textarea(components.TextareaProps{
					Name:  "description",
					Label: "Description",
					Rows:  3,
				}) !!}
			</div>
			{!! components.Button(components.ButtonProps{Label: "Create group", Type: "submit"}) !!}
		</form>
	</section>
@endsection
