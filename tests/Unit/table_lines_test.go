package unit_test

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/arandu-io/framework/foundation"
)

// The words and the finish of the tables the panel draws.

// TestEveryTableSentenceIsWrittenInEveryLocale keeps a table from saying a
// sentence in English because one locale forgot it: Kyse fills an empty line
// with English, so a gap here is invisible on a screen except to its reader.
func TestEveryTableSentenceIsWrittenInEveryLocale(t *testing.T) {
	t.Parallel()

	for _, locale := range []string{"en", "pt-BR"} {
		lines := module(t).Labels(locale).TableLines()
		value := reflect.ValueOf(lines)
		if value.NumField() == 0 {
			t.Fatal("the table has no sentence to check")
		}
		for i := 0; i < value.NumField(); i++ {
			if text := value.Field(i).String(); strings.TrimSpace(text) == "" || strings.HasPrefix(text, "permission.table.") {
				t.Errorf("%s: %s is %q", locale, value.Type().Field(i).Name, text)
			}
		}
	}
	if got := module(t).Labels("pt-BR").TableLines().Filter; got != "Buscar em {label}..." {
		t.Errorf("the pt-BR search placeholder is %q", got)
	}
}

// TestACountAgreesWithItsNoun is the footer's plural: one row is "1 person",
// not "1 person(s)".
func TestACountAgreesWithItsNoun(t *testing.T) {
	t.Parallel()

	en, pt := module(t).Labels("en"), module(t).Labels("pt-BR")
	for _, c := range []struct {
		got, want string
	}{
		{en.Count(1, "person"), "1 person"},
		{en.Count(2, "person"), "2 people"},
		{en.Count(0, "group"), "0 groups"},
		{pt.Count(1, "person"), "1 pessoa"},
		{pt.Count(3, "person"), "3 pessoas"},
		{pt.Count(1, "group"), "1 grupo"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

// TestTheListsHaveTheFinish reads the two published lists for what the finish
// is made of: the panel's sentences, a footer that counts, an empty result that
// names who was looked for, the header menu, and a row that leads to its record.
func TestTheListsHaveTheFinish(t *testing.T) {
	t.Parallel()

	publications, err := foundation.Publications(module(t))
	if err != nil {
		t.Fatal(err)
	}
	read := func(suffix string) string {
		var source string
		for _, publication := range publications {
			err := fs.WalkDir(publication.Files, publication.From, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, suffix) {
					return walkErr
				}
				raw, readErr := fs.ReadFile(publication.Files, path)
				source = string(raw)
				return readErr
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		if source == "" {
			t.Fatalf("%s is not published", suffix)
		}
		return source
	}

	for suffix, want := range map[string][]string{
		"users/index.kyse.go": {
			"data.Labels.TableLines()", `data.Labels.Count(len(rows), "person") + "."`, "data.Cursor == \"\" && data.Next == \"\"", "Paginator: pager",
			`data.Labels.T("members.none_found")`, "HeaderMenu: true", `"table.selected_label"`, `"data-row-link"`,
		},
		"matrix/index.kyse.go": {
			"data.Labels.TableLines()", `data.Labels.Count(len(rows), "group") + "."`, "data.Cursor == \"\" && data.Matrix.Next == \"\"", "Paginator:      pager",
			`data.Labels.T("matrix.none_found")`, "HeaderMenu:     true", `"table.selected_label"`, `"data-row-link"`,
		},
	} {
		source := read(suffix)
		for _, text := range want {
			if !strings.Contains(source, text) {
				t.Errorf("%s does not contain %q", suffix, text)
			}
		}
	}
}
