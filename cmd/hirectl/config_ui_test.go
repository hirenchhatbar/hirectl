package main

import (
	"path/filepath"
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
	var m module = configModel{path: path}
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
