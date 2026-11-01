//go:build kyse

package groups

import (
	"net/url"

	"github.com/arandu-io/hesape/pagination"
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

func groupsTable(data IndexData) components.DataTableProps {
	rows := make([]components.TableRow, 0, len(data.Groups))
	for _, group := range data.Groups {
		rows = append(rows, components.TableRow{Key: group.ID, Label: group.Name, Cells: []components.TableCell{
			{HTML: components.Link(components.LinkProps{
				Label:   group.Name,
				URL:     data.Prefix + "/groups/" + group.ID,
				Variant: "hover",
				// The identifying cell leads to the group, and a script that
				// follows data-row-link makes the whole row do the same.
				ComponentProps: components.ComponentProps{Attrs: components.Attrs{"data-row-link": ""}},
			})},
			{HTML: components.Badge(components.BadgeProps{Label: group.Slug, Variant: "outline"})},
		}})
	}
	// The finish every list has: the panel's own sentences, a footer that counts
	// the groups with the noun in the right form, and an empty result that names
	// who was looked for. The footer is drawn only where the size of the list is
	// known, and it is when this page holds all of it: the first page, with none
	// after it. A page of a longer list has no total to count, and says none.
	lines := data.Labels.TableLines()
	lines.Rows = data.Labels.Count(len(rows), "group") + "."
	var pager *pagination.LengthAwarePage
	if data.Cursor == "" && data.Next == "" {
		pager = pagination.NewLengthAwarePage(len(rows), len(rows), max(len(rows), 1), 1, pagination.Options{Path: data.Prefix + "/groups"})
	}
	empty := components.EmptyProps{
		Title:   data.Labels.T("groups.empty_title"),
		Message: data.Labels.T("groups.empty_message"),
	}
	if data.Search != "" {
		lines.NoResult = data.Labels.T("groups.none_found")
		empty = components.EmptyProps{
			Title:   data.Labels.T("groups.none_found"),
			Message: data.Labels.T("groups.none_found_message"),
		}
	}
	return components.DataTableProps{
		ComponentProps: components.ComponentProps{Class: "mt-6"},
		ID:             "permission-groups",
		Label:          data.Labels.T("groups.title"),
		URL:            data.Prefix + "/groups",
		Caption:        data.Labels.T("groups.lead"),
		Columns: []components.TableColumn{
			{Label: data.Labels.T("field.name"), Key: "name"},
			{Label: data.Labels.T("field.slug"), Key: "slug", Hideable: true},
		},
		Rows:          rows,
		Paginator:     pager,
		HeaderMenu:    true,
		Lines:         lines,
		ColumnsLabel:  data.Labels.T("control.columns"),
		SelectedLabel: data.Labels.T("table.selected_label"),
		Empty:         empty,
	}
}

func groupsNextURL(data IndexData) string {
	if data.Next == "" {
		return ""
	}
	q := url.Values{}
	q.Set("cursor", data.Next)
	if data.Search != "" {
		q.Set("q", data.Search)
	}
	return data.Prefix + "/groups?" + q.Encode()
}
@endgo

@extends('layouts.app')

@section('content')
	<section class="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("groups.title") }}</h1>
			<p class="text-muted-foreground mt-1 text-sm">
				{{ .Labels.T("groups.lead") }}
			</p>
		</div>
		<nav class="flex flex-wrap items-center gap-2">
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

	{!! components.DataTable(groupsTable(.)) !!}

	@if(groupsNextURL(.) != "")
		<div class="mt-6 flex justify-end">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ groupsNextURL(.) }}">{{ .Labels.T("control.next_page") }}</a>
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
	</section>
@endsection
