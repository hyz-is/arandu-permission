package unit_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// What a release of this package has to be true about itself.
//
// None of it is about behaviour. It is about the three places a version number
// is written down and the one page that says what a break costs whoever
// installed the last one -- which is the part nobody notices is missing until
// they are the one upgrading.

// currentVersion is the release this working tree is preparing.
//
// It is written here and compared against the changelog rather than read out of
// it: a test that took the number from the file it checks would pass on a file
// that lost its heading.
const currentVersion = "0.1.0"

func TestTheManifestFrameworkFloorMatchesGoMod(t *testing.T) {
	root := packageRoot(t)
	goMod := readReleaseFile(t, root, "go.mod")
	manifest := readReleaseFile(t, root, "arandu.mod.toml")

	required := captureReleaseValue(t, goMod,
		`(?m)^\s*github\.com/arandu-io/framework v([0-9]+\.[0-9]+)\.[0-9]+\s*$`,
		"Framework version in go.mod")
	declared := captureReleaseValue(t, manifest,
		`(?m)^framework = ">= ([0-9]+\.[0-9]+)"$`,
		"Framework floor in arandu.mod.toml")

	if declared != required {
		t.Fatalf("manifest Framework floor = %s, want %s from go.mod", declared, required)
	}
}

func TestTheReleaseSkillUsesTheManifestFrameworkFloor(t *testing.T) {
	root := packageRoot(t)
	manifest := readReleaseFile(t, root, "arandu.mod.toml")
	skill := readReleaseFile(t, root, ".agents/skills/permission-release/SKILL.md")
	declared := captureReleaseValue(t, manifest,
		`(?m)^framework = ">= ([0-9]+\.[0-9]+)"$`,
		"Framework floor in arandu.mod.toml")

	want := `framework = ">= ` + declared + `"`
	if !strings.Contains(skill, want) {
		t.Fatalf("release skill does not teach manifest floor %q", want)
	}
}

// TestTheChangelogAndTheUpgradeGuideAgreeOnTheVersion keeps the two pages a
// person reads before upgrading from naming different releases.
func TestTheChangelogAndTheUpgradeGuideAgreeOnTheVersion(t *testing.T) {
	root := packageRoot(t)
	changelog := readReleaseFile(t, root, "CHANGELOG.md")
	upgrade := readReleaseFile(t, root, "UPGRADE.md")

	if got := strings.Count(changelog, "## ["+currentVersion+"] - "); got != 1 {
		t.Fatalf("v%s changelog headings = %d, want exactly one pre-versioned entry", currentVersion, got)
	}
	if !strings.Contains(upgrade, "## v"+currentVersion) {
		t.Fatalf("UPGRADE.md has no v%s entry", currentVersion)
	}
}

// TestTheFirstReleaseNamesWhatCannotBeFixedLater holds the two facts that are
// not recoverable by editing a call site: a catalogue that leaves out this
// package's own actions, and the group nothing but a seed can create.
func TestTheFirstReleaseNamesWhatCannotBeFixedLater(t *testing.T) {
	upgrade := readReleaseFile(t, packageRoot(t), "UPGRADE.md")

	for _, want := range []string{
		"Config.Actions",
		"permission.Actions()",
		"CreateGroupRequest",
		"System",
	} {
		if !strings.Contains(upgrade, want) {
			t.Errorf("UPGRADE.md does not name %s", want)
		}
	}
}

func TestCIGuardsIncompatibleAPIChanges(t *testing.T) {
	ci := readReleaseFile(t, packageRoot(t), ".github/workflows/ci.yml")
	required := []string{
		"fetch-depth: 0",
		"go vet -tags example ./example",
		"go run -tags example ./example",
		"name: api diff against the last release",
		`modpath=$(GOWORK=off go list -m -f '{{.Path}}')`,
		`git show "${tag}:go.mod"`,
		"git worktree add",
		"golang.org/x/exp/cmd/apidiff@latest -m -w",
		"-m -incompatible",
		`git diff --quiet "$latest" -- UPGRADE.md`,
		`added=$(git diff "$latest" -- UPGRADE.md`,
	}
	for _, want := range required {
		if !strings.Contains(ci, want) {
			t.Errorf("CI does not contain the API compatibility gate %q", want)
		}
	}
}

func TestTheReleasePublishesThePreVersionedChangelogEntryOnce(t *testing.T) {
	release := readReleaseFile(t, packageRoot(t), ".github/workflows/release.yml")

	required := []string{
		`tags: ["v*"]`,
		"contents: write",
		`version="${TAG#v}"`,
		`heading="## [$version] - "`,
		`gh release create "$TAG"`,
		"--verify-tag",
		`--notes-file "$notes"`,
	}
	for _, want := range required {
		if !strings.Contains(release, want) {
			t.Errorf("release workflow does not contain %q", want)
		}
	}
	if got := strings.Count(release, `gh release create "$TAG"`); got != 1 {
		t.Errorf("release creation commands = %d, want exactly one", got)
	}
	for _, forbidden := range []string{
		"Write it into the changelog",
		"git commit",
		"git push",
		"git tag",
		"gh release delete",
		"CHANGELOG.next",
	} {
		if strings.Contains(release, forbidden) {
			t.Errorf("release workflow mutates the pre-versioned changelog through %q", forbidden)
		}
	}
}

func readReleaseFile(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}

func captureReleaseValue(t *testing.T, body, pattern, label string) string {
	t.Helper()
	match := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("%s is missing", label)
	}
	return match[1]
}
