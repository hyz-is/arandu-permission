package unit_test

import (
	"strings"
	"testing"

	security "github.com/arandu-io/framework/security"
	permission "github.com/hyz-is/arandu-permission"
)

// A slug that means two things is two authorizations wearing one name.
//
// The guide says to splice: append(myapp.Actions(), permission.Actions()...).
// So a repeat of one of this package's actions means the application declared
// the same slug -- and this package's permission.create opens the screen that
// administers groups, while an application's governs whatever its own policy
// governs. They collapsed into one entry, silently, and granting either granted
// both: a person given the screen was given the application's write.

func TestASlugDeclaredByBothIsRefused(t *testing.T) {
	t.Parallel()

	// What an application with its own permissions table would write, following
	// the guide to the letter.
	application := []security.Action{
		"permission.view", "permission.create", "permission.update", "permission.delete",
	}

	_, err := permission.NewCatalogue(append(application, permission.Actions()...)...)
	if err == nil {
		t.Fatal("a slug declared by the application and by this package was accepted: " +
			"granting one grants the other, and nothing said so")
	}
	if !strings.Contains(err.Error(), "permission.view") &&
		!strings.Contains(err.Error(), "permission.create") {
		t.Errorf("the refusal does not name the action that collided: %v", err)
	}
	if !strings.Contains(err.Error(), "Rename one of them") {
		t.Errorf("the refusal does not say what to do about it: %v", err)
	}
}

// TestTheApplicationsOwnRepeatIsStillCheap keeps the other half of the
// distinction. An application splicing several of its own lists repeats its own
// actions, and that is not two authorizations -- it is one, listed twice.
func TestTheApplicationsOwnRepeatIsStillCheap(t *testing.T) {
	t.Parallel()

	first := []security.Action{"invoice.view", "invoice.create"}
	second := []security.Action{"invoice.view", "report.read"}

	catalogue, err := permission.NewCatalogue(
		append(append(first, second...), permission.Actions()...)...)
	if err != nil {
		t.Fatalf("an application repeating its own action was refused: %v", err)
	}

	// Listed twice, carried once.
	var seen int
	for _, action := range catalogue.All() {
		if action == "invoice.view" {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("invoice.view is in the catalogue %d times, want 1", seen)
	}
}

// TestTheGuideSpliceStillWorks is the ordinary case, and it has to keep
// working: an application whose slugs do not collide splices and gets a
// catalogue.
func TestTheGuideSpliceStillWorks(t *testing.T) {
	t.Parallel()

	application := []security.Action{"invoice.view", "invoice.create", "user.view"}

	catalogue, err := permission.NewCatalogue(append(application, permission.Actions()...)...)
	if err != nil {
		t.Fatalf("the spliced catalogue the guide teaches was refused: %v", err)
	}
	for _, action := range permission.Actions() {
		if !catalogue.Has(action) {
			t.Errorf("this package's own %q is not in the catalogue, so its screen is unreachable", action)
		}
	}
	for _, action := range application {
		if !catalogue.Has(action) {
			t.Errorf("the application's %q is not in the catalogue", action)
		}
	}
}
