package unit_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/arandu-io/framework/foundation"
)

func TestMatrixPublicationUsesNativeDataTableInsteadOfRawGrid(t *testing.T) {
	publications, err := foundation.Publications(module(t))
	if err != nil {
		t.Fatal(err)
	}
	var source string
	for _, publication := range publications {
		err = fs.WalkDir(publication.Files, publication.From, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, "matrix/index.kyse.go") {
				return walkErr
			}
			raw, readErr := fs.ReadFile(publication.Files, path)
			if readErr != nil {
				return readErr
			}
			source = string(raw)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if source == "" {
		t.Fatal("matrix published view not found")
	}
	for _, want := range []string{
		"components.DataTable(matrixTable(.))",
		"Navigable:      true",
		`max-w-7xl px-4 py-8`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("matrix view does not contain %q", want)
		}
	}
	if strings.Contains(source, "<table") {
		t.Error("matrix view bypasses Kyse with a raw HTML table")
	}
	if !strings.Contains(source, `Hideable: true`) {
		t.Error("permission columns cannot be managed through the native table column control")
	}
}
