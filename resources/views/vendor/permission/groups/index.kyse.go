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
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("groups.title") }}</h1>
			<p class="text-muted-foreground mt-1 text-sm">
				{{ .Labels.T("groups.lead") }}
			</p>
		</div>
		<nav class="flex items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/catalogue">{{ .Labels.T("catalogue.title") }}</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/matrix">{{ .Labels.T("matrix.title") }}</a>
		</nav>
	</div>

	{{-- The search is a GET form so the result has an address: a narrowed
	     listing somebody can send to a colleague is worth more than one that
	     only exists in their tab. --}}
	<form class="mt-8 flex items-end gap-2" method="get" action="{{ .Prefix }}/groups">
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
				   href="{{ .Prefix }}/groups?q={{ .Search }}&cursor={{ .Next }}">{{ .Labels.T("control.next_page") }}</a>
			</div>
		@endif
	@else
		<div class="mt-6">
			{!! components.Empty(components.EmptyProps{
				Title:   .Labels.T("groups.empty_title"),
				Message: .Labels.T("groups.empty_message"),
			}) !!}
		</div>
	@endif

	<section class="mt-12 border-t pt-8">
		<h2 class="text-lg font-semibold tracking-tight">{{ .Labels.T("groups.new") }}</h2>
		<p class="text-muted-foreground mt-1 text-sm">
			{{ .Labels.T("groups.new_lead") }}
		</p>

		<form class="mt-4 grid max-w-lg gap-4" method="post" action="{{ .Prefix }}/groups">
			@csrf
			<div>
				{!! components.Label(components.LabelProps{For: "slug", Text: .Labels.T("field.slug")}) !!}
				{!! components.Input(components.InputProps{
					Name:        "slug",
					ID:          "slug",
					Placeholder: "editors",
					Required:    true,
				}) !!}
			</div>
			<div>
				{!! components.Label(components.LabelProps{For: "name", Text: .Labels.T("field.name")}) !!}
				{!! components.Input(components.InputProps{Name: "name", ID: "name", Required: true}) !!}
			</div>
			<div>
				{!! components.Textarea(components.TextareaProps{
					Name:  "description",
					Label: .Labels.T("field.description"),
					Rows:  3,
				}) !!}
			</div>
			{!! components.Button(components.ButtonProps{Label: .Labels.T("control.create_group"), Type: "submit"}) !!}
		</form>
	</section>
@endsection
