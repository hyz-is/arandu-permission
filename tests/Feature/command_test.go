package feature_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/console"
	hdatabase "github.com/arandu-io/hesape/database"

	permission "github.com/hyz-is/arandu-permission"
)

// What the terminal can do, and what it cannot.
//
// A command here is not a way around the panel. It calls the same use case a
// screen calls, so the same policy answers -- and it acts as a person whose
// permissions are read out of the database rather than asserted by whoever typed
// the command. Somebody with a shell can already read the rows; what they must
// not get for free is the ability to write one nobody authorized.
//
// The one thing that has to work without a person is the first group, because
// until it exists there is nobody to act as. That door is guarded by the state:
// it shuts the moment a group exists, and never opens again.

// terminal is a module over an empty, migrated database, with its commands
// registered, and the buffers they write to.
type terminal struct {
	module *permission.Module
	app    *console.Application
	out    *bytes.Buffer
	err    *bytes.Buffer
}

// run executes one command and returns everything it printed.
func (c *terminal) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c.out.Reset()
	c.err.Reset()
	err := c.app.Handle(context.Background(), args)
	return c.out.String() + c.err.String(), err
}

func console_(t *testing.T) *terminal {
	t.Helper()

	pool, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "permission.db"))
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	module, err := build(t, settings(), data.Wrap(pool, data.DialectSQLite))
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}
	connection := hdatabase.NewConnection(pool, "main", "", map[string]any{"driver": "sqlite"})
	for _, migration := range module.Migrations() {
		if err := migration.Up(context.Background(), hdatabase.ForMigrations(connection)); err != nil {
			t.Fatalf("applying %s: %v", migration.GetName(), err)
		}
	}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := console.NewApplication(out, errOut, strings.NewReader("")).Add(module.Commands()...)
	return &terminal{module: module, app: app, out: out, err: errOut}
}

// TestTheFirstGroupComesFromTheTerminalAndOnlyOnce is the whole of the
// bootstrap story: the door opens once, because there is nobody to authorize as,
// and the state shuts it.
func TestTheFirstGroupComesFromTheTerminalAndOnlyOnce(t *testing.T) {
	t.Parallel()

	c := console_(t)
	printed, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice")
	if err != nil {
		t.Fatalf("the first group: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "administrators") {
		t.Errorf("the first group printed %q", printed)
	}

	printed, err = c.run(t, "permission:create-group", "second", "Second", "--member=bob")
	if !errors.Is(err, permission.ErrBootstrapped) {
		t.Fatalf("a second bootstrap = %v, want ErrBootstrapped\n%s", err, printed)
	}
}

// TestACommandActsAsSomebodyAndCanDoNoMoreThanTheyCould is the property that
// makes the terminal safe to have.
func TestACommandActsAsSomebodyAndCanDoNoMoreThanTheyCould(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}

	// alice is in the first group and may administer permissions. She does not
	// hold invoice.delete, which the first group was not given.
	if _, err := c.run(t, "permission:create-group", "editors", "Editors", "--as=alice"); err != nil {
		t.Fatalf("alice creating a group: %v", err)
	}

	_, err := c.run(t, "permission:grant", "editors", "invoice.delete", "--as=alice")
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("alice granting what she does not hold = %v, want ErrForbidden", err)
	}

	// bob is nobody. Every policy refuses him, which is what a subject carrying
	// nothing has to mean.
	_, err = c.run(t, "permission:create-group", "anything", "Anything", "--as=bob")
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("a stranger creating a group = %v, want ErrForbidden", err)
	}

	// And a command run as nobody at all is refused before it reaches a policy.
	if _, err := c.run(t, "permission:grant", "editors", "permission.view"); err == nil {
		t.Error("a write command with no --as was allowed")
	}
}

