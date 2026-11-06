package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const configFile = "config.json"

// defaultBoards is how many boards run fetches per feed when config.json sets none.
const defaultBoards = 10

// Sidebar width bounds and step, in cells including the border.
const (
	defaultSidebar = 30
	minSidebar     = 16
	maxSidebar     = 80
	sidebarStep    = 2
)

// spinRows are the config page's rows cycled with ←/→ above the text inputs: Theme, Sidebar.
const spinRows = 2

// Config is config.json: settings the config module changes.
type Config struct {
	Theme     string   `json:"theme,omitempty"`     // "" means defaultTheme
	Locations []string `json:"locations,omitempty"` // prefilled in a new feed
	Boards    int      `json:"boards,omitempty"`    // top boards run fetches per feed; 0 means defaultBoards
	Sidebar   int      `json:"sidebar,omitempty"`   // feeds sidebar width; 0 means defaultSidebar
	Refresh   int      `json:"refresh,omitempty"`   // seconds between TUI auto refreshes; 0 means off
}

func (c Config) boards() int { return cmp.Or(c.Boards, defaultBoards) }

// sidebar is the sidebar width, clamped so a hand-edited config.json cannot break the layout.
func (c Config) sidebar() int {
	return min(max(cmp.Or(c.Sidebar, defaultSidebar), minSidebar), maxSidebar)
}

// configInputs are the config page's text rows, below the spin rows: label, placeholder, note.
var configInputs = [][3]string{
	{"Locations", "Remote Worldwide, Remote India, India", "prefilled in a new feed"},
	{"Boards", strconv.Itoa(defaultBoards), "boards fetched per feed, those with most matching jobs"},
	{"Refresh", "0", "seconds between auto refreshes of data run and discover write, 0 off; F5 any time"},
}

// configModel is a module: one settings page, saved to config.json on every change.
type configModel struct {
	path          string
	cfg           Config
	row           int               // 0 theme, 1 sidebar, else inputs[row-spinRows]
	inputs        []textinput.Model // in configInputs order; inputs[row-spinRows] is focused
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
	for i, v := range []string{strings.Join(m.cfg.Locations, ", "), strconv.Itoa(m.cfg.boards()), strconv.Itoa(m.cfg.Refresh)} {
		m.inputs[i].SetValue(v)
	}
}

func (m configModel) Update(msg tea.Msg) (module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for i := range m.inputs {
			m.inputs[i].SetWidth(max(m.paneWidth()-14, 10))
		}
		return m, nil
	case tea.KeyPressMsg:
		m.status = ""
		rows := len(m.inputs) + spinRows
		switch msg.String() {
		case "esc": // drop unsaved input, back to home
			m.revert()
			return m, closeModule
		case "up", "shift+tab":
			return m, m.focus((m.row + rows - 1) % rows)
		case "down", "tab":
			return m, m.focus((m.row + 1) % rows)
		}
		if m.typing() {
			if msg.String() == "enter" {
				return m, m.saveInput()
			}
			break // typing goes to the input
		}
		switch msg.String() {
		case "+", "=": // = is + unshifted
			return m, m.stepSidebar(1)
		case "-":
			return m, m.stepSidebar(-1)
		}
		dir := 0
		switch msg.String() {
		case "left", "h":
			dir = -1
		case "right", "l", "enter", "space":
			dir = 1
		}
		if dir == 0 {
			return m, nil
		}
		if m.row == 0 {
			m.cycleTheme(dir)
			return m, nil
		}
		return m, m.stepSidebar(dir)
	}
	if !m.typing() {
		return m, nil
	}
	var cmd tea.Cmd
	m.inputs[m.row-spinRows], cmd = m.inputs[m.row-spinRows].Update(msg)
	return m, cmd
}

// focus moves to row, focusing its input if it has one.
func (m *configModel) focus(row int) tea.Cmd {
	if m.typing() {
		m.inputs[m.row-spinRows].Blur()
	}
	if m.row = row; m.typing() {
		return m.inputs[row-spinRows].Focus()
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

// stepSidebar saves and applies a sidebar width one step wider (dir>0) or narrower, within bounds;
// it asks for the window size again so the feeds module relayouts.
func (m *configModel) stepSidebar(dir int) tea.Cmd {
	cfg := m.cfg
	cfg.Sidebar = min(max(cfg.sidebar()+dir*sidebarStep, minSidebar), maxSidebar)
	if cfg.Sidebar == m.cfg.sidebar() || !m.save(cfg) {
		return nil
	}
	sidebarWidth = cfg.Sidebar
	return tea.RequestWindowSize
}

// saveInput saves the focused input's value, tidied: default locations for new feeds, boards per feed,
// or the auto-refresh interval, whose new timer it returns.
func (m *configModel) saveInput() tea.Cmd {
	cfg := m.cfg
	v := m.inputs[m.row-spinRows].Value()
	switch m.row - spinRows {
	case 0:
		cfg.Locations = parseList(v)
	case 1:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 {
			m.status = "boards: want a whole number, at least 1"
			return nil
		}
		cfg.Boards = n
	case 2:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			m.status = "refresh: want a whole number of seconds, 0 for off"
			return nil
		}
		cfg.Refresh = n
	}
	if !m.save(cfg) {
		return nil
	}
	m.revert()
	if d := time.Duration(cfg.Refresh) * time.Second; d != refreshEvery {
		refreshEvery = d
		refreshGen++
		return scheduleRefresh()
	}
	return nil
}

// paneWidth is the page's width right of the home sidebar, which the app draws beside it.
func (m configModel) paneWidth() int { return max(m.width-sidebarWidth, 20) }

// typing is true on Locations, Boards and Refresh, the text rows.
func (m configModel) typing() bool { return m.row >= spinRows }

func (m configModel) View() string {
	if m.width == 0 {
		return "" // size not known yet
	}
	theme := fmt.Sprintf(labelFmt, "Theme") + "‹ " + themeName + " ›"
	sidebar := fmt.Sprintf(labelFmt, "Sidebar") + fmt.Sprintf("‹ %d ›", sidebarWidth)
	bar := " enter save  esc revert, back  ↑/↓ row"
	switch m.row {
	case 0:
		theme, bar = selStyle.Render(theme), " ←/→ theme  +/- sidebar  ↑/↓ row  esc back  q quit"
	case 1:
		sidebar, bar = selStyle.Render(sidebar), " ←/→ +/- width  ↑/↓ row  esc back  q quit"
	}
	lines := []string{headStyle.Render("Config"), "", theme, sidebar, offStyle.Render(strings.Repeat(" ", 10) + "feeds sidebar width, in cells")}
	for i, in := range m.inputs {
		lines = append(lines, in.View(), offStyle.Render(strings.Repeat(" ", 10)+configInputs[i][2]))
	}
	if m.status != "" {
		bar = " " + errStyle.Render(m.status)
	}
	return paneStyle.Width(m.paneWidth()).Height(m.height-1).Render(strings.Join(lines, "\n")) + "\n" + ansi.Truncate(bar, m.width, "…")
}
