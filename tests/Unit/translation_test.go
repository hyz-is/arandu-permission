package unit_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	permission "github.com/hyz-is/arandu-permission"
)

// What the shipped sentences have to be true about themselves.
//
// A catalogue is data, so the compiler has nothing to say about it. What can go
// wrong is that one locale grows a line the others do not, that an action is
// added without a label, or that a key a screen reads was never written -- and
// each of those shows up as a screen with an identifier where a sentence
// belongs, on somebody else's installation.

func TestTheShippedLocalesAreTheOnesTheDirectoryHolds(t *testing.T) {
	t.Parallel()

	root := filepath.Join(packageRoot(t), "resources", "lang")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading %s: %v", root, err)
	}
	var found []string
	for _, entry := range entries {
		if entry.IsDir() {
			found = append(found, entry.Name())
		}
	}
	sort.Strings(found)

	if got := permission.Locales(); strings.Join(got, " ") != strings.Join(found, " ") {
		t.Errorf("Locales() = %v, and the directory holds %v", got, found)
	}
	for _, required := range []string{"en", "pt-BR"} {
		if !slices.Contains(found, required) {
			t.Errorf("the catalogue does not ship %s", required)
		}
	}
	if permission.FallbackLocale != "en" {
		t.Errorf("FallbackLocale = %s, and the lines are written in English first", permission.FallbackLocale)
	}
}

// TestEveryLocaleCarriesEveryLine keeps one language from quietly falling
// behind another.
//
// A missing line does not fail: it falls back to English, on one screen, for
// whoever reads that language. Nothing says so, which is why this does.
func TestEveryLocaleCarriesEveryLine(t *testing.T) {
	t.Parallel()

	locales := permission.Locales()
	reference := permission.Lines(permission.FallbackLocale)
	if len(reference) == 0 {
		t.Fatalf("the %s catalogue is empty, so every check below would pass by having nothing to compare", permission.FallbackLocale)
	}

	for _, locale := range locales {
		lines := permission.Lines(locale)
		if len(lines) == 0 {
			t.Errorf("%s ships no line", locale)
			continue
		}
		for key := range reference {
			if _, carried := lines[key]; !carried {
				t.Errorf("%s does not carry %s", locale, key)
			}
		}
		for key := range lines {
			if _, known := reference[key]; !known {
				t.Errorf("%s carries %s, which %s does not", locale, key, permission.FallbackLocale)
			}
		}
	}
}

// TestEveryActionOfThisPackageHasALabel holds the half of the catalogue that is
// derived from code.
//
// An action added without a label is a checkbox on the panel that reads as an
// identifier, and the identifier is the one thing a person administering
// permissions was not meant to have to learn.
func TestEveryActionOfThisPackageHasALabel(t *testing.T) {
	t.Parallel()

	for _, locale := range permission.Locales() {
		lines := permission.Lines(locale)
		for _, action := range permission.Actions() {
			key := permission.ActionKeyPrefix + string(action)
			if line, carried := lines[key]; !carried || strings.TrimSpace(line) == "" {
				t.Errorf("%s has no label for %s (key %s)", locale, action, key)
			}
		}
	}
}

// TestEveryKeyAViewReadsIsShipped is the other direction, and it reads the
// markup rather than a list beside it.
//
// A list of the keys the screens use would be a second place to keep the same
// thing, and it would be wrong the first time somebody wrote a sentence into a
// page.
func TestEveryKeyAViewReadsIsShipped(t *testing.T) {
	t.Parallel()

	// The archive, which is not the address the files are published to: the
	// destination carries a vendor segment, and go mod publishes no path that
	// has one.
	root := filepath.Join(packageRoot(t), "resources", "publish")
	read := regexp.MustCompile(`\.Labels\.T\("([^"]+)"\)`)
	lines := permission.Lines(permission.FallbackLocale)

	seen := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".kyse.go") {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range read.FindAllStringSubmatch(string(body), -1) {
			seen++
			key := permission.TranslationGroup + "." + match[1]
			if _, carried := lines[key]; !carried {
				t.Errorf("%s reads %s and the catalogue does not ship it", filepath.Base(path), key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading the views: %v", err)
	}
	if seen == 0 {
		t.Fatal("no view reads a line, so this test proved nothing")
	}
}

// TestALabelFallsBackToTheIdentifier holds what a screen draws for an action
// nobody has written a sentence for.
//
// It is every action of the application except this package's own, so it is the
// ordinary case rather than the edge one. The identifier is what the
// application's developers named it and what they will search for, which makes
// it a better answer than a translation key.
func TestALabelFallsBackToTheIdentifier(t *testing.T) {
	t.Parallel()

	labels := module(t).Labels("pt-BR")

	if got := labels.Action("invoice.delete"); got != "invoice.delete" {
		t.Errorf("an unlabelled action reads as %q, want the identifier", got)
	}
	if got := labels.Domain("invoice"); got != "invoice" {
		t.Errorf("an unlabelled domain reads as %q, want the identifier", got)
	}
	if got := labels.Action(permission.PermissionGrantDirect); got == string(permission.PermissionGrantDirect) {
		t.Errorf("a labelled action reads as its identifier %q", got)
	}
	if got := labels.T("groups.title"); got != "Grupos" {
		t.Errorf("groups.title in pt-BR = %q, want Grupos", got)
	}
	if got := labels.T("nothing.here"); got != "permission.nothing.here" {
		t.Errorf("a key with no line reads as %q, want the key", got)
	}
}

// TestTheZeroLabelsAnswerWithTheKey keeps a screen that was handed nothing from
// looking finished.
func TestTheZeroLabelsAnswerWithTheKey(t *testing.T) {
	t.Parallel()

	var empty permission.Labels
	if got := empty.T("groups.title"); got != "groups.title" {
		t.Errorf("the zero value answers %q", got)
	}
	if got := empty.Action("invoice.delete"); got != "invoice.delete" {
		t.Errorf("the zero value answers %q for an action", got)
	}
}