// TestASelectorFromTheTerminalNamesManyPermissionsAtOnce is the reason the
// commands exist for anybody administering more than a handful.
func TestASelectorFromTheTerminalNamesManyPermissionsAtOnce(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}
	if _, err := c.run(t, "permission:create-group", "readers", "Readers", "--as=alice"); err != nil {
		t.Fatalf("creating: %v", err)
	}

	printed, err := c.run(t, "permission:grant", "readers", "permission.*", "--as=alice")
	if err != nil {
		t.Fatalf("granting by selector: %v\n%s", err, printed)
	}
	for _, want := range []string{"permission.view", "permission.list", "permission.grant"} {
		if !strings.Contains(printed, want) {
			t.Errorf("the selector did not report %s\n%s", want, printed)
		}
	}

	printed, err = c.run(t, "permission:revoke", "readers", "permission.grant,revoke", "--as=alice")
	if err != nil {
		t.Fatalf("revoking by selector: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "removed:") {
		t.Errorf("the revocation reported %q", printed)
	}

	// A selector nobody declared is refused rather than silently doing nothing.
	if _, err := c.run(t, "permission:grant", "readers", "invoce.*", "--as=alice"); !errors.Is(err, permission.ErrNoMatch) {
		t.Errorf("a mistyped selector = %v, want ErrNoMatch", err)
	}
}

