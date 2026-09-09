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
const currentVersion = "0.3.0"

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

// releasedChangelog is CHANGELOG.md with the [Unreleased] section removed.
//
// Everything a tag shipped has to be under a version heading. The section above
// the first one is where work waits, and a release that forgets to move it is a
// published version whose own changelog calls its contents unreleased -- which
// is what v0.2.1 and v0.2.2 of this package did: both went out with no entry
// in either release file.
func releasedChangelog(t *testing.T) string {
	t.Helper()
	body := readReleaseFile(t, packageRoot(t), "CHANGELOG.md")
	first := regexp.MustCompile(`(?m)^## \[[0-9]`).FindStringIndex(body)
	if first == nil {
		t.Fatal("CHANGELOG.md has no version heading")
	}
	return body[first[0]:]
}

// TestEveryActionIsNamedInAReleasedChangelogEntry is the gate that catches a
// tag pushed without filing what it shipped.
//
// An action is added in the same change that adds the capability behind it, so
// an action still sitting in [Unreleased] means the version that introduced it
// went out undocumented. It is the cheapest signal of that, and it needs no git
// history to read.
func TestEveryActionIsNamedInAReleasedChangelogEntry(t *testing.T) {
	policy := readReleaseFile(t, packageRoot(t), "policy.go")
	released := releasedChangelog(t)

	names := regexp.MustCompile(`(?m)^\t([A-Z][A-Za-z]*) security\.Action = `).FindAllStringSubmatch(policy, -1)
	if len(names) == 0 {
		t.Fatal("policy.go declares no actions")
	}
	for _, name := range names {
		if !strings.Contains(released, "`"+name[1]+"`") {
			t.Errorf("no released changelog entry names %s", name[1])
		}
	}
}

// TestEveryMigrationIsNamedInAReleasedChangelogEntry holds the same for schema.
//
// A migration is the one thing an operator has to run before a version serves,
// so a version that shipped one and did not say so is a version that fails at
// the first request against a column that is not there.
func TestEveryMigrationIsNamedInAReleasedChangelogEntry(t *testing.T) {
	module := readReleaseFile(t, packageRoot(t), "module.go")
	released := releasedChangelog(t)

	ids := regexp.MustCompile(`"([0-9]{8}_[0-9]{4}_[a-z_]+)"`).FindAllStringSubmatch(module, -1)
	if len(ids) == 0 {
		t.Fatal("module.go declares no migrations")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id[1]] {
			continue
		}
		seen[id[1]] = true
		if !strings.Contains(released, id[1]) {
			t.Errorf("no released changelog entry names migration %s", id[1])
		}
	}
}

// TestEveryChangelogVersionHasUpgradeNotes keeps the two files describing the
// same set of releases.
//
// They drifted once already: two releases were tagged with no entry in either
// of them.
func TestEveryChangelogVersionHasUpgradeNotes(t *testing.T) {
	root := packageRoot(t)
	changelog := readReleaseFile(t, root, "CHANGELOG.md")
	upgrade := readReleaseFile(t, root, "UPGRADE.md")

	inChangelog := regexp.MustCompile(`(?m)^## \[([0-9]+\.[0-9]+\.[0-9]+)\] - `).FindAllStringSubmatch(changelog, -1)
	inUpgrade := regexp.MustCompile(`(?m)^## v([0-9]+\.[0-9]+\.[0-9]+)$`).FindAllStringSubmatch(upgrade, -1)
	if len(inChangelog) == 0 || len(inUpgrade) == 0 {
		t.Fatal("one of the two release files has no version heading")
	}

	versions := func(matches [][]string) map[string]bool {
		out := map[string]bool{}
		for _, m := range matches {
			out[m[1]] = true
		}
		return out
	}
	logged, upgraded := versions(inChangelog), versions(inUpgrade)
	for v := range logged {
		if !upgraded[v] {
			t.Errorf("CHANGELOG.md has %s and UPGRADE.md has no notes for it", v)
		}
	}
	for v := range upgraded {
		if !logged[v] {
			t.Errorf("UPGRADE.md has notes for %s and CHANGELOG.md has no entry for it", v)
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
