package permission

import (
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
