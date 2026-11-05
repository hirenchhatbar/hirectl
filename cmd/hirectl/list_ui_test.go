package main

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFormPicks(t *testing.T) {
	k := feedKind
	k.picks = func(v []string) (string, []pick) {
		f, _ := k.parse(v)
		return suggest([]Job{{Board: "lever:a", Title: "Go Dev"}, {Board: "lever:b", Title: "PHP Dev"}}, f)
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
	if !lm.onPicks() || len(lm.picks) != 2 || lm.picks[0].value != "lever:a" {
		t.Fatalf("on picks %v, picks %v; want lever:a first, as skills say Go, then lever:b", lm.onPicks(), lm.picks)
	}
	if got := lm.inputs[4].Value(); got != "lever:a" {
		t.Fatalf("boards on opening suggestions = %q, want lever:a auto-selected", got)
	}
	if v := m.View(); !strings.Contains(v, "[x] lever:a") || !strings.Contains(v, "Suggested boards for Go: 1 of 2 boards") || !strings.Contains(v, "Matching roles: Go Dev") {
		t.Fatalf("view lacks checked pick, heading or detail:\n%s", v)
	}
	press(space)
	if got := m.(listModel[Feed]).inputs[4].Value(); got != "" {
		t.Fatalf("boards after space = %q, want empty", got)
	}
	press(down, space)
	if got := m.(listModel[Feed]).inputs[4].Value(); got != "lever:b" {
		t.Fatalf("boards after down, space = %q, want lever:b", got)
	}
	press(tab)
	if lm := m.(listModel[Feed]); lm.focus != 4 {
		t.Fatalf("tab from picks: focus = %d, want 4 (Boards)", lm.focus)
	}
	press(tab)
	if lm := m.(listModel[Feed]); lm.focus != 0 {
		t.Fatalf("tab from Boards: focus = %d, want 0", lm.focus)
	}
}

func TestFormDuplicateID(t *testing.T) {
	var m module = newListModel(feedKind, filepath.Join(t.TempDir(), "feeds.json"), []Feed{{ID: "go-remote", Title: "Go remote", Boards: []string{"lever:a"}, Enabled: true}})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	for _, k := range []tea.KeyPressMsg{{Code: 'a', Text: "a"}, {Code: 'G', Text: "Go Remote"}, tab, tab, tab, tab, {Code: 'l', Text: "lever:b"}, {Code: tea.KeyEnter}} {
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
	f := Feed{ID: "old-id", Title: "Go remote", Boards: []string{"lever:a"}, Enabled: true}
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
