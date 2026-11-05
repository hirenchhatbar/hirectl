package main

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const configFile = "config.json"

// Config is config.json: settings the config module changes.
type Config struct {
	Theme string `json:"theme,omitempty"` // "" means defaultTheme
}

// configModel is a module: one settings page, saved to config.json on every change.
type configModel struct {
	path          string
	cfg           Config
	status        string // error shown in the bottom bar until the next key
	width, height int
}

func (m configModel) Update(msg tea.Msg) (module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		m.status = ""
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "left", "h":
			m.cycleTheme(-1)
		case "right", "l", "enter", "space":
			m.cycleTheme(1)
		}
	}
	return m, nil
}

// cycleTheme saves and applies the next (dir>0) or previous theme.
func (m *configModel) cycleTheme(dir int) {
	names := themeNames()
	i := slices.Index(names, themeName)
	cfg := m.cfg
	cfg.Theme = names[(i+dir+len(names))%len(names)]
	if err := saveJSON(m.path, cfg); err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	m.cfg = cfg
	_ = applyTheme(cfg.Theme) // from themeNames, so known
}

func (m configModel) View() string {
	if m.width == 0 {
		return "" // size not known yet
	}
	row := fmt.Sprintf(labelFmt, "Theme") + "‹ " + themeName + " ›"
	body := strings.Join([]string{headStyle.Render("Config"), "", selStyle.Render(row)}, "\n")
	bar := " ←/→ theme  q quit"
	if m.status != "" {
		bar = " " + errStyle.Render(m.status)
	}
	return paneStyle.Width(m.width).Height(m.height-1).Render(body) + "\n" + ansi.Truncate(bar, m.width, "…")
}
