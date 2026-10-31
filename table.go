package permission

import (
	"strconv"

	"github.com/arandu-io/kyse/components"
)

// The words and the finish of a data table.
//
// A table in a published view says the same few sentences as every other table
// of the application it sits in -- the placeholder of the search box, what it
// says when nothing matched, the pager, the columns menu -- and Kyse says them
// in English wherever the caller leaves them empty. They are here, with the
// rest of the panel's sentences, so they follow the locale of the request and
// the application's own override of any line, instead of being written into a
// view the project then has to keep in step.

// TableLines are the sentences a data table says, in this locale. The views hand
// them to the table as its Lines, and an application that wants other words
// writes them under the same keys ("permission.table.filter") in its own
// catalogue.
func (l Labels) TableLines() components.DataTableLines {
	say := func(key string) string { return l.T("table." + key) }
	return components.DataTableLines{
		Filter:         say("filter"),
		NoResult:       say("no_result"),
		Rows:           say("rows"),
		SelectedCount:  say("selected_count"),
		RowsPerPage:    say("rows_per_page"),
		PageOf:         say("page_of"),
		First:          say("first"),
		Previous:       say("previous"),
		Next:           say("next"),
		Last:           say("last"),
		Back:           say("back"),
		Forward:        say("forward"),
		SelectAll:      say("select_all"),
		SelectRow:      say("select_row"),
		Bulk:           say("bulk"),
		Apply:          say("apply"),
		Ascending:      say("ascending"),
		Descending:     say("descending"),
		Unsorted:       say("unsorted"),
		Reset:          say("reset"),
		ClearFilters:   say("clear_filters"),
		NoFacetResults: say("no_facet_results"),
		FacetSelected:  say("facet_selected"),
		OpenMenu:       say("open_menu"),
		Actions:        say("actions"),
		ToggleColumns:  say("toggle_columns"),
		Asc:            say("asc"),
		Desc:           say("desc"),
		Hide:           say("hide"),
	}
}

// Count is a number with the noun it counts, in the form that agrees with it:
// "1 person", "2 people". The entity is the name its lines are written under
// ("person", "group"): "entity.person_one" and "entity.person_many".
func (l Labels) Count(n int, entity string) string {
	form := "_many"
	if n == 1 {
		form = "_one"
	}
	return strconv.Itoa(n) + " " + l.T("entity."+entity+form)
}
