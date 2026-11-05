package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func testApp() app {
	return newApp(slot{key: "f2", name: "Feeds", m: newListModel(feedKind, "", nil)})
}

func TestAppKeys(t *testing.T) {
	var m tea.Model = testApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if got := m.(app).active; got != -1 {
		t.Fatalf("start: active = %d, want home (-1)", got)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	if got := m.(app).active; got != 0 {
		t.Fatalf("F2: active = %d, want 0", got)
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyF10}); !isQuit(cmd) {
		t.Fatal("F10 did not quit")
	}
}

func TestAppBarClick(t *testing.T) {
	var m tea.Model = testApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// Bar reads " F2 Feeds F10 Quit" from column 0.
	if _, cmd := m.Update(tea.MouseClickMsg{X: 40, Y: 23, Button: tea.MouseLeft}); cmd != nil {
		t.Fatal("click right of the bar did something")
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 3, Y: 23, Button: tea.MouseLeft})
	if got := m.(app).active; got != 0 {
		t.Fatalf("click F2 Feeds: active = %d, want 0", got)
	}
	if _, cmd := m.Update(tea.MouseClickMsg{X: 12, Y: 23, Button: tea.MouseLeft}); !isQuit(cmd) {
		t.Fatal("click F10 Quit did not quit")
	}
}

func TestAppBarFillsWidth(t *testing.T) {
	var m tea.Model = testApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != 24 {
		t.Fatalf("got %d lines, want 24", len(lines))
	}
	if w := ansi.StringWidth(lines[23]); w != 80 {
		t.Fatalf("bar width = %d, want 80", w)
	}
}
