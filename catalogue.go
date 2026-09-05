package permission

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/arandu-io/framework/security"
)

// Catalogue is the set of actions a group may carry.
//
// It is not a table and there is no way to add to it from a screen. A row that
// attached a group to an action nothing decides with would be a permission
// somebody was told they had, and no policy would ever read it -- so the write
// path checks every action against this set and refuses what is not in it.
//
// The application builds it from what its own code declares and hands it to
// New. Nothing here discovers actions: a catalogue assembled at boot would hold
// only the modules that happened to be linked, and the screen administers
// permissions for the whole application.
//
// The zero value is empty and refuses every action, which is the safe direction:
// a module wired without a catalogue grants nothing rather than everything.
type Catalogue struct {
	// actions is the sorted, deduplicated set, kept for the reader that wants
	// it flat.
	actions []security.Action
	// member answers Has without a scan, which matters because every write
	// checks every action it is given.
	member map[security.Action]bool
	// domains is the same set grouped by what comes before the first dot,
	// sorted by domain and then by action.
	domains []Domain
}

// Domain is one group of the catalogue: the part of an action before its first
// dot, and every action that starts with it.
//
// It is what a screen draws a section from. The name is the identifier and not
// a label: what a person reads is resolved when the page is rendered, so it can
// be written in their language without anything in the database changing.
type Domain struct {
	// Name is the part before the first dot, shared by every action below.
	Name string
	// Actions are the actions of this domain, sorted.
	Actions []security.Action
}

// NewCatalogue returns the catalogue of these actions, or the reason it cannot
// be one.
//
// It refuses an action that is not in module.verb form, because that shape is
// what the grouping reads and an action with no domain would sit in a section
// with no name. It refuses the empty set, because a module whose catalogue is
// empty can never grant anything and would say so one screen at a time instead
// of once, here.
//
// Repeats are not an error. The same action reaching it twice is what happens
// when an application splices several lists together, and refusing that would
// make the caller deduplicate a set this function returns deduplicated anyway.
func NewCatalogue(actions ...security.Action) (Catalogue, error) {
	if len(actions) == 0 {
		return Catalogue{}, fmt.Errorf("permission: the catalogue is empty: a group can only carry an action some policy reads, so an empty catalogue is a screen that can grant nothing")
	}

	member := make(map[security.Action]bool, len(actions))
	byDomain := map[string][]security.Action{}
	for _, action := range actions {
		domain, err := domainOf(action)
		if err != nil {
			return Catalogue{}, err
		}
		if member[action] {
			continue
		}
		member[action] = true
		byDomain[domain] = append(byDomain[domain], action)
	}

	flat := make([]security.Action, 0, len(member))
	for action := range member {
		flat = append(flat, action)
	}
	sort.Slice(flat, func(i, j int) bool { return flat[i] < flat[j] })

	names := make([]string, 0, len(byDomain))
	for name := range byDomain {
		names = append(names, name)
	}
	sort.Strings(names)

	domains := make([]Domain, 0, len(names))
	for _, name := range names {
		owned := byDomain[name]
		sort.Slice(owned, func(i, j int) bool { return owned[i] < owned[j] })
		domains = append(domains, Domain{Name: name, Actions: owned})
	}

	return Catalogue{actions: flat, member: member, domains: domains}, nil
}

// Has reports whether the action may be attached to a group.
func (c Catalogue) Has(action security.Action) bool { return c.member[action] }

// Len is how many distinct actions the catalogue holds.
func (c Catalogue) Len() int { return len(c.actions) }

// All returns every action, sorted. The slice is a copy, so a caller that sorts
// or truncates it does not change the catalogue every write is checked against.
func (c Catalogue) All() []security.Action {
	return append([]security.Action(nil), c.actions...)
}

// Domains returns the catalogue grouped by domain, sorted. The slices are
// copies, for the reason All returns one.
func (c Catalogue) Domains() []Domain {
	out := make([]Domain, 0, len(c.domains))
	for _, domain := range c.domains {
		out = append(out, Domain{
			Name:    domain.Name,
			Actions: append([]security.Action(nil), domain.Actions...),
		})
	}
	return out
}

// domainOf is the part of an action before its first dot, and the check that
// there is one.
//
// The shape is held here rather than trusted, because everything downstream
// reads it: the grouping on the screen, the section a new action lands in, and
// the sort that decides the order both appear in.
func domainOf(action security.Action) (string, error) {
	text := string(action)
	if text == "" {
		return "", fmt.Errorf("permission: the catalogue holds an empty action")
	}
	if strings.TrimSpace(text) != text || strings.ContainsAny(text, " \t\r\n") {
		return "", fmt.Errorf("permission: the action %q carries whitespace, and an action is an identifier", text)
	}
	if strings.ToLower(text) != text {
		return "", fmt.Errorf("permission: the action %q is not lowercase, and two spellings of one action are two permissions nobody can tell apart", text)
	}
	domain, verb, found := strings.Cut(text, ".")
	if !found || domain == "" || verb == "" {
		return "", fmt.Errorf("permission: the action %q is not in module.verb form, so it belongs to no domain and the catalogue has no section to draw it in", text)
	}
	return domain, nil
}

