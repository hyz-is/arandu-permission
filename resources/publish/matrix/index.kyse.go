//go:build kyse

package matrix

import (
	"html/template"

	"github.com/arandu-io/hesape/pagination"
	"github.com/arandu-io/kyse/components"
	permission "github.com/hyz-is/arandu-permission"
)

@go
// IndexData is what the handler hands this page.
type IndexData = permission.MatrixPageData

func matrixTable(data IndexData) components.DataTableProps {
	columns := make([]components.TableColumn, 0, len(data.Matrix.Actions)+1)
	columns = append(columns, components.TableColumn{
		Label: data.Labels.T("noun.group_one"),
		Key:   "group",
	})
	for _, action := range data.Matrix.Actions {
		columns = append(columns, components.TableColumn{
			Label:    data.Labels.Action(action),
			Key:      string(action),
			Hideable: true,
			Align:    "center",
		})
	}
	rows := make([]components.TableRow, 0, len(data.Matrix.Rows))
	for _, row := range data.Matrix.Rows {
		group := components.Link(components.LinkProps{
			Label:   row.Group.Slug,
			URL:     data.Prefix + "/groups/" + row.Group.ID,
			Variant: "hover",
			// The identifying cell leads to the group, and a script that
			// follows data-row-link makes the whole row do the same.
			ComponentProps: components.ComponentProps{Attrs: components.Attrs{"data-row-link": ""}},
		})
		if row.System {
			group = template.HTML(string(group) + " " + string(components.Badge(components.BadgeProps{
				Label:   data.Labels.T("group.system"),
				Variant: "secondary",
			})))
		}
		cells := make([]components.TableCell, 0, len(row.Cells)+1)
		cells = append(cells, components.TableCell{HTML: group})
		for _, cell := range row.Cells {
			mark := "·"
			if cell.Held {
				mark = "✓"
			}
			cells = append(cells, components.TableCell{Text: mark})
		}
		rows = append(rows, components.TableRow{
			Key:   row.Group.ID,
			Label: row.Group.Slug,
			Cells: cells,
		})
	}
	// The finish every list has: the panel's own sentences, a footer that counts
	// the groups with the noun in the right form, and an empty result that names
	// who was looked for. The footer is drawn only where the size of the list is
	// known, and it is when this page holds all of it: the first page, with none
	// after it. A page of a longer list has no total to count, and says none.
	lines := data.Labels.TableLines()
	lines.Rows = data.Labels.Count(len(rows), "group") + "."
	var pager *pagination.LengthAwarePage
	if data.Cursor == "" && data.Matrix.Next == "" {
		pager = pagination.NewLengthAwarePage(len(rows), len(rows), max(len(rows), 1), 1, pagination.Options{Path: data.Prefix + "/matrix"})
	}
	empty := components.EmptyProps{
		Title:       data.Labels.T("groups.empty_title"),
		Message:     data.Labels.T("groups.empty_message"),
		ActionLabel: data.Labels.T("control.create_group"),
		ActionURL:   data.Prefix + "/groups",
	}
	if data.Search != "" {
		lines.NoResult = data.Labels.T("matrix.none_found")
		empty = components.EmptyProps{
			Title:   data.Labels.T("matrix.none_found"),
			Message: data.Labels.T("matrix.none_found_message"),
		}
	}
	return components.DataTableProps{
		ComponentProps: components.ComponentProps{Class: "mt-6"},
		ID:             "permission-matrix",
		Label:          data.Labels.T("matrix.title"),
		URL:            data.Prefix + "/matrix",
		Caption:        data.Labels.T("matrix.lead"),
		Columns:        columns,
		Rows:           rows,
		Navigable:      true,
		Paginator:      pager,
		HeaderMenu:     true,
		Lines:          lines,
		ColumnsLabel:   data.Labels.T("control.columns"),
		SelectedLabel:  data.Labels.T("table.selected_label"),
		Empty:          empty,
	}
}
@endgo

@extends('layouts.app')

@section('content')
<section class="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{{ .Labels.T("matrix.title") }}</h1>
			<p class="text-muted-foreground mt-1 text-sm">{{ .Labels.T("matrix.lead") }}</p>
		</div>
		<nav class="flex flex-wrap items-center gap-2">
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/groups">{{ .Labels.T("groups.title") }}</a>
			<a class="btn" data-variant="outline" data-size="sm" href="{{ .Prefix }}/catalogue">{{ .Labels.T("catalogue.title") }}</a>
		</nav>
	</div>

	<form class="mt-8 flex flex-wrap items-end gap-2" method="get" action="{{ .Prefix }}/matrix">
		<div class="min-w-64 grow">
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

	{!! components.DataTable(matrixTable(.)) !!}

	@if(.Matrix.Next != "")
		<div class="mt-6 flex justify-end">
			<a class="btn" data-variant="outline" data-size="sm"
			   href="{{ .Prefix }}/matrix?q={{ .Search }}&cursor={{ .Matrix.Next }}">{{ .Labels.T("control.next_page") }}</a>
		</div>
	@endif
</section>
@endsection
