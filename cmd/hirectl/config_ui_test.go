package main

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestConfigCycleTheme(t *testing.T) {
	t.Cleanup(func() { _ = applyTheme(defaultTheme) })
	if err := applyTheme("nope"); err == nil || themeName != defaultTheme {
		t.Fatalf("unknown theme: err %v, theme %q; want error and %s", err, themeName, defaultTheme)
	}
	path := filepath.Join(t.TempDir(), configFile)
	var m module = newConfigModel(path, Config{})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
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
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
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
	m, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	press := func(keys ...tea.KeyPressMsg) {
		for _, k := range keys {
			m, _ = m.Update(k)
		}
	}
	bs, enter := tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyEnter}
	press(tea.KeyPressMsg{Code: tea.KeyUp}, bs, bs, tea.KeyPressMsg{Code: '0', Text: "0"}, enter)
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
