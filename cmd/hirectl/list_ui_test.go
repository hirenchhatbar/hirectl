package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFormPicks(t *testing.T) {
	k := feedKind
	k.picks = func(v []string) (string, []pick) {
		f, _ := k.parse(v)
		return suggest([]Job{{Board: "lever:a", Title: "Go Dev"}, {Board: "lever:b", Title: "PHP Dev"}}, f, 10)
	}
	var m module = newListModel(k, "", nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	press := func(keys ...tea.KeyPressMsg) {
		for _, key := range keys {
			m, _ = m.Update(key)
		}
	}
	tab, space, down := tea.KeyPressMsg{Code: tea.KeyTab}, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, tea.KeyPressMsg{Code: tea.KeyDown}
	press(tea.KeyPressMsg{Code: 'a', Text: "a"}, tab, tab, tea.KeyPressMsg{Code: 'G', Text: "Go"}, tab, tab)
	lm := m.(listModel[Feed])
	lever := slices.Index(integrationNames(), "lever")
	if !lm.onPicks() || len(lm.picks) != len(integrations) || lm.picks[lever].value != "lever" || lm.inputs[4].Value() != "" {
		t.Fatalf("on picks %v, picks %v, sources %q; want every source, none checked", lm.onPicks(), lm.picks, lm.inputs[4].Value())
	}
	press(append(slices.Repeat([]tea.KeyPressMsg{down}, lever), space)...)
	if got := m.(listModel[Feed]).inputs[4].Value(); got != "lever" {
		t.Fatalf("sources after down to lever, space = %q, want lever", got)
	}
	if v := m.View(); !strings.Contains(v, "[x] lever ") || !strings.Contains(v, "Sources for Go: 1 boards match") || !strings.Contains(v, "Top boards: lever:a (1)") {
		t.Fatalf("view lacks checked pick, heading or detail:\n%s", v)
	}
	press(space)
	if got := m.(listModel[Feed]).inputs[4].Value(); got != "" {
		t.Fatalf("sources after second space = %q, want empty", got)
	}
	press(tab)
	if lm := m.(listModel[Feed]); lm.focus != 4 {
		t.Fatalf("tab from picks: focus = %d, want 4 (Sources)", lm.focus)
	}
	press(tab)
	if lm := m.(listModel[Feed]); lm.focus != 0 {
		t.Fatalf("tab from Sources: focus = %d, want 0", lm.focus)
	}
}

func TestFormDuplicateID(t *testing.T) {
	var m module = newListModel(feedKind, filepath.Join(t.TempDir(), "feeds.json"), []Feed{{ID: "go-remote", Title: "Go remote", Enabled: true}})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	for _, k := range []tea.KeyPressMsg{{Code: 'a', Text: "a"}, {Code: 'G', Text: "Go Remote"}, tab, tab, tab, tab, {Code: 'l', Text: "lever"}, {Code: tea.KeyEnter}} {
		m, _ = m.Update(k)
	}
	if lm := m.(listModel[Feed]); len(lm.items) != 1 || !strings.Contains(lm.status, `"go-remote" already exists`) {
		t.Fatalf("items %v, status %q; want duplicate ID rejected", lm.items, lm.status)
	}
}

func TestFormFillsID(t *testing.T) {
	var m module = newListModel(feedKind, "", nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	id := func() string { return m.(listModel[Feed]).inputs[1].Value() }
	tab, shiftTab := tea.KeyPressMsg{Code: tea.KeyTab}, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	for _, k := range []tea.KeyPressMsg{{Code: 'a', Text: "a"}, {Code: 'G', Text: "Go remote"}} {
		m, _ = m.Update(k)
	}
	if id() != "go-remote" {
		t.Fatalf("id = %q, want go-remote from title", id())
	}
	for _, k := range []tea.KeyPressMsg{tab, {Code: 'x', Text: "x"}, shiftTab, {Code: 'y', Text: "y"}} {
		m, _ = m.Update(k)
	}
	if id() != "go-remotex" {
		t.Fatalf("id = %q, want go-remotex kept once typed over", id())
	}
}

func TestFormDeriveKey(t *testing.T) {
	f := Feed{ID: "old-id", Title: "Go remote", Enabled: true}
	var m module = newListModel(feedKind, "", []Feed{f})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	for _, k := range []tea.KeyPressMsg{{Code: 'e', Text: "e"}, {Code: 'x', Text: "x"}} {
		m, _ = m.Update(k)
	}
	if id := m.(listModel[Feed]).inputs[1].Value(); id != "old-id" {
		t.Fatalf("editing title changed id to %q, want old-id kept", id)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl})
	if id := m.(listModel[Feed]).inputs[1].Value(); id != "go-remotex" {
		t.Fatalf("id after ctrl+g = %q, want go-remotex", id)
	}
}

func TestFormPicksEnterSaves(t *testing.T) {
	k := feedKind
	k.picks = func([]string) (string, []pick) { return "", []pick{{value: "lever"}} }
	var m module = newListModel(k, filepath.Join(t.TempDir(), "feeds.json"), nil)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	for _, k := range []tea.KeyPressMsg{{Code: 'a', Text: "a"}, {Code: 'G', Text: "Go"}, tab, tab, tab, tab} {
		m, _ = m.Update(k)
	}
	if !m.(listModel[Feed]).onPicks() {
		t.Fatal("not on the checklist")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if lm := m.(listModel[Feed]); len(lm.items) != 1 || lm.mode != modeView {
		t.Fatalf("items %v, mode %v, status %q; want feed saved with Enter on the checklist", lm.items, lm.mode, lm.status)
	}
}
