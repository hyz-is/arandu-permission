package unit_test

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What this package proves about itself, before anybody installs it.
//
// `aru doctor` audits the application it is run inside. It walks that project's
// own tree, skips vendor/, reads the one arandu.mod.toml at its root, and
// refuses a directory that is not an application at all. It never loads a
// dependency. So nothing an installed package does is audited by the
// application that installed it: a package that reached a table without a
// Grant, took a tenant out of a request, or declared no capabilities and then
// opened a socket would pass every check the installer runs.
//
// The package is therefore the only place that check can happen, and this is
// it. These tests read this package's own Go files as syntax and hold four
// properties:
//
//  1. no Service method reaches a configured Model without a decision, and
//     every use case authorizes before its first reach;
//  2. the tenant comes from the Grant, and nothing a request carried is read as
//     one;
//  3. the Model remains tenant-scoped and CRUD does not grow a second data path;
//  4. what arandu.mod.toml declares is what the code does.
//
// The fifth is held where the routes exist rather than here, because a prefix
// arrives through configuration and syntax cannot follow it:
// TestNoRouteLandsInTheFrameworkNamespace, in tests/Feature/routes_test.go,
// registers the module and reads the table back.
//
// What these tests do not reach is worth as much as what they do. They read
// syntax: a call hidden behind an interface, a wrapper around the named Model
// entry point, and anything reached by reflection are all invisible to them. A
// green run means no such thing was found written down, not that none exists.
// What is absolute is what the compiler holds alongside them -- a Model terminal
// with no Grant in its call does not compile.

// buildable reports whether the compiler ever reads this file.
//
// It asks the build constraint rather than the file name, so a file excluded by
// a tag is left out of the audit whatever it is called: what the compiler never
// builds is not part of what the package does, and reporting it as such would
// be reporting a capability nobody can reach.
func buildable(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, comment := range group.List {
			expression, err := constraint.Parse(comment.Text)
			if err != nil {
				continue
			}
			// No tag set, which is what the gates run with.
			if !expression.Eval(func(string) bool { return false }) {
				return false
			}
		}
	}
	return true
}

// linked reports whether an application that installs this package compiles
// this file into its binary.
//
// A command is not linked. `go get` of a library never pulls in the main
// package beside it, and `go build` of the application never reaches it, so
// what a command does is not a capability anybody who installs this agreed to.
// Auditing one would make the manifest declare a capability that no running
// application has -- which is the same defect as declaring one nothing uses,
// pointed the other way.
//
// The package carries no command today, and the rule is here rather than in the
// commit that adds one: a capability audit that grows a hole the first time
// somebody needs it is an audit whose answer depends on who ran it.
//
// Every rule in this file reads what the installer links, and this is where
// that is decided once.
func linked(file *ast.File) bool { return file.Name.Name != "main" }

// auditedFiles is every Go file an application that installs this package
// compiles into its binary.
func auditedFiles(t *testing.T) []parsedGoFile {
	t.Helper()

	out := []parsedGoFile{}
	for _, source := range productionGoFiles(t, packageRoot(t)) {
		if buildable(source.file) && linked(source.file) {
			out = append(out, source)
		}
	}
	// An audit with nothing to read passes, and a test that passes by finding
	// nothing is the failure mode of every rule below.
	if len(out) == 0 {
		t.Fatal("no buildable Go file was found, so everything below would pass by having nothing to read")
	}
	return out
}

// selectorName is the trailing name of a selector, or the name of an
// identifier.
func selectorName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.Ident:
		return expression.Name
	}
	return ""
}

// calledName is the trailing name of whatever a call names, so a call can be
// recognised without resolving what it is called on.
func calledName(call *ast.CallExpr) string {
	return selectorName(call.Fun)
}

// firstCallTo is where a body first calls something by this name.
func firstCallTo(body *ast.BlockStmt, name string) token.Pos {
	found := token.NoPos
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || calledName(call) != name {
			return true
		}
		if found == token.NoPos || call.Pos() < found {
			found = call.Pos()
		}
		return true
	})
	return found
}

