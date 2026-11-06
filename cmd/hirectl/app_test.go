package main

import (
	"strings"
	"testing"
	"time"

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
	if got := m.(app).active; got != 0 {
		t.Fatalf("start: active = %d, want home (0)", got)
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyF10}); !isQuit(cmd) {
		t.Fatal("F10 did not quit")
	}
}

func TestAppBarClick(t *testing.T) {
	var m tea.Model = testApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// Bar reads " F2 Feeds F5 Refresh F10 Quit" from column 0.
	if _, cmd := m.Update(tea.MouseClickMsg{X: 40, Y: 23, Button: tea.MouseLeft}); cmd != nil {
		t.Fatal("click right of the bar did something")
	}
	m, _ = m.Update(tea.MouseClickMsg{X: 3, Y: 23, Button: tea.MouseLeft})
	if got := m.(app).active; got != 0 {
		t.Fatalf("click F2 Feeds: active = %d, want 0", got)
	}
	if _, cmd := m.Update(tea.MouseClickMsg{X: 12, Y: 23, Button: tea.MouseLeft}); isQuit(cmd) {
		t.Fatal("click F5 Refresh quit")
	}
	if _, cmd := m.Update(tea.MouseClickMsg{X: 22, Y: 23, Button: tea.MouseLeft}); !isQuit(cmd) {
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

func TestAppEscHome(t *testing.T) {
	cfg := newConfigModel(t.TempDir()+"/"+configFile, Config{})
	feeds := slot{key: "f2", name: "Feeds", m: newListModel(feedKind, "", nil)}
	var m tea.Model = newApp(feeds, slot{key: "f9", name: "Config", m: cfg})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF9})
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc: no cmd")
	}
	if m, _ = m.Update(cmd()); m.(app).active != 0 {
		t.Fatalf("esc: active = %d, want home (0)", m.(app).active)
	}
}

func TestAppQuitQ(t *testing.T) {
	var m tea.Model = testApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	q := tea.KeyPressMsg{Code: 'q', Text: "q"}
	if _, cmd := m.Update(q); !isQuit(cmd) {
		t.Fatal("q on the list did not quit")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"}) // feed form, Title focused
	if _, cmd := m.Update(q); isQuit(cmd) {
		t.Fatal("q in a text input quit, want it typed")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyF10}); !isQuit(cmd) {
		t.Fatal("F10 in a text input did not quit")
	}
}

func TestAppConfigBesideSidebar(t *testing.T) {
	cfg := newConfigModel(t.TempDir()+"/"+configFile, Config{})
	feeds := slot{key: "f2", name: "Feeds", m: newListModel(feedKind, "", nil)}
	var m tea.Model = newApp(feeds, slot{key: "f9", name: "Config", m: cfg})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyF9})
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != 24 || !strings.Contains(lines[1], "no feeds") || !strings.Contains(lines[1], "Config") {
		t.Fatalf("config not right of the feeds sidebar:\n%s", strings.Join(lines, "\n"))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > 80 {
			t.Fatalf("line %d is %d wide, want at most 80", i, w)
		}
	}
}

func TestAppAutoRefresh(t *testing.T) {
	t.Cleanup(func() { refreshEvery = 0 })
	var m tea.Model = testApp()
	if m.Init() != nil {
		t.Fatal("auto refresh off should start no timer")
	}
	refreshEvery = time.Second
	if m.Init() == nil {
		t.Fatal("auto refresh on should start a timer")
	}
	if _, cmd := m.Update(refreshTick(refreshGen)); cmd == nil {
		t.Fatal("tick should refresh and rearm")
	}
	if _, cmd := m.Update(refreshTick(refreshGen - 1)); cmd != nil {
		t.Fatal("tick of a replaced timer should be dropped")
	}
}
