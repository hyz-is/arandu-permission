package unit_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/arandu-io/framework/foundation"
)

func TestMembersPublicationUsesNativeDataTableInsteadOfRawTable(t *testing.T) {
	publications, err := foundation.Publications(module(t))
	if err != nil {
		t.Fatal(err)
	}
	var source string
	for _, publication := range publications {
		err = fs.WalkDir(publication.Files, publication.From, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, "users/index.kyse.go") {
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
		t.Fatal("members published view not found")
	}
	for _, want := range []string{
		"components.DataTable(membersTable(.))",
		"ID: \"permission-members\"",
		"Name: \"q\"",
		"name=\"group\"",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("members view does not contain %q", want)
		}
	}
	if strings.Contains(source, "<table") {
		t.Error("members view bypasses Kyse with a raw HTML table")
	}
}