// TestTheTerminalGivesAndTakesBackADirectPermission walks the path that has no
// group in it, including its refusal.
func TestTheTerminalGivesAndTakesBackADirectPermission(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators",
		"--member=alice", "--permission=invoice.delete"); err != nil {
		t.Fatalf("the first group: %v", err)
	}

	printed, err := c.run(t, "permission:give", "carol", "invoice.delete", "--as=alice")
	if err != nil {
		t.Fatalf("giving: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "added: invoice.delete") {
		t.Errorf("giving printed %q", printed)
	}

	// Not to herself, from a terminal any more than from a screen.
	if _, err := c.run(t, "permission:give", "alice", "invoice.delete", "--as=alice"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("giving oneself a permission from the terminal = %v, want ErrForbidden", err)
	}

	printed, err = c.run(t, "permission:give", "carol", "invoice.delete", "--as=alice", "--revoke")
	if err != nil {
		t.Fatalf("taking back: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "removed: invoice.delete") {
		t.Errorf("taking back printed %q", printed)
	}
}

// TestTheTerminalMovesPeopleInAndOutOfAGroup covers the membership pair.
func TestTheTerminalMovesPeopleInAndOutOfAGroup(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}
	if _, err := c.run(t, "permission:create-group", "editors", "Editors", "--as=alice"); err != nil {
		t.Fatalf("creating: %v", err)
	}

	printed, err := c.run(t, "permission:assign", "editors", "dan", "erin", "--as=alice")
	if err != nil {
		t.Fatalf("assigning: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "dan") || !strings.Contains(printed, "erin") {
		t.Errorf("assigning printed %q", printed)
	}

	printed, err = c.run(t, "permission:unassign", "editors", "dan", "--as=alice")
	if err != nil {
		t.Fatalf("unassigning: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "removed: dan") {
		t.Errorf("unassigning printed %q", printed)
	}

	// Nobody may put themselves in a group, whatever they type it at.
	if _, err := c.run(t, "permission:assign", "editors", "alice", "--as=alice"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("assigning oneself from the terminal = %v, want ErrForbidden", err)
	}
}

// TestShowPrintsTheGroupsAgainstTheCatalogue is the reference's own command,
// and the one an operator reads before changing anything.
func TestShowPrintsTheGroupsAgainstTheCatalogue(t *testing.T) {
	t.Parallel()

	c := console_(t)

	// Reading is where a permission system leaks, so the listing takes a
	// subject like every write does.
	if _, err := c.run(t, "permission:show", "--tenant=acme"); err == nil {
		t.Error("the listing ran with no --as")
	}
	if _, err := c.run(t, "permission:show", "--as=nobody"); !errors.Is(err, security.ErrForbidden) {
		t.Error("a stranger read the listing")
	}

	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}
	printed, err := c.run(t, "permission:show", "--as=alice")
	if err != nil {
		t.Fatalf("showing: %v", err)
	}
	for _, want := range []string{"administrators", "(system)", "permission.view", "group"} {
		if !strings.Contains(printed, want) {
			t.Errorf("the table does not mention %s\n%s", want, printed)
		}
	}
}

// TestTheResetCommandMovesTheToken holds what the reference's cache-reset
// becomes when there is no shared cache to clear.
func TestTheResetCommandMovesTheToken(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}

	printed, err := c.run(t, "permission:cache-reset", "--as=alice")
	if err != nil {
		t.Fatalf("resetting: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "moved from") {
		t.Errorf("the reset printed %q", printed)
	}

	// A stranger may not move it: it writes a row, and something that writes is
	// not reachable by whoever may only look.
	if _, err := c.run(t, "permission:cache-reset", "--as=nobody"); !errors.Is(err, security.ErrForbidden) {
		t.Errorf("a stranger resetting = %v, want ErrForbidden", err)
	}
}

// TestEveryCommandIsNamedAndDescribed keeps the listing usable, which is the
// only documentation a command has where somebody is reading it.
func TestEveryCommandIsNamedAndDescribed(t *testing.T) {
	t.Parallel()

	module, err := build(t, settings(), data.Wrap(nil, data.DialectSQLite))
	if err != nil {
		t.Fatalf("building the module: %v", err)
	}

	commands := module.Commands()
	if len(commands) == 0 {
		t.Fatal("the module registers no command, so this test proved nothing")
	}
	seen := map[string]bool{}
	for _, command := range commands {
		name, _, _ := strings.Cut(command.Signature, " ")
		if !strings.HasPrefix(name, "permission:") {
			t.Errorf("%s is not in this module's namespace", name)
		}
		if seen[name] {
			t.Errorf("%s is registered twice", name)
		}
		seen[name] = true
		if strings.TrimSpace(command.Description) == "" {
			t.Errorf("%s has no description", name)
		}
		if command.Run == nil {
			t.Errorf("%s has nothing to run", name)
		}
	}
	for _, named := range []string{
		permission.CommandShow, permission.CommandCreateGroup, permission.CommandGrant,
		permission.CommandRevoke, permission.CommandAssign, permission.CommandUnassign,
		permission.CommandGive, permission.CommandForget,
	} {
		if !seen[named] {
			t.Errorf("%s is a declared name and no command answers to it", named)
		}
	}
}

// TestAnActionNobodyHoldsIsGrantedWithNoAs is the second door of the terminal.
// An application gained invoice.delete after the first group was made, so
// nobody holds it and nobody could ever grant it; the operator starts it, and it
// is the same write a person's grant would have been.
func TestAnActionNobodyHoldsIsGrantedWithNoAs(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}

	// With --as the ordinary rule still holds: alice does not hold it.
	if _, err := c.run(t, "permission:grant", "administrators", "invoice.delete", "--as=alice"); !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("alice granting what nobody holds = %v, want ErrForbidden", err)
	}

	printed, err := c.run(t, "permission:grant", "administrators", "invoice.delete")
	if err != nil {
		t.Fatalf("starting an action nobody holds: %v\n%s", err, printed)
	}
	for _, want := range []string{"bootstrapped", "invoice.delete", "administrators"} {
		if !strings.Contains(printed, want) {
			t.Errorf("the bootstrap printed %q, missing %q", printed, want)
		}
	}

	// It is a real write: alice now holds it and may hand it on, by the ordinary
	// rules.
	if _, err := c.run(t, "permission:create-group", "editors", "Editors", "--as=alice"); err != nil {
		t.Fatalf("alice creating a group: %v", err)
	}
	if printed, err := c.run(t, "permission:grant", "editors", "invoice.delete", "--as=alice"); err != nil {
		t.Fatalf("alice granting what she now holds: %v\n%s", err, printed)
	}

	// Asked again, the group already carries it and nothing changes.
	printed, err = c.run(t, "permission:grant", "administrators", "invoice.delete")
	if err != nil {
		t.Fatalf("asking again: %v\n%s", err, printed)
	}
	if !strings.Contains(printed, "nothing changed") {
		t.Errorf("asking again printed %q", printed)
	}
}

