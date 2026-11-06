package main

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestConfigCycleTheme(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme(defaultTheme) })
	if err := applyTheme("nope"); err == nil || themeName != defaultTheme {
		t.Fatalf("unknown theme: err %v, theme %q; want error and %s", err, themeName, defaultTheme)
	}
	path := filepath.Join(t.TempDir(), configFile)
	var m module = newConfigModel(path, Config{})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	names := themeNames()
	want := names[(slices.Index(names, defaultTheme)+1)%len(names)]
	if themeName != want {
		t.Fatalf("after →: theme %q, want %q", themeName, want)
	}
	if cfg, err := loadJSON[Config](path); err != nil || cfg.Theme != want {
		t.Fatalf("saved %+v, %v; want theme %q", cfg, err, want)
	}
	if !strings.Contains(m.View(), want) {
		t.Fatal("page does not show the theme")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if themeName != defaultTheme {
		t.Fatalf("after ←: theme %q, want %s", themeName, defaultTheme)
	}
}

func TestConfigLocations(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFile)
	var m module = newConfigModel(path, Config{Theme: "nord"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 10})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	for _, r := range "Remote India,, India " {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	want := Config{Theme: "nord", Locations: []string{"Remote India", "India"}}
	if cfg, err := loadJSON[Config](path); err != nil || !reflect.DeepEqual(cfg, want) {
		t.Fatalf("saved %+v, %v; want %+v", cfg, err, want)
	}
	if !strings.Contains(m.View(), "Remote India, India") {
		t.Fatal("page does not show the tidied locations")
	}
}

func TestConfigBoards(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFile)
	var m module = newConfigModel(path, Config{})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	press := func(keys ...tea.KeyPressMsg) {
		for _, k := range keys {
			m, _ = m.Update(k)
		}
	}
	bs, enter := tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyEnter}
	up := tea.KeyPressMsg{Code: tea.KeyUp}
	press(up, up, bs, bs, tea.KeyPressMsg{Code: '0', Text: "0"}, enter)
	if cfg, _ := loadJSON[Config](path); cfg.Boards != 0 || !strings.Contains(m.View(), "at least 1") {
		t.Fatalf("saved %+v; want 0 rejected", cfg)
	}
	press(bs, tea.KeyPressMsg{Code: '3', Text: "3"}, enter)
	if cfg, err := loadJSON[Config](path); err != nil || cfg.Boards != 3 || cfg.boards() != 3 {
		t.Fatalf("saved %+v, %v; want 3 boards", cfg, err)
	}
	if (Config{}).boards() != defaultBoards {
		t.Fatal("unset boards should mean the default")
	}
}

func TestConfigSidebar(t *testing.T) {
	t.Cleanup(func() { sidebarWidth = defaultSidebar })
	path := filepath.Join(t.TempDir(), configFile)
	var m module = newConfigModel(path, Config{Sidebar: maxSidebar - 1})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.typing() {
		t.Fatal("sidebar row should not take text")
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if cfg, err := loadJSON[Config](path); err != nil || cfg.Sidebar != maxSidebar || sidebarWidth != maxSidebar || cmd == nil {
		t.Fatalf("saved %+v, %v, applied %d; want %d and a relayout", cfg, err, sidebarWidth, maxSidebar)
	}
	if m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyRight}); cmd != nil || sidebarWidth != maxSidebar {
		t.Fatalf("→ past max: width %d", sidebarWidth)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if sidebarWidth != maxSidebar-sidebarStep || !strings.Contains(m.View(), "‹ 78 ›") {
		t.Fatalf("after ←: width %d", sidebarWidth)
	}
	if (Config{Sidebar: 1}).sidebar() != minSidebar || (Config{}).sidebar() != defaultSidebar {
		t.Fatal("sidebar should clamp and default")
	}
}

func TestConfigRefresh(t *testing.T) {
	t.Cleanup(func() { refreshEvery = 0 })
	path := filepath.Join(t.TempDir(), configFile)
	var m module = newConfigModel(path, Config{})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 14})
	gen := refreshGen
	bs, enter := tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyEnter}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	m, _ = m.Update(bs)
	m, _ = m.Update(tea.KeyPressMsg{Code: '-', Text: "-"})
	m, _ = m.Update(enter)
	if !strings.Contains(m.View(), "0 for off") || refreshEvery != 0 {
		t.Fatal("negative refresh should be rejected")
	}
	m, _ = m.Update(bs)
	m, _ = m.Update(tea.KeyPressMsg{Code: '5', Text: "5"})
	m, cmd := m.Update(enter)
	if cfg, err := loadJSON[Config](path); err != nil || cfg.Refresh != 5 || refreshEvery != 5*time.Second || refreshGen != gen+1 || cmd == nil {
		t.Fatalf("saved %+v, %v, every %v; want 5s and a new timer", cfg, err, refreshEvery)
	}
}