// TestNoServiceMethodReachesTheModelWithoutADecision holds the mandatory path
// on every use case rather than on the three that exist today.
//
// A nil-database denial test proves the closed path. This syntax audit is its
// twin for an allowed path: model construction is visible in the method body,
// and moving it above Authorize fails even when no terminal is executed.
//
// A method reaches a model legitimately in one of two shapes, and both are
// checked here rather than one being trusted:
//
//   - it authorizes first, which is every use case;
//   - it takes the Grant somebody else's decision produced, which is every
//     helper the use cases share. A Grant cannot be built by writing a struct
//     literal, so a parameter of that type is proof a policy already answered.
//
// A method that does neither is a path to a customer's rows that nothing
// decided about.
//
// What this does not reach is worth as much as what it does. It reads syntax: a
// call hidden behind an interface and anything reached by reflection are
// invisible to it. What is absolute is what the compiler holds alongside it --
// a model terminal with no Grant in its call does not compile.
func TestNoServiceMethodReachesTheModelWithoutADecision(t *testing.T) {
	t.Parallel()

	useCases, reaching := 0, 0
	for _, source := range auditedFiles(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || receiverType(function) != "PermissionService" {
				continue
			}

			decided := firstCallTo(function.Body, "Authorize")
			reach := firstModelReach(function.Body)
			issued := takesAGrant(function)

			// A method that takes a context is a use case. One that does not is
			// an accessor over what the service was built with, and there is
			// nothing for it to authorize.
			if takesAContext(function) {
				useCases++
				if decided == token.NoPos && !issued {
					t.Errorf("%s: %s neither calls security.Authorize nor takes the Grant one produced, so no Policy decided what it does",
						source.path, function.Name.Name)
				}
			}

			if reach == token.NoPos {
				continue
			}
			reaching++
			if decided == token.NoPos && !issued {
				t.Errorf("%s: %s reaches a configured Model with no decision behind it",
					source.path, function.Name.Name)
				continue
			}
			if decided != token.NoPos && decided > reach && !issued {
				t.Errorf("%s: %s reaches the Model before security.Authorize",
					source.path, function.Name.Name)
			}
		}
	}
	if useCases == 0 {
		t.Fatal("no PermissionService use case was found, so this test proved nothing")
	}
	if reaching == 0 {
		t.Fatal("no PermissionService method reaches the configured Model, so this audit found no data boundary to order")
	}
}

// TestEveryExportedUseCaseAuthorizesItself keeps the entry points from
// delegating the decision to a helper.
//
// The distinction matters because the rule above admits a method holding a
// Grant, and an exported method that took one would be an exported method whose
// caller decided -- and the caller of an exported method is somebody else's
// code.
func TestEveryExportedUseCaseAuthorizesItself(t *testing.T) {
	t.Parallel()

	audited := 0
	for _, source := range auditedFiles(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || receiverType(function) != "PermissionService" ||
				!function.Name.IsExported() || !takesAContext(function) {
				continue
			}
			audited++

			if takesAGrant(function) {
				t.Errorf("%s: %s takes a Grant, so whoever calls it decided rather than this package",
					source.path, function.Name.Name)
			}
			if firstCallTo(function.Body, "Authorize") == token.NoPos {
				t.Errorf("%s: %s never calls security.Authorize", source.path, function.Name.Name)
			}
		}
	}
	if audited == 0 {
		t.Fatal("no exported PermissionService use case was found, so this test proved nothing")
	}
}

// takesAContext reports whether a method takes a context, which is what tells a
// use case from an accessor.
func takesAContext(function *ast.FuncDecl) bool {
	return takesParameterNamed(function, "Context")
}

// takesAGrant reports whether a method takes an already-issued Grant.
func takesAGrant(function *ast.FuncDecl) bool {
	return takesParameterNamed(function, "Grant")
}

func takesParameterNamed(function *ast.FuncDecl, name string) bool {
	if function.Type.Params == nil {
		return false
	}
	for _, parameter := range function.Type.Params.List {
		if namedType(parameter.Type) == name {
			return true
		}
	}
	return false
}

// modelConstructors are the configured Model entry points of this package.
//
// Naming them rather than matching a shape is deliberate: the audit has to fail
// when a new table is reached without a decision, and a new table means a new
// name here. A list that is one name short is exactly the failure this file
// exists to catch, so a test below reads the constructors back out of model.go
// and fails when one is missing from this map.
var modelConstructors = map[string]bool{
	"Groups": true, "GroupActions": true, "GroupUsers": true,
	"UserActions": true, "Versions": true,
}

// firstModelReach is where a method first constructs a configured Model or
// calls a promoted write terminal. The constructor itself counts: moving only
// its construction before Authorize is the mutation this audit exists to
// reject.
func firstModelReach(body *ast.BlockStmt) token.Pos {
	terminals := map[string]bool{
		"Save": true, "Delete": true, "Restore": true, "Touch": true,
		"Get": true, "First": true, "Count": true, "Exists": true,
		"Value": true, "Pluck": true, "Insert": true, "Update": true,
	}
	found := token.NoPos
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := calledName(call)
		if !modelConstructors[name] && !terminals[name] {
			return true
		}
		if found == token.NoPos || call.Pos() < found {
			found = call.Pos()
		}
		return true
	})
	return found
}