// TestAnActionSomebodyHoldsIsRefusedWithNoAs is what shuts the second door: the
// moment anybody holds the action, there is somebody to act as.
func TestAnActionSomebodyHoldsIsRefusedWithNoAs(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice", "--member=dave"); err != nil {
		t.Fatalf("the first group: %v", err)
	}
	if _, err := c.run(t, "permission:create-group", "readers", "Readers", "--as=alice"); err != nil {
		t.Fatalf("creating: %v", err)
	}

	// Held by a group.
	if _, err := c.run(t, "permission:grant", "readers", "permission.grant"); !errors.Is(err, permission.ErrActionHeld) {
		t.Errorf("an action a group holds = %v, want ErrActionHeld", err)
	}
	// A selector that covers one action somebody holds writes nothing.
	if _, err := c.run(t, "permission:grant", "readers", "permission.*"); !errors.Is(err, permission.ErrActionHeld) {
		t.Errorf("a selector over held actions = %v, want ErrActionHeld", err)
	}

	// Held directly, with no group carrying it.
	if _, err := c.run(t, "permission:create-group", "writers", "Writers", "--as=alice"); err != nil {
		t.Fatalf("creating: %v", err)
	}
	if _, err := c.run(t, "permission:grant", "writers", "invoice.delete"); err != nil {
		t.Fatalf("starting the action: %v", err)
	}
	if _, err := c.run(t, "permission:give", "bob", "invoice.delete", "--as=alice"); err == nil {
		t.Fatal("alice gave what she does not hold")
	}
	if _, err := c.run(t, "permission:assign", "writers", "alice", "--as=dave"); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	if _, err := c.run(t, "permission:give", "bob", "invoice.delete", "--as=alice"); err != nil {
		t.Fatalf("giving it directly: %v", err)
	}
	if _, err := c.run(t, "permission:revoke", "writers", "invoice.delete", "--as=alice"); err != nil {
		t.Fatalf("taking it off the group: %v", err)
	}
	if _, err := c.run(t, "permission:grant", "readers", "invoice.delete"); !errors.Is(err, permission.ErrActionHeld) {
		t.Errorf("an action held directly = %v, want ErrActionHeld", err)
	}
}

// TestTheNoAsGrantRefusesWhatTheCatalogueDoesNotHold keeps the door to the
// catalogue: an action nobody declared is not an action nobody holds.
func TestTheNoAsGrantRefusesWhatTheCatalogueDoesNotHold(t *testing.T) {
	t.Parallel()

	c := console_(t)
	if _, err := c.run(t, "permission:create-group", "administrators", "Administrators", "--member=alice"); err != nil {
		t.Fatalf("the first group: %v", err)
	}

	if _, err := c.run(t, "permission:grant", "administrators", "invoice.explode"); !errors.Is(err, permission.ErrNoMatch) {
		t.Errorf("an action outside the catalogue = %v, want ErrNoMatch", err)
	}
	// Revoking has no such door: it still needs somebody to act as.
	if _, err := c.run(t, "permission:revoke", "administrators", "permission.view"); err == nil {
		t.Error("a revoke with no --as was allowed")
	}
	// Neither does an unknown group, and no row was written for it.
	if _, err := c.run(t, "permission:grant", "ghosts", "invoice.delete"); !errors.Is(err, permission.ErrNotFound) {
		t.Errorf("an unknown group = %v, want ErrNotFound", err)
	}

	// The service refuses an action the catalogue lacks as well, for a caller
	// that does not go through a selector.
	if _, err := c.module.Service().BootstrapActions(context.Background(), "acme", "administrators", []security.Action{"invoice.explode"}); !errors.Is(err, permission.ErrUnknownAction) {
		t.Errorf("the service on an unknown action = %v, want ErrUnknownAction", err)
	}
}
