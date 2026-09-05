package permission

import (
	"embed"
	"io/fs"
	"sort"
	"strings"

	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/translation"
)

// The sentences this package ships.
//
// They are embedded and not published, and that is the same decision the views
// make in the other direction. A view is meant to be edited, so the project takes
// ownership of the file; a sentence is meant to keep up with the code that
// produces it, so a copy in every project is a copy that goes stale -- and it
// goes stale silently, one release at a time, until a screen says something the
// code stopped doing.
//
// An application that wants different words writes them in its own catalogue
// under the same keys. Its translator is asked first, and this is the floor
// under it.
//
//go:embed resources/lang
var langSources embed.FS

// The catalogue this package's keys live in, and the locale every line is
// written in before anything else.
const (
	// TranslationGroup is the catalogue group, so every key of this package
	// reads "permission." followed by what it names.
	TranslationGroup = "permission"
	// FallbackLocale is the locale a line is read from when the one asked for
	// has none.
	FallbackLocale = "en"
)

// The prefixes of the two key families that are derived from an identifier
// rather than written out.
//
// They are constants because the derivation happens in two places -- the lookup
// and the catalogue file -- and a prefix spelled twice is a label that resolves
// in one of them.
const (
	// ActionKeyPrefix turns an action into the key its label is read from, so
	// "invoice.delete" is read from "permission.action.invoice.delete".
	ActionKeyPrefix = TranslationGroup + ".action."
	// DomainKeyPrefix does the same for the part of an action before its first
	// dot.
	DomainKeyPrefix = TranslationGroup + ".domain."
)

// shipped is the embedded catalogue, read once at load.
//
// A failure here is a broken binary rather than a condition to recover from: the
// files are compiled in, so either they parse or the build that produced this
// program was wrong.
var shipped, shippedLocales = mustShipped()

func mustShipped() (*translation.Translator, []string) {
	root, err := fs.Sub(langSources, "resources/lang")
	if err != nil {
		panic("permission: the embedded catalogue has no lang directory: " + err.Error())
	}
	loader, err := translation.NewFileLoader(root, ".")
	if err != nil {
		panic("permission: the embedded catalogue does not parse: " + err.Error())
	}

	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		panic("permission: the embedded catalogue cannot be listed: " + err.Error())
	}
	locales := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			locales = append(locales, entry.Name())
		}
	}
	sort.Strings(locales)
	if len(locales) == 0 {
		panic("permission: the embedded catalogue holds no locale, so every screen would draw its keys")
	}

	return translation.New(loader, FallbackLocale, FallbackLocale), locales
}

// Locales are the locales this package ships sentences for, sorted.
//
// An application reads it to know which of its own it has to write, and a test
// reads it to check that none of them is missing a line the others have.
func Locales() []string { return append([]string(nil), shippedLocales...) }

// Lines are the sentences this package ships for one locale, keyed the way a
// translator asks for them, and nil for a locale it does not ship.
//
// It is exported so an application can load them into its own translator, look
// at what there is to override, or check a locale of its own against them.
func Lines(locale string) translation.Lines {
	lines := shipped.GetLoader().Load(locale, TranslationGroup, "*")
	if len(lines) == 0 {
		return nil
	}
	out := make(translation.Lines, len(lines))
	for item, line := range lines {
		out[TranslationGroup+"."+item] = line
	}
	return out
}

// Labels is what one screen reads, resolved for the locale the request asked
// for.
//
// It is filled by the handler and handed to the view, which is the whole of why
// there is no helper a template calls for itself. A view that reached for a
// translator would be a view that can be rendered outside a request, in whatever
// locale the process happened to be left in -- and the failure looks like one
// person's page coming back in somebody else's language.
//
// The zero value answers every key with the key, which is what a screen drawn by
// something that forgot to fill it in should look like: obviously unfinished,
// rather than quietly English.
type Labels struct {
	// Locale is what these were resolved for.
	Locale string

	// module is what resolves a key. It is a field rather than a package-level
	// lookup so that two modules wired in one process, with different
	// translators, do not answer each other's screens.
	module *Module
}

// T is the sentence at this key, which is written without the group: T("field.name")
// reads "permission.field.name".
//
// A key with no line anywhere comes back as itself. It is the one answer that
// cannot be mistaken for a translation, which is what somebody staring at a
// screen needs in order to find the missing line.
func (l Labels) T(key string) string {
	if l.module == nil {
		return key
	}
	return l.module.line(l.Locale, TranslationGroup+"."+key)
}

// Action is what a person reads in place of an action.
//
// The identifier is what is stored and what every decision is taken on; this is
// derived from it when the page is drawn. That is what lets a label be rewritten
// in any language, or corrected in this one, without a row changing and without
// anything that referenced the action breaking.
//
// An action with no line reads as itself, which is the right answer rather than
// a fallback: the identifier is what the application's own developers named it,
// and it is what they will search for.
func (l Labels) Action(action security.Action) string {
	if l.module == nil {
		return string(action)
	}
	return l.module.lineOr(l.Locale, ActionKeyPrefix+string(action), string(action))
}

// Domain is what a person reads in place of the part of an action before its
// first dot. It falls back to the domain itself, for the reason Action does.
func (l Labels) Domain(name string) string {
	if l.module == nil {
		return name
	}
	return l.module.lineOr(l.Locale, DomainKeyPrefix+name, name)
}

// line reads a key, asking the application's translator first.
//
// Two translators and one rule: the application's wins wherever it has a line,
// and the shipped catalogue is the floor under it. That is one override path,
// and it is the path an application already has -- rather than a second one this
// package would invent, in which the same sentence could be written in two files
// and only one of them read.
func (m *Module) line(locale, key string) string {
	if m.cfg.Translator != nil && m.cfg.Translator.Has(locale, key) {
		return m.cfg.Translator.Get(locale, key, nil)
	}
	return shipped.Get(locale, key, nil)
}

// lineOr reads a key and answers with the fallback when nothing has a line for
// it, rather than with the key.
func (m *Module) lineOr(locale, key, fallback string) string {
	if m.cfg.Translator != nil && m.cfg.Translator.Has(locale, key) {
		return m.cfg.Translator.Get(locale, key, nil)
	}
	if shipped.Has(locale, key) || shipped.Has(FallbackLocale, key) {
		return shipped.Get(locale, key, nil)
	}
	return fallback
}

// Labels are the sentences of this panel in one locale.
//
// It is exported because it is what a handler outside this package needs in
// order to draw the same words -- an application that wraps a screen of its own
// around these, or replaces one, reads its labels from here rather than writing
// a second copy of them.
//
// An empty locale is answered in the shipped one. A request that went through no
// negotiation middleware carries none, and a panel in the wrong language is
// still a panel somebody can read, where a refusal would not be.
func (m *Module) Labels(locale string) Labels {
	if strings.TrimSpace(locale) == "" {
		locale = FallbackLocale
	}
	return Labels{Locale: locale, module: m}
}
