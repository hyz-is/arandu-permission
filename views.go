package permission

import (
	"embed"
	"io/fs"
	"strings"

	"github.com/arandu-io/framework/foundation"
)

// The view sources this package hands to the project that installs it.
//
// They are embedded rather than read off disk because what a module publishes
// is a tree it carries, not a directory it points at: the program that writes
// the files is the application's own binary, running somewhere this repository
// does not exist as files. Whatever is handed over has to already be inside the
// program doing the handing.
//
// The tree is kept apart from the address it lands at, and the publication
// carries both. It has to: the destination has a segment named vendor, and the
// go command drops every path with one when it packs a module -- so a source
// tree living at its own destination is present in this repository and absent
// from what anybody downloads, and this embed then matches nothing in their
// build.
//
//go:embed all:resources/publish/*/*.kyse.go
var viewSources embed.FS

// Where a view is kept, where it is written, and what it is called.
//
// The first two are different directories on purpose, and the publication
// carries both so a file moves from one to the other untouched. viewSuffix is
// the extension: it ends in .go so the build tag on the first line keeps the
// compiler out of a file that is markup below the package clause.
const (
	// viewRoot is the directory every path in the archive starts with. It is
	// deliberately not the destination: go mod drops every file whose path
	// contains a segment named vendor, at any depth, so a source tree under
	// resources/views/vendor is in the repository and missing from the
	// published module.
	viewRoot = "resources/publish"
	// viewPrefix is where the same files are written in a project, which is the
	// address an application looks for a package's views at. It is what a view
	// name is derived from, and what Boot holds the archive to.
	viewPrefix = "resources/views/vendor/permission"
	viewSuffix = ".kyse.go"
)

// compiledRoot is where the view compiler writes the Go it produces, mirroring
// the tree of the sources. It is build output: gitignored, rebuilt on demand,
// and never edited.
const compiledRoot = "storage/framework/views"

// Publishes declares the files this package offers, each at the path it takes
// relative to the root of the project.
//
// One tree, under one tag. A publication may carry a page, a component, a
// configuration file, a migration, a catalogue of sentences or an asset, and
// this package offers the first of those and nothing else. Each absence is a
// decision rather than an omission:
//
//   - configuration is the Config struct New is handed, checked by the compiler
//     and validated before the module exists. A file copied into the project
//     beside it would be a second place to say the same thing, and only one of
//     the two could be the one the code reads.
//   - a stylesheet, a script or any other asset is registered with the view
//     layer and served from an address derived from its own bytes. Copying one
//     into the project would put a second copy of those bytes under a second
//     address, and a page can only reference one of them.
//   - translations are overridden by writing the lines the application wants
//     into its own vendor tree, which the catalogue loader already reads. A
//     copy of every line this package ships is a copy that goes stale, and it
//     goes stale without saying so.
//   - migrations are declared and collected, never copied. A copy in the
//     project's own migration directory is found by the runner as well, so one
//     schema change applies twice under two names.
//
// What is left is the markup, and it is here for the reason the others are not:
// it is the one thing the project is expected to edit. A package cannot know
// what a screen should say in a product it has never seen.
func (m *Module) Publishes() []foundation.Publication {
	// From and To are what make the archive and the destination two different
	// paths. They have to be: go mod publishes no file whose path carries a
	// segment named vendor, and the destination -- the address an application
	// looks for a package's views at -- carries one.
	return []foundation.Publication{{
		Tag:   foundation.PublishView,
		Files: viewSources,
		From:  viewRoot,
		To:    viewPrefix,
	}}
}

// PublishedPaths are the files the archive offers, each at the path it is
// written at relative to the root of the application, sorted.
func PublishedPaths() []string { return append([]string(nil), publishedPaths...) }

// ViewNames are the names the published views are rendered by, sorted.
//
// The name is the path a view is written at, under the directory an application
// keeps its views in, with its separators turned into dots -- which is what the
// view compiler writes into the registration call. It is derived from the same
// archive the publication carries, so a view that was renamed cannot keep an
// old name here.
func ViewNames() []string { return append([]string(nil), viewNames...) }

// ViewPackages are the directories the compiled views land in, each relative to
// the root of the application, sorted and without repeats.
//
// Importing one is what puts its views in the binary: a compiled view calls the
// registry from init(), and a package nothing imports is not linked at all. The
// import is named rather than written into bootstrap/app.go, because one line
// somebody reads beats a file that changed while they were not looking.
func ViewPackages() []string {
	var out []string
	seen := make(map[string]bool)
	for _, path := range publishedPaths {
		// The paths are already destinations, so what is cut off here is the
		// directory a project writes under, not the one the archive keeps.
		dir := compiledRoot + strings.TrimPrefix(path[:strings.LastIndexByte(path, '/')], "resources/views")
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

// The archive, read once at load.
//
// A failure here is a broken binary rather than a condition to recover from:
// the files are compiled in, so either they are all there or the build that
// produced this program was wrong.
var publishedPaths, viewNames = readArchive()

func readArchive() (paths, names []string) {
	err := fs.WalkDir(viewSources, viewRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, viewSuffix) {
			return nil
		}
		paths = append(paths, publishedPath(path))
		names = append(names, viewName(path))
		return nil
	})
	if err != nil {
		panic("permission: reading the embedded views: " + err.Error())
	}
	if len(paths) == 0 {
		panic("permission: the embedded view directory holds no view, so every name this package renders would be missing and nothing would say so")
	}
	return paths, names
}

// publishedPath turns an archive path into the path the file is written at.
//
//	resources/publish/groups/index.kyse.go -> resources/views/vendor/permission/groups/index.kyse.go
func publishedPath(path string) string {
	return viewPrefix + strings.TrimPrefix(path, viewRoot)
}

// viewName turns an archive path into the name the view is registered under.
//
// The name comes from where the file is written and not from where it is kept,
// so that the two directories cannot produce two spellings of one view.
//
//	resources/publish/groups/index.kyse.go -> vendor.permission.groups.index
func viewName(path string) string {
	name := strings.TrimPrefix(publishedPath(path), "resources/views/")
	name = strings.TrimSuffix(name, viewSuffix)
	return strings.ReplaceAll(name, "/", ".")
}
