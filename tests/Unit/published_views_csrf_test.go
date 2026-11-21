package unit_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	permission "github.com/hyz-is/arandu-permission"
)

// csrfProvider is what @csrf asks the data of a view for.
type csrfProvider interface{ CSRFToken() string }

// TestEveryViewWithAFormIsHandedDataThatProvidesTheToken holds the contract
// @csrf has with the data of a view: a form written in a view needs the data
// that view declares to answer CSRFToken, or the first request that draws it
// is a 500.
//
// The page data that embeds the page chrome answers it from there. A fragment
// is not a page and does not embed the chrome, so it has to answer on its own,
// and it was the one that did not. The sources are read rather than rendered
// because the application's layout is not here to render them: each view names
// the type it is drawn from with an alias to this package, and that type is
// held to the interface the compiled markup asserts.
func TestEveryViewWithAFormIsHandedDataThatProvidesTheToken(t *testing.T) {
	t.Parallel()

	// Every page data type this package declares. A view that aliases a type
	// not in this list fails below, so adding a screen means adding it here.
	declared := map[string]reflect.Type{
		"GroupsPageData":    reflect.TypeOf(permission.GroupsPageData{}),
		"GroupPageData":     reflect.TypeOf(permission.GroupPageData{}),
		"CataloguePageData": reflect.TypeOf(permission.CataloguePageData{}),
		"MatrixPageData":    reflect.TypeOf(permission.MatrixPageData{}),
		"SummaryPageData":   reflect.TypeOf(permission.SummaryPageData{}),
		"MembersPageData":   reflect.TypeOf(permission.MembersPageData{}),
		"MemberPageData":    reflect.TypeOf(permission.MemberPageData{}),
	}
	alias := regexp.MustCompile(`(?m)^type\s+\w+\s*=\s*permission\.(\w+)\s*$`)
	provider := reflect.TypeOf((*csrfProvider)(nil)).Elem()

	sources, err := filepath.Glob(filepath.Join(packageRoot(t), "resources", "publish", "*", "*.kyse.go"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("listing the published views: %v (%d found)", err, len(sources))
	}
	var withForm int
	for _, source := range sources {
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("reading %s: %v", source, err)
		}
		text := string(raw)
		if !strings.Contains(text, "@csrf") {
			continue
		}
		withForm++

		match := alias.FindStringSubmatch(text)
		if match == nil {
			t.Errorf("%s writes @csrf and does not alias the page data of this package, so nothing here can say it provides the token", source)
			continue
		}
		typ, ok := declared[match[1]]
		if !ok {
			t.Errorf("%s is drawn from %s, which this test does not know: add it to the list", source, match[1])
			continue
		}
		if !typ.Implements(provider) {
			t.Errorf("%s writes @csrf and %s has no CSRFToken() string, so drawing it is a 500", source, typ.Name())
		}
	}
	if withForm == 0 {
		t.Fatal("no published view writes @csrf, so this test would pass by having nothing to read")
	}
}
