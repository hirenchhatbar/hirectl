package main

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"
)

// Feed is one saved job-post search. `hirectl run` fetches its top boards by matching jobs in suggestions.json.
type Feed struct {
	ID        string   `json:"id"` // unique slug, e.g. go-remote; names the feed in `hirectl run --feeds` and jobs/<id>/
	Title     string   `json:"title"`
	Skills    []string `json:"skills,omitempty"`
	Locations []string `json:"locations,omitempty"` // any one matches, e.g. "Remote India"
	Sources   []string `json:"sources,omitempty"`   // integrations whose boards run picks from, e.g. greenhouse; none means all
	Enabled   bool     `json:"enabled"`
}

var feedKind = kind[Feed]{
	name:   "Feed",
	file:   "feeds.json",
	fields: []string{"Title", "ID", "Skills", "Locations", "Sources"},
	hints:  []string{"Go remote", "go-remote (fills from title)", "Golang, PHP, Laravel", "Remote Worldwide, Remote India, India", "all when blank, e.g. greenhouse, lever"},
	blank:  func() Feed { return Feed{Enabled: true} },
	parse: func(v []string) (Feed, error) {
		f := Feed{
			Title:     strings.TrimSpace(v[0]),
			ID:        strings.TrimSpace(v[1]),
			Skills:    parseList(v[2]),
			Locations: parseList(v[3]),
			Sources:   parseList(v[4]),
			Enabled:   true,
		}
		if f.ID == "" {
			f.ID = slug(f.Title)
		}
		return f, f.validate()
	},
	derive:     func(v []string) { v[1] = slug(v[0]) },
	deriveHint: "ID from title",
}

func (f Feed) label() string { return f.Title }
func (f Feed) key() string   { return f.ID }
func (f Feed) on() bool      { return f.Enabled }

func (f Feed) withOn(on bool) Feed {
	f.Enabled = on
	return f
}

// values returns f's fields in feedKind.fields order, as the form edits them.
func (f Feed) values() []string {
	return []string{f.Title, f.ID, strings.Join(f.Skills, ", "), strings.Join(f.Locations, ", "), strings.Join(f.Sources, ", ")}
}

var (
	idRe     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	nonIDRun = regexp.MustCompile(`[^a-z0-9]+`)
)

// slug turns a title into an ID: "Go remote (EU)" becomes "go-remote-eu".
func slug(title string) string {
	return strings.Trim(nonIDRun.ReplaceAllString(strings.ToLower(title), "-"), "-")
}

func (f Feed) validate() error {
	if f.Title == "" {
		return errors.New("title is required")
	}
	if !idRe.MatchString(f.ID) {
		return fmt.Errorf("id %q: want lowercase letters, digits and dashes, e.g. go-remote", f.ID)
	}
	return checkSources(f.Sources)
}

// matches reports whether j fits f: any of f's locations in j's, and any of f's skills in j (see score).
// No locations or skills match all.
func (f Feed) matches(j Job) bool {
	if len(f.Locations) > 0 && !slices.ContainsFunc(f.Locations, func(l string) bool { return inLocation(j.Location, l) }) {
		return false
	}
	return len(f.Skills) == 0 || f.score(j) > 0
}

// score rates how much j is about f's skills, summed over them: 10 for a skill in the title, plus 1 per
// time it is in the description, up to 5, else 1 for it among the techs discover tagged. A skill counts
// under any spelling ("Golang" for Go) as a word not followed by "-" ("Go-to-Market" is not Go); in the
// title in any case, in the description case-sensitive for a tech, so "go" in prose is not Go.
func (f Feed) score(j Job) int {
	n := 0
	for _, s := range f.Skills {
		name := techName(s)
		spellings, flags := []string{name}, "(?i)"
		if i := slices.IndexFunc(techs, func(t []string) bool { return t[0] == name }); i >= 0 {
			spellings, flags = techs[i], ""
		}
		if slices.ContainsFunc(spellings, func(w string) bool { return wordRe("(?i)", w, techEnd).MatchString(j.Title) }) {
			n += 10
		}
		hits := 0
		for _, w := range spellings {
			hits += len(wordRe(flags, w, techEnd).FindAllStringIndex(j.text, 5))
		}
		switch {
		case hits > 0:
			n += min(hits, 5)
		case slices.Contains(j.Tech, name):
			n++
		}
	}
	return n
}

// inLocation reports whether every word of want is a word of loc, in any order, ignoring case and
// punctuation: "Remote India" is in "Bangalore, India - Remote", "India" is not in "Indiana".
// shortcut: words only, no place aliases; a feed lists cities itself, e.g. "India, Bangalore, Mumbai"
func inLocation(loc, want string) bool {
	return !slices.ContainsFunc(strings.FieldsFunc(want, isSep), func(w string) bool { return !hasWord(loc, w) })
}

func isSep(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }

// wordRes caches wordRe's patterns, as suggestions match every stored job.
var wordRes sync.Map

// hasWord reports whether word appears in text, case-insensitive, not inside another word ("Go" misses "Google").
func hasWord(text, word string) bool { return wordRe("(?i)", word, `($|[^\pL\pN])`).MatchString(text) }

// wordRe matches word after a non-word, with regexp flags such as "(?i)", and the given end pattern.
func wordRe(flags, word, end string) *regexp.Regexp {
	key := flags + "\x00" + word + "\x00" + end
	re, ok := wordRes.Load(key)
	if !ok {
		re, _ = wordRes.LoadOrStore(key, regexp.MustCompile(flags+`(^|[^\pL\pN])`+regexp.QuoteMeta(word)+end))
	}
	return re.(*regexp.Regexp)
}

// parseList splits a comma-separated list, dropping blanks.
func parseList(s string) []string {
	var items []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			items = append(items, p)
		}
	}
	return items
}
