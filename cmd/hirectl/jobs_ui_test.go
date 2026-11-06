package main

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
}