// TestEveryModelConstructorIsAudited keeps the list above from going stale.
//
// A table added to model.go and left out of the list is a table the ordering
// audit does not watch, and nothing else would say so.
func TestEveryModelConstructorIsAudited(t *testing.T) {
	t.Parallel()

	found := 0
	for _, source := range auditedFiles(t) {
		if source.path != "model.go" {
			continue
		}
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !function.Name.IsExported() {
				continue
			}
			if function.Type.Results == nil || len(function.Type.Results.List) != 1 ||
				namedType(function.Type.Results.List[0].Type) != "Model" {
				continue
			}
			found++
			if !modelConstructors[function.Name.Name] {
				t.Errorf("model.go declares %s and audit_test.go does not watch it: add it to modelConstructors",
					function.Name.Name)
			}
		}
	}
	if found != len(modelConstructors) {
		t.Fatalf("model.go declares %d Model constructors and %d are watched", found, len(modelConstructors))
	}
}

// requestAccessors are the ways a value that arrived with the request is read.
// A tenant taken through any of them is a tenant the caller chose.
var requestAccessors = map[string]bool{
	"Param":         true,
	"Query":         true,
	"Input":         true,
	"FormValue":     true,
	"PostFormValue": true,
	"PathValue":     true,
	"Cookie":        true,
	"Get":           true,
}

// TestNoTenantIsReadOutOfTheRequest is the rule with no exception in it. A
// tenant the client can name is a client who chooses whose rows they read, and
// every other check in the package passes while it happens.
func TestNoTenantIsReadOutOfTheRequest(t *testing.T) {
	t.Parallel()

	for _, source := range auditedFiles(t) {
		ast.Inspect(source.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !requestAccessors[selector.Sel.Name] {
				return true
			}
			for _, argument := range call.Args {
				literal, ok := argument.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				if strings.Contains(strings.ToLower(literal.Value), "tenant") {
					t.Errorf("%s: %s(%s) reads a tenant out of the request; it comes from the Grant, which came from the session",
						source.path, selector.Sel.Name, literal.Value)
				}
			}
			return true
		})
	}
}

// TestTheServiceWritesTenantOnlyFromTheGrant holds the write half of tenant
// isolation. The proposed value is policy input; the value persisted after
// Authorize must take TenantID directly from data.Tenant(g).
func TestTheServiceWritesTenantOnlyFromTheGrant(t *testing.T) {
	t.Parallel()

	audited := 0
	for _, source := range auditedFiles(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || receiverType(function) != "PermissionService" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				assignment, ok := node.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, target := range assignment.Lhs {
					if i >= len(assignment.Rhs) || selectorName(target) != "TenantID" {
						continue
					}
					audited++
					call, ok := assignment.Rhs[i].(*ast.CallExpr)
					if !ok || calledName(call) != "Tenant" {
						t.Errorf("%s: %s writes TenantID from something other than data.Tenant(g)",
							source.path, function.Name.Name)
					}
				}
				return true
			})
		}
	}
	if audited == 0 {
		t.Fatal("no Service tenant assignment was found, so this test proved nothing")
	}
}

// capabilities is what arandu.mod.toml declares, and what the code does, under
// the same four names.
type capabilities struct {
	network    bool
	filesystem bool
	exec       bool
	migrations bool
}

// TestTheDeclaredCapabilitiesAreWhatTheCodeDoes is the audit the installer
// cannot run. `aru doctor` compares an application's manifest against the
// application's code; the manifest of a package it installed is never opened,
// so this comparison exists here or nowhere.
//
// Both directions fail. Using more than was declared is the one that costs
// somebody something, and the doctor calls it an error too. Declaring more than
// is used is milder -- a warning, there -- and it fails here because `go test`
// has one outcome and because asking for what is not needed is how a
// declaration stops being worth reading.
func TestTheDeclaredCapabilitiesAreWhatTheCodeDoes(t *testing.T) {
	t.Parallel()

	declared := declaredCapabilities(t)
	used := usedCapabilities(auditedFiles(t))

	for _, capability := range []struct {
		name        string
		declared    bool
		used        bool
		consequence string
	}{
		{"network", declared.network, used.network,
			"the code makes calls that leave the process, and whoever installs it agreed to a package that does not"},
		{"filesystem", declared.filesystem, used.filesystem,
			"the code reads or writes files outside the database, which nothing in its API shows"},
		{"exec", declared.exec, used.exec,
			"the code runs another program, which is the widest capability there is"},
		{"migrations", declared.migrations, used.migrations,
			"the code owns tables, and an installer who did not expect that will not know to migrate before deploying"},
	} {
		switch {
		case capability.used && !capability.declared:
			t.Errorf("arandu.mod.toml declares %s = false and the code uses it: %s. Declare it, or remove what needs it",
				capability.name, capability.consequence)
		case capability.declared && !capability.used:
			t.Errorf("arandu.mod.toml declares %s = true and nothing uses it: set %s = false, or the declaration stops being worth reading",
				capability.name, capability.name)
		}
	}
}

