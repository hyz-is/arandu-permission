//go:build kyse

package users

import (
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/arandu-io/kyse/components"
	permission "github.com/hyz-is/arandu-permission"
)

@go
// IndexData is what the handler hands this page.
type IndexData = permission.MembersPageData

func groupOptions(groups []permission.GroupRef) []components.SelectOption {
	options := make([]components.SelectOption, 0, len(groups))
	for _, group := range groups {
		options = append(options, components.SelectOption{Value: group.Slug, Label: group.Name})
	}
	return options
}

func membersTable(data IndexData) components.DataTableProps {
	rows := make([]components.TableRow, 0, len(data.Members))
	for _, person := range data.Members {
		links := make([]string, 0, len(person.Groups))
		for _, group := range person.Groups {
			links = append(links, string(components.Link(components.LinkProps{
				Label: group.Slug,
				URL: data.Prefix + "/groups/" + group.ID,
				Variant: "hover",
			})))
		}
		cells := []components.TableCell{
			{HTML: components.Link(components.LinkProps{
				Label: person.UserID,
				URL: data.Prefix + "/users/" + person.UserID,
				Variant: "hover",
				ComponentProps: components.ComponentProps{Class: "font-mono text-xs"},
			})},
		}
		if len(links) > 0 {
			cells = append(cells, components.TableCell{HTML: template.HTML(strings.Join(links, " · "))})
		} else {
			cells = append(cells, components.TableCell{Text: "·"})
		}
		if person.Direct > 0 {
			cells = append(cells, components.TableCell{HTML: components.Badge(components.BadgeProps{
				Label: strconv.Itoa(person.Direct) + " " + data.Labels.T("members.direct_count"),
				Variant: "outline",
			})})
		} else {
			cells = append(cells, components.TableCell{Text: "·"})
		}
		rows = append(rows, components.TableRow{Key: person.UserID, Cells: cells})
	}
	return components.DataTableProps{
		ComponentProps: components.ComponentProps{Class: "mt-6"},
		ID: "permission-members",
		Label: data.Labels.T("members.title"),
		URL: data.Prefix + "/users",
		Caption: data.Labels.T("members.lead"),
		Columns: []components.TableColumn{
			{Label: data.Labels.T("field.identifier"), Key: "identifier"},
			{Label: data.Labels.T("noun.group_many"), Key: "groups", Hideable: true},
			{Label: data.Labels.T("member.direct_title"), Key: "direct", Hideable: true},
		},
		Rows: rows,
		ColumnsLabel: data.Labels.T("control.columns"),
		Empty: components.EmptyProps{
			Title: data.Labels.T("members.empty_title"),
			Message: data.Labels.T("members.empty_message"),
		},
	}
}

func membersNextURL(data IndexData) string {
	if data.Next == "" {
		return ""
	}
	q := url.Values{}
	q.Set("cursor", data.Next)
	if data.Search != "" {
		q.Set("q", data.Search)
	}
	if data.Group != "" {
		q.Set("group", data.Group)
	}
	return data.Prefix + "/users?" + q.Encode()
}
@endgo

@extends('layouts.app')

@section('content')
<section class="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("members.title") }}</h1>
			<p class="text-muted-foreground mt-1 max-w-prose text-sm">{{ .Labels.T("members.lead") }}</p>
		</div>
		<nav class="flex items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">{{ .Labels.T("groups.title") }}</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/catalogue">{{ .Labels.T("catalogue.title") }}</a>
		</nav>
	</div>

	<form class="mt-8 flex flex-wrap items-end gap-2" method="get" action="{{ .Prefix }}/users">
		<div class="min-w-64 grow">
			{!! components.Label(components.LabelProps{For: "q", Text: .Labels.T("field.search")}) !!}
			{!! components.Input(components.InputProps{
				Name: "q", ID: "q", Type: "search", Value: .Search,
				Placeholder: .Labels.T("control.search_placeholder"),
			}) !!}
		</div>
		<div class="min-w-56">
			{!! components.Label(components.LabelProps{For: "group", Text: .Labels.T("noun.group_one")}) !!}
			{!! components.Select(components.SelectProps{
				Name: "group", Value: .Group, Options: groupOptions(.Groups),
				Placeholder: .Labels.T("members.all_groups"),
			}) !!}
		</div>
		{!! components.Button(components.ButtonProps{Label: .Labels.T("field.search"), Type: "submit", Variant: "outline"}) !!}
	</form>

	{!! components.DataTable(membersTable(.)) !!}

	@if(membersNextURL(.) != "")
		<div class="mt-6 flex justify-end">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ membersNextURL(.) }}">{{ .Labels.T("control.next_page") }}</a>
		</div>
	@endif
</section>
@endsection