// The syntax of a selector.
//
// They are named here because three things read them: the parser, the matcher,
// and the sentence a refusal is written with. A delimiter spelled twice is a
// selector that parses one way and is explained another.
const (
	// SelectorSeparator divides a selector into parts, the way a dot divides an
	// action.
	SelectorSeparator = "."
	// SelectorAlternative divides one part into the alternatives it accepts, so
	// that "invoice.create,update" names two actions rather than one with a
	// comma in it.
	SelectorAlternative = ","
	// SelectorWildcard is the part that matches whatever is in that position.
	SelectorWildcard = "*"
)

// ErrNoMatch is returned when a selector names no action the catalogue holds.
//
// The catalogue is finite and known before the selector is read, so a selector
// that matches nothing is a mistake rather than an empty result. Answering it as
// an empty list would let "invoce.*" take every invoice permission off a group
// and report success.
var ErrNoMatch = errors.New("permission: the selector matches no action in the catalogue")

// Match returns every action of the catalogue the selector names, sorted.
//
// A selector is an action pattern: parts divided by dots, alternatives inside a
// part divided by commas, and a part that is a single asterisk matching whatever
// stands in that position.
//
//	invoice.create           one action, named outright
//	invoice.create,update    two actions of one module
//	invoice.*                every action of the invoice module
//	*.delete                 the delete of every module
//	invoice                  every action below invoice, however deep
//
// A selector that runs out of parts covers everything below where it stopped,
// which is why "invoice" names the whole module -- and why a wildcard in the
// last position does the same: "invoice.*" names "invoice.line.create" as well
// as "invoice.create". A selector longer than an action does not match it, so
// "invoice.create.line" names nothing while "invoice.create" is all there is.
//
// What comes back is always actions the catalogue holds. A selector cannot bring
// one into being, cannot widen past the set the application declared, and cannot
// be stored: it is read where a person writes it and what is written down is the
// actions it resolved to. That is the difference between naming many permissions
// at once and keeping a pattern somewhere a decision would later have to match
// against -- the second is a decision moved out of Go and into a string, and
// nothing here does it.
//
// A selector that matches nothing is ErrNoMatch rather than an empty list.
func (c Catalogue) Match(selector string) ([]security.Action, error) {
	pattern, err := parseSelector(selector)
	if err != nil {
		return nil, err
	}

	var out []security.Action
	for _, action := range c.actions {
		if pattern.implies(strings.Split(string(action), SelectorSeparator)) {
			out = append(out, action)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoMatch, selector)
	}
	return out, nil
}

// selectorPart is one position of a selector: the literals it accepts, and
// whether it accepts anything.
type selectorPart struct {
	// literals are the alternatives written at this position, without the
	// wildcard.
	literals map[string]bool
	// wildcard is whether the asterisk was one of them.
	wildcard bool
}

// selectorPattern is a parsed selector, one entry per part.
type selectorPattern []selectorPart

// implies reports whether the pattern names this action, given as its parts.
//
// The pattern running out is a match and not a failure: it is what makes
// "invoice" name every action of the module, and it is the same rule that makes
// a trailing wildcard cover whatever depth an action happens to have. The action
// running out first is not: a pattern with more parts than the action names
// something more specific than anything the catalogue holds.
//
// A part that carries both a literal and the wildcard tries the literal first.
// The two can only disagree about how much of the action is left to match, and
// the more specific reading is the one a person writing it meant.
func (p selectorPattern) implies(action []string) bool {
	if len(p) == 0 {
		return true
	}
	if len(action) == 0 {
		return false
	}
	head, rest := p[0], p[1:]
	if head.literals[action[0]] && rest.implies(action[1:]) {
		return true
	}
	return head.wildcard && rest.implies(action[1:])
}

// parseSelector reads a selector, or the reason it is not one.
//
// It refuses an empty part and an empty alternative rather than ignoring them,
// because "invoice..create" and "invoice.create," are both somebody's typing
// slip and both would otherwise silently name something other than what was
// written.
func parseSelector(selector string) (selectorPattern, error) {
	if selector == "" {
		return nil, fmt.Errorf("permission: the selector is empty")
	}
	if strings.TrimSpace(selector) != selector || strings.ContainsAny(selector, " \t\r\n") {
		return nil, fmt.Errorf("permission: the selector %q carries whitespace, and a selector names identifiers", selector)
	}
	if strings.ToLower(selector) != selector {
		return nil, fmt.Errorf("permission: the selector %q is not lowercase, and an action is", selector)
	}

	parts := strings.Split(selector, SelectorSeparator)
	pattern := make(selectorPattern, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("permission: the selector %q has an empty part, so it names a position with nothing in it", selector)
		}
		entry := selectorPart{literals: map[string]bool{}}
		for _, alternative := range strings.Split(part, SelectorAlternative) {
			switch alternative {
			case "":
				return nil, fmt.Errorf("permission: the selector %q has an empty alternative, so one of the names it lists is missing", selector)
			case SelectorWildcard:
				entry.wildcard = true
			default:
				entry.literals[alternative] = true
			}
		}
		pattern = append(pattern, entry)
	}
	return pattern, nil
}
