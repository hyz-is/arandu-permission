package unit_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/arandu-io/framework/security"

	permission "github.com/hyz-is/arandu-permission"
)

// What a selector is for, and what it deliberately cannot do.
//
// A selector names many permissions at once, which is the thing somebody
// administering a hundred actions needs and clicking a hundred boxes is not. It
// is read where it is written and never stored: what lands in a row is the
// actions it resolved to, so no decision anywhere has to match a string against
// a pattern.
//
// The cases below are the shapes the reference implementation guarantees, run
// against a finite catalogue rather than an open one. That is the whole of the
// difference: the answer is bounded by what the application declared.

// wide is a catalogue with enough shape to tell the rules apart: two modules,
// one of them three levels deep.
func wide(t *testing.T) permission.Catalogue {
	t.Helper()

	catalogue, err := permission.NewCatalogue(
		"invoice.create", "invoice.update", "invoice.delete", "invoice.view",
		"invoice.line.create", "invoice.line.delete",
		"report.view", "report.delete",
	)
	if err != nil {
		t.Fatalf("building the catalogue: %v", err)
	}
	return catalogue
}

func TestASelectorNamesTheActionsTheCatalogueHolds(t *testing.T) {
	t.Parallel()

	catalogue := wide(t)
	for _, c := range []struct {
		selector string
		want     []string
	}{
		{"invoice.create", []string{"invoice.create"}},
		{"invoice.create,update", []string{"invoice.create", "invoice.update"}},
		{"invoice.*", []string{
			"invoice.create", "invoice.delete", "invoice.line.create", "invoice.line.delete",
			"invoice.update", "invoice.view",
		}},
		{"*.delete", []string{"invoice.delete", "report.delete"}},
		{"report", []string{"report.delete", "report.view"}},
		{"invoice.line", []string{"invoice.line.create", "invoice.line.delete"}},
		{"invoice.*.delete", []string{"invoice.line.delete"}},
		{"*", []string{
			"invoice.create", "invoice.delete", "invoice.line.create", "invoice.line.delete",
			"invoice.update", "invoice.view", "report.delete", "report.view",
		}},
	} {
		matched, err := catalogue.Match(c.selector)
		if err != nil {
			t.Errorf("%s: %v", c.selector, err)
			continue
		}
		got := make([]string, 0, len(matched))
		for _, action := range matched {
			got = append(got, string(action))
		}
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s matched %v, want %v", c.selector, got, c.want)
		}
	}
}

// TestASelectorCoversWhateverDepthAnActionHas holds the rule that makes a
// module name mean the module: a selector that runs out covers everything below
// where it stopped, and a selector longer than the action names nothing.
func TestASelectorCoversWhateverDepthAnActionHas(t *testing.T) {
	t.Parallel()

	catalogue := wide(t)

	deep, err := catalogue.Match("invoice")
	if err != nil {
		t.Fatalf("invoice: %v", err)
	}
	if len(deep) != 6 {
		t.Errorf("invoice matched %d actions, want the whole module", len(deep))
	}

	if _, err := catalogue.Match("invoice.create.line"); !errors.Is(err, permission.ErrNoMatch) {
		t.Errorf("a selector longer than every action = %v, want ErrNoMatch", err)
	}
	wild, err := catalogue.Match("invoice.*")
	if err != nil {
		t.Fatalf("invoice.*: %v", err)
	}
	if len(wild) != len(deep) {
		t.Errorf("invoice.* matched %d actions and invoice matched %d; a wildcard covers the depth below it too", len(wild), len(deep))
	}
}

// TestASelectorThatNamesNothingIsRefused is the divergence from the reference,
// and it is deliberate. There the catalogue is open, so nothing can say whether
// a pattern is a typo. Here it is closed and read before the selector is, so a
// selector that matches nothing has exactly one explanation.
func TestASelectorThatNamesNothingIsRefused(t *testing.T) {
	t.Parallel()

	catalogue := wide(t)
	for _, selector := range []string{"invoce.*", "invoice.publish", "*.publish", "ledger"} {
		if _, err := catalogue.Match(selector); !errors.Is(err, permission.ErrNoMatch) {
			t.Errorf("%s = %v, want ErrNoMatch", selector, err)
		}
	}
}

func TestASelectorThatIsNotOneIsRefusedBeforeItIsMatched(t *testing.T) {
	t.Parallel()

	catalogue := wide(t)
	for _, selector := range []string{"", "invoice..create", "invoice.create,", "invoice. create", "Invoice.*", " invoice.*"} {
		matched, err := catalogue.Match(selector)
		if err == nil {
			t.Errorf("%q was read as a selector and matched %v", selector, matched)
			continue
		}
		if errors.Is(err, permission.ErrNoMatch) {
			t.Errorf("%q was answered as an empty match rather than as a malformed selector", selector)
		}
	}
}

// TestASelectorCannotWidenPastTheCatalogue is the property the whole mechanism
// rests on: whatever a selector says, what comes back is a subset of what the
// application declared.
func TestASelectorCannotWidenPastTheCatalogue(t *testing.T) {
	t.Parallel()

	catalogue, err := permission.NewCatalogue("invoice.create", "invoice.view")
	if err != nil {
		t.Fatalf("building the catalogue: %v", err)
	}
	matched, err := catalogue.Match("*")
	if err != nil {
		t.Fatalf("*: %v", err)
	}
	for _, action := range matched {
		if !catalogue.Has(action) {
			t.Errorf("%s came back from a selector and is not in the catalogue", action)
		}
	}
	if len(matched) != catalogue.Len() {
		t.Errorf("* matched %d of %d actions", len(matched), catalogue.Len())
	}

	var _ security.Action = matched[0]
}
