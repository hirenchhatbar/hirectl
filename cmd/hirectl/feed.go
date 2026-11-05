package main

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Feed is one saved job-post search and the boards it runs on.
type Feed struct {
	ID       string   `json:"id"` // unique slug, e.g. go-remote; names the feed in `hirectl run --feeds` and jobs.json
	Title    string   `json:"title"`
	Skills   []string `json:"skills,omitempty"`
	Location string   `json:"location,omitempty"`
	Boards   []string `json:"boards"` // integration:company, e.g. greenhouse:stripe
	Enabled  bool     `json:"enabled"`
}

var feedKind = kind[Feed]{
	name:   "Feed",
	file:   "feeds.json",
	fields: []string{"Title", "ID", "Skills", "Location", "Boards"},
	hints:  []string{"Go remote", "go-remote (fills from title)", "Golang, PHP, Laravel", "Remote", "greenhouse:stripe, lever:netflix"},
	blank:  Feed{Enabled: true},
	parse: func(v []string) (Feed, error) {
		f := Feed{
			Title:    strings.TrimSpace(v[0]),
			ID:       strings.TrimSpace(v[1]),
			Skills:   parseList(v[2]),
			Location: strings.TrimSpace(v[3]),
			Boards:   parseList(v[4]),
			Enabled:  true,
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
	return []string{f.Title, f.ID, strings.Join(f.Skills, ", "), f.Location, strings.Join(f.Boards, ", ")}
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
	if len(f.Boards) == 0 {
		return errors.New("at least one board is required, e.g. greenhouse:stripe")
	}
	for _, b := range f.Boards {
		name, _, ok := splitBoard(b)
		if !ok {
			return fmt.Errorf("board %q: want source:name", b)
		}
		if integrations[name] == nil {
			return fmt.Errorf("board %q: unknown integration, have %s", b, strings.Join(integrationNames(), ", "))
		}
	}
	return nil
}

// matches reports whether j fits f: f's location within j's, and any of f's skills as a word in j's title
// or text, or among its techs under any spelling ("Golang" finds Go).
func (f Feed) matches(j Job) bool {
	// shortcut: plain substring location match, upgrade to normalised places if feeds need city/country aliases
	if !strings.Contains(strings.ToLower(j.Location), strings.ToLower(f.Location)) {
		return false
	}
	text := j.Title + "\n" + j.text
	return len(f.Skills) == 0 || slices.ContainsFunc(f.Skills, func(s string) bool {
		return hasWord(text, s) || slices.Contains(j.Tech, techName(s))
	})
}

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
