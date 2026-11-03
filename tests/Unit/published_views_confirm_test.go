package unit_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A published view asks before it deletes with a Kyse Dialog. hx-confirm is
// refused in them: htmx turns it into window.confirm, the browser's own box,
// which no stylesheet reaches and which a person who sees it takes for a defect
// of the product. The group screen's delete button used to carry one.
func TestNoPublishedViewAsksWithTheBrowsersConfirmBox(t *testing.T) {
	root := filepath.Join(packageRoot(t), "resources", "publish")
	seen := 0
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(file, ".kyse.go") {
			return err
		}
		seen++
		body, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, refused := range []string{"hx-confirm", "HxConfirm", "window.confirm"} {
			if strings.Contains(string(body), refused) {
				t.Errorf("%s writes %s: ask with a Dialog, which is drawn over the page", strings.TrimPrefix(file, root+string(filepath.Separator)), refused)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("no published view was read")
	}
}
