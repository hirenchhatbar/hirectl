package main

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestFuzzy(t *testing.T) {
	for _, c := range []struct {
		query, text string
		ok          bool
	}{
		{"", "anything", true},
		{"gorem", "Go Developer, Remote", true},
		{"go remote", "Senior Go Engineer greenhouse:acme Remote India", true},
		{"rust", "Go Developer", false},
		{"og", "Go", false}, // letters in order only
	} {
		if _, ok := fuzzy(strings.Fields(c.query), c.text); ok != c.ok {
			t.Errorf("fuzzy(%q, %q) ok = %v, want %v", c.query, c.text, ok, c.ok)
		}
	}
	tight, _ := fuzzy([]string{"go"}, "Go Developer")
	loose, _ := fuzzy([]string{"go"}, "Gateway Ops")
	if tight <= loose {
		t.Errorf("score of go in Go Developer %d <= in Gateway Ops %d, want letters in a row first", tight, loose)
	}
}

func TestJobsPane(t *testing.T) {
	day := 24 * time.Hour
	jobs := []Job{
		{Board: "lever:a", Title: "Go Developer", Location: "Remote India", URL: "https://a/1", Posted: time.Now().Add(-10 * day), Score: 10, Level: "3-4"},
		{Board: "greenhouse:b", Title: "PHP Engineer", Location: "Berlin, Germany", URL: "https://b/2", Posted: time.Now().Add(-2 * day), Score: 2, Level: "5-7"},
		{Board: "lever:c", Title: "Golang Lead", Location: "Remote", URL: "https://c/3", Score: 10},
	}
	k := feedKind
	saved := map[string]jobView{}
	k.jobs = func(f Feed) ([]Job, jobView, error) { return jobs, saved[f.ID], nil }
	k.saveView = func(f Feed, v jobView) error { saved[f.ID] = v; return nil }
	var m module = newListModel(k, "", []Feed{{ID: "go", Title: "Go", Enabled: true}})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	var opened string
	openURL = func(u string) error { opened = u; return nil }
	press := func(keys ...string) {
		for _, s := range keys {
			msg := tea.KeyPressMsg{Text: s}
			switch s {
			case "esc":
				msg = tea.KeyPressMsg{Code: tea.KeyEscape}
			case "enter":
				msg = tea.KeyPressMsg{Code: tea.KeyEnter}
			default:
				msg.Code = []rune(s)[0]
			}
			m, _ = m.Update(msg)
		}
	}
	titles := func() string {
		var got []string
		for _, j := range m.(listModel[Feed]).jobs.shown {
			got = append(got, j.Title)
		}
		return strings.Join(got, ", ")
	}
	if !strings.Contains(ansi.Strip(m.View()), " 3/3│") {
		t.Fatalf("sidebar lacks the job count:\n%s", m.View())
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if lm := m.(listModel[Feed]); lm.mode != modeJobs {
		t.Fatalf("after →: mode %v, want jobs", lm.mode)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if lm := m.(listModel[Feed]); lm.mode != modeLogo {
		t.Fatalf("after ← in jobs: mode %v, want the sidebar", lm.mode)
	}
	press("o")
	if lm := m.(listModel[Feed]); lm.mode != modeJobs || titles() != "PHP Engineer, Go Developer, Golang Lead" {
		t.Fatalf("after o: mode %v, jobs %q; want newest first, undated last", lm.mode, titles())
	}
	press("r", "y", "y") // score 3+, level 3-4: PHP Engineer drops, unknown level stays
	if titles() != "Go Developer, Golang Lead" {
		t.Fatalf("score 3+, level 3-4: %q", titles())
	}
	press("S", "S", "S", "S") // sort level: unknown last
	if got := m.(listModel[Feed]).jobs.settings(); got.Score != 3 || got.Level != "3-4" || got.Sort != "level" {
		t.Fatalf("settings %+v", got)
	}
	press("r", "r", "y", "y", "y", "S") // back to any score, any level, newest
	press("l", "I", "n", "d", "i", "a", "enter", "enter")
	if titles() != "Go Developer" || opened != "https://a/1" {
		t.Fatalf("location India: %q, opened %q", titles(), opened)
	}
	press("esc", "o") // reopened with location India applied
	if titles() != "Go Developer" {
		t.Fatalf("reopened: %q, want saved location applied", titles())
	}
	press("l")
	for range "India" {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	press("enter")
	press("/", "g", "o", "l", "a", "n", "g", "enter")
	if titles() != "Golang Lead" {
		t.Fatalf("search golang: %q", titles())
	}
	press("s", " ", "enter") // check greenhouse, first source
	if titles() != "" || !strings.Contains(m.View(), "no jobs match") {
		t.Fatalf("source greenhouse with search golang: %q", titles())
	}
	press("s", " ", "j", " ", "enter", "d", "d") // greenhouse off, lever on, posted within 7d: undated stays
	if titles() != "Golang Lead" {
		t.Fatalf("lever, 7d: %q", titles())
	}
	press("s", "j", " ", "esc") // esc undoes unchecking lever
	if got := m.(listModel[Feed]).jobs.checked; !slices.Equal(got, []string{"lever"}) {
		t.Fatalf("checked after esc in checklist: %q, want lever", got)
	}
	press("/", "x", "esc") // esc undoes the typing, keeping golang
	if titles() != "Golang Lead" {
		t.Fatalf("after esc in search: %q", titles())
	}
	press("esc")
	if lm := m.(listModel[Feed]); lm.mode != modeView {
		t.Fatalf("esc in jobs: mode %v, want view", lm.mode)
	}
	if want := (jobView{Search: "golang", Sources: []string{"lever"}, Days: 7, Sort: "newest"}); !reflect.DeepEqual(saved["go"], want) {
		t.Fatalf("saved %+v, want %+v", saved["go"], want)
	}
	press("o")
	if p := m.(listModel[Feed]).jobs; !reflect.DeepEqual(p.settings(), saved["go"]) || titles() != "Golang Lead" {
		t.Fatalf("reopened with %+v, jobs %q; want saved settings applied", p.settings(), titles())
	}
	press("c") // clear: search, filters and sort reset, and saved
	if titles() != "PHP Engineer, Go Developer, Golang Lead" || !reflect.DeepEqual(saved["go"], jobView{Sources: []string{}, Sort: "newest"}) {
		t.Fatalf("after c: jobs %q, saved %+v", titles(), saved["go"])
	}
}

func TestCompany(t *testing.T) {
	for board, want := range map[string]string{"greenhouse:stripe": "stripe", "workday:nvidia.wd5/nvidiaexternalcareersite": "nvidia", "lever-eu:a.b": "a.b"} {
		if got := company(board); got != want {
			t.Errorf("company(%q) = %q, want %q", board, got, want)
		}
	}
}

func TestPlainText(t *testing.T) {
	for in, want := range map[string]string{
		"<h2>About</h2><p>We write <b>Go</b> &amp; Rust.</p><ul><li>One</li><li>Two</li></ul>": "About\n\nWe write Go & Rust.\n\n• One\n• Two",
		"Plain text, <5 years.\n\n\n\nNext":                                                    "Plain text, <5 years.\n\nNext",
	} {
		if got := plainText(in); got != want {
			t.Errorf("plainText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJobsPaneThumbsAndSnapshot(t *testing.T) {
	jobs := []Job{
		{Board: "lever:a", Title: "Go Developer", URL: "https://a/1"},
		{Board: "lever:a", Title: "Rust Developer", URL: "https://a/2"},
	}
	k := feedKind
	k.jobs = func(Feed) ([]Job, jobView, error) { return jobs, jobView{}, nil }
	likes := map[string]string{}
	k.likes = func() (map[string]string, error) { return maps.Clone(likes), nil }
	k.saveLike = func(url, thumb string) error {
		if thumb == "" {
			delete(likes, url)
		} else {
			likes[url] = thumb
		}
		return nil
	}
	defer func(f func(context.Context, Job) (map[string]string, error)) { describe = f }(describe)
	describe = func(context.Context, Job) (map[string]string, error) { return nil, nil } // the test sends descsMsg itself
	var m module = newListModel(k, "", []Feed{{ID: "go", Title: "Go", Enabled: true}})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	press := func(s string) tea.Cmd {
		msg := tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
		if s == "esc" {
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		}
		var cmd tea.Cmd
		m, cmd = m.Update(msg)
		return cmd
	}
	pane := func() jobsPane { return m.(listModel[Feed]).jobs }

	press("+") // Go Developer liked
	press("j")
	press("-") // Rust Developer disliked
	press("-") // and cleared again
	if !maps.Equal(likes, map[string]string{"https://a/1": "up"}) {
		t.Fatalf("likes %v, want Go Developer up", likes)
	}
	press("t") // liked only
	if got := pane().shown; len(got) != 1 || got[0].Title != "Go Developer" {
		t.Fatalf("thumb filter up shows %v", got)
	}
	if body, _ := pane().view(); !strings.Contains(body, "👍  Go Developer") {
		t.Fatalf("no thumb before the title:\n%s", body)
	}

	cmd := press(" ")
	if p := pane(); p.snap != "https://a/1" || p.loading != "https://a/1" || cmd == nil {
		t.Fatalf("space: snap %q, loading %q; want the job fetching", p.snap, p.loading)
	}
	m, _ = m.Update(descsMsg{"https://a/1", map[string]string{"https://a/1": "<p>Write Go.</p>", "https://a/2": ""}, nil})
	if body, keys := pane().view(); !strings.Contains(body, "Write Go.") || !strings.Contains(keys, "close") {
		t.Fatalf("snapshot lacks the description:\n%s", body)
	}
	press("esc")
	if lm := m.(listModel[Feed]); lm.mode != modeJobs || lm.jobs.snap != "" {
		t.Fatalf("esc in snapshot: mode %v, snap %q; want the list back", lm.mode, lm.jobs.snap)
	}
	if press(" ") != nil { // cached
		t.Fatalf("reopened snapshot fetches again")
	}
}
