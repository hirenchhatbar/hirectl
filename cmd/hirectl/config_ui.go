package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const configFile = "config.json"

// defaultBoards is how many boards run fetches per feed when config.json sets none.
const defaultBoards = 10

// Config is config.json: settings the config module changes.
type Config struct {
	Theme     string   `json:"theme,omitempty"`     // "" means defaultTheme
	Locations []string `json:"locations,omitempty"` // prefilled in a new feed
	Boards    int      `json:"boards,omitempty"`    // top boards run fetches per feed; 0 means defaultBoards
}

func (c Config) boards() int { return cmp.Or(c.Boards, defaultBoards) }

// configInputs are the config page's text rows, below Theme: label, placeholder, note.
var configInputs = [][3]string{
	{"Locations", "Remote Worldwide, Remote India, India", "prefilled in a new feed"},
	{"Boards", strconv.Itoa(defaultBoards), "boards fetched per feed, those with most matching jobs"},
}

// configModel is a module: one settings page, saved to config.json on every change.
type configModel struct {
	path          string
	cfg           Config
	row           int               // 0 theme, else inputs[row-1]
	inputs        []textinput.Model // in configInputs order; inputs[row-1] is focused
	status        string            // error shown in the bottom bar until the next key
	width, height int
}

func newConfigModel(path string, cfg Config) configModel {
	m := configModel{path: path, cfg: cfg}
	for _, c := range configInputs {
		in := textinput.New()
		in.Prompt = fmt.Sprintf(labelFmt, c[0])
		in.Placeholder = c[1]
		m.inputs = append(m.inputs, in)
	}
	m.revert()
	return m
}

// revert sets the inputs to the saved values.
func (m *configModel) revert() {
	for i, v := range []string{strings.Join(m.cfg.Locations, ", "), strconv.Itoa(m.cfg.boards())} {
		m.inputs[i].SetValue(v)
	}
}

func (m configModel) Update(msg tea.Msg) (module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for i := range m.inputs {
			m.inputs[i].SetWidth(max(m.width-14, 10))
		}
		return m, nil
	case tea.KeyPressMsg:
		m.status = ""
		rows := len(m.inputs) + 1
		switch msg.String() {
		case "esc": // drop unsaved input, back to home
			m.revert()
			return m, closeModule
		case "up", "shift+tab":
			return m, m.focus((m.row + rows - 1) % rows)
		case "down", "tab":
			return m, m.focus((m.row + 1) % rows)
		}
		if m.row > 0 {
			if msg.String() == "enter" {
				m.saveInput()
				return m, nil
			}
			break // typing goes to the input
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "left", "h":
			m.cycleTheme(-1)
		case "right", "l", "enter", "space":
			m.cycleTheme(1)
		}
		return m, nil
	}
	if m.row == 0 {
		return m, nil
	}
	var cmd tea.Cmd
	m.inputs[m.row-1], cmd = m.inputs[m.row-1].Update(msg)
	return m, cmd
}

// focus moves to row, focusing its input if it has one.
func (m *configModel) focus(row int) tea.Cmd {
	if m.row > 0 {
		m.inputs[m.row-1].Blur()
	}
	if m.row = row; row > 0 {
		return m.inputs[row-1].Focus()
	}
	return nil
}

// save writes cfg and only then makes it current, so a failed save loses nothing on disk.
func (m *configModel) save(cfg Config) bool {
	if err := saveJSON(m.path, cfg); err != nil {
		m.status = "save failed: " + err.Error()
		return false
	}
	m.cfg = cfg
	return true
}

// cycleTheme saves and applies the next (dir>0) or previous theme.
func (m *configModel) cycleTheme(dir int) {
	names := themeNames()
	i := slices.Index(names, themeName)
	cfg := m.cfg
	cfg.Theme = names[(i+dir+len(names))%len(names)]
	if m.save(cfg) {
		_ = applyTheme(cfg.Theme) // from themeNames, so known
	}
}

// saveInput saves the focused input's value, tidied: default locations for new feeds, or boards per feed.
func (m *configModel) saveInput() {
	cfg := m.cfg
	v := m.inputs[m.row-1].Value()
	switch m.row {
	case 1:
		cfg.Locations = parseList(v)
	case 2:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 {
			m.status = "boards: want a whole number, at least 1"
			return
		}
		cfg.Boards = n
	}
	if m.save(cfg) {
		m.revert()
	}
}

func (m configModel) View() string {
	if m.width == 0 {
		return "" // size not known yet
	}
	theme := fmt.Sprintf(labelFmt, "Theme") + "‹ " + themeName + " ›"
	bar := " ←/→ theme  ↑/↓ row  esc back  q quit"
	if m.row == 0 {
		theme = selStyle.Render(theme)
	} else {
		bar = " enter save  esc revert, back  ↑/↓ row"
	}
	lines := []string{headStyle.Render("Config"), "", theme}
	for i, in := range m.inputs {
		lines = append(lines, in.View(), offStyle.Render(strings.Repeat(" ", 10)+configInputs[i][2]))
	}
	if m.status != "" {
		bar = " " + errStyle.Render(m.status)
	}
	return paneStyle.Width(m.width).Height(m.height-1).Render(strings.Join(lines, "\n")) + "\n" + ansi.Truncate(bar, m.width, "…")
}