// declaredCapabilities reads the [permissions] block.
//
// By hand, because the alternative is a third dependency in a module that is
// compiled into other people's builds, and four booleans do not earn one.
func declaredCapabilities(t *testing.T) capabilities {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(packageRoot(t), "arandu.mod.toml"))
	if err != nil {
		t.Fatalf("reading arandu.mod.toml: %v", err)
	}

	var declared capabilities
	seen := map[string]bool{}
	inside := false

	for _, line := range strings.Split(string(body), "\n") {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inside = line == "[permissions]"
			continue
		}
		if !inside {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		seen[key] = true
		switch key {
		case "network":
			declared.network = value == "true"
		case "filesystem":
			declared.filesystem = value == "true"
		case "exec":
			declared.exec = value == "true"
		case "migrations":
			declared.migrations = value == "true"
		}
	}

	for _, required := range []string{"network", "filesystem", "exec", "migrations"} {
		if !seen[required] {
			t.Errorf("arandu.mod.toml leaves %s undeclared under [permissions]; silence reads as false to whoever installs this, and a capability nobody declared is one nobody agreed to",
				required)
		}
	}
	return declared
}

// usedCapabilities is what the code actually does, by syntax.
//
// It reads calls rather than imports, for the reason the manifest gives: this
// package imports net/http for a method constant and a request type, and an
// import says nothing about whether anything leaves the process.
func usedCapabilities(files []parsedGoFile) capabilities {
	var used capabilities

	for _, source := range files {
		if strings.Contains(source.path, "migrations/") {
			used.migrations = true
		}
		for _, imported := range source.file.Imports {
			switch strings.Trim(imported.Path.Value, `"`) {
			case "os/exec":
				used.exec = true
			case "net/smtp", "net/rpc":
				used.network = true
			case "io/ioutil":
				used.filesystem = true
			}
		}

		standard := standardImports(source.file)
		ast.Inspect(source.file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				switch name := qualifiedName(node, standard); {
				case name == "http.Get", name == "http.Post", name == "http.Head",
					name == "http.PostForm", name == "http.NewRequest",
					name == "http.NewRequestWithContext",
					name == "net.Dial", name == "net.DialTimeout",
					strings.HasSuffix(name, ".Do"):
					used.network = true

				case name == "os.Open", name == "os.OpenFile", name == "os.Create",
					name == "os.ReadFile", name == "os.WriteFile", name == "os.Remove",
					name == "os.RemoveAll", name == "os.Mkdir", name == "os.MkdirAll",
					name == "os.Rename", name == "os.ReadDir",
					name == "filepath.Walk", name == "filepath.WalkDir":
					used.filesystem = true

				case name == "exec.Command", name == "exec.CommandContext", name == "syscall.Exec":
					used.exec = true
				}
			case *ast.FuncDecl:
				if node.Recv != nil && node.Name.Name == "Migrations" {
					used.migrations = true
				}
			}
			return true
		})
	}
	return used
}

// standardImports maps the local name of every standard library import to the
// package's own name, so an aliased import is recognised by what it is.
//
// Only the standard library, because the names matched above are its names. A
// third-party package whose last element is "http" is not net/http, and reading
// it as one reports a router helper as an outbound call.
func standardImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if first, _, _ := strings.Cut(path, "/"); strings.Contains(first, ".") {
			continue
		}

		name := path
		if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
			name = path[slash+1:]
		}
		local := name
		if imported.Name != nil {
			local = imported.Name.Name
		}
		out[local] = name
	}
	return out
}

// qualifiedName is a call as "package.Function" where the receiver is an
// imported package, and ".Method" where it is a value.
func qualifiedName(call *ast.CallExpr, standard map[string]string) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		if identifier, ok := call.Fun.(*ast.Ident); ok {
			return identifier.Name
		}
		return ""
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return "." + selector.Sel.Name
	}
	if name, imported := standard[owner.Name]; imported {
		return name + "." + selector.Sel.Name
	}
	return owner.Name + "." + selector.Sel.Name
}
