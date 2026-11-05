package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// module is one screen of the app, opened by its function key.
type module interface {
	Update(tea.Msg) (module, tea.Cmd)
	View() string
}

type slot struct {
	key, name string // key as tea reports it, e.g. "f2"
	m         module
}

// app is the root model: home screen or the active module, plus the function-key bar.
type app struct {
	slots         []slot
	active        int // index into slots, -1 for home
	width, height int
}

const (
	quitKey = "f10"
	barPad  = 1 // columns before the first function key
	barRows = 2 // blank spacer row plus the function-key bar
)

var (
	barStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#303030")).Foreground(lipgloss.Color("#9E9E9E"))
	fkeyStyle   = barStyle.Bold(true).Foreground(lipgloss.Color("#D0D0D0"))
	activeStyle = barStyle.Bold(true).Foreground(lipgloss.Color("#87AFD7"))
	logoColors  = []string{"#FF5F87", "#FF87AF", "#D787FF", "#87AFFF", "#5FD7FF"}
)

const logo = ` _      _                    _    _
| |__  (_) _ __   ___   ___ | |_ | |
| '_ \ | || '__| / _ \ / __|| __|| |
| | | || || |   |  __/| (__ | |_ | |
|_| |_||_||_|    \___| \___| \__||_|`

// logoView centers the colored logo and version in a w×h box.
func logoView(w, h int) string {
	var art []string
	for i, line := range strings.Split(logo, "\n") {
		art = append(art, lipgloss.NewStyle().Foreground(lipgloss.Color(logoColors[i%len(logoColors)])).Render(line))
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center,
		strings.Join(art, "\n")+"\n\n"+offStyle.Render("v"+version))
}

func newApp(slots ...slot) app { return app{slots: slots, active: -1} }

func (a app) Init() tea.Cmd { return nil }

// barItems lists the function-key bar entries in display order; the last is quit.
func (a app) barItems() []slot {
	return append(append([]slot(nil), a.slots...), slot{key: quitKey, name: "Quit"})
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, a.broadcast(tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height - barRows})
	case tea.KeyPressMsg:
		if k := msg.String(); k == "ctrl+c" || k == quitKey {
			return a, tea.Quit
		}
		if a.open(msg.String()) {
			return a, nil
		}
		if a.active < 0 {
			if msg.String() == "q" {
				return a, tea.Quit
			}
			return a, nil
		}
	case tea.MouseClickMsg:
		if msg.Y == a.height-1 {
			x := barPad
			for _, it := range a.barItems() {
				w := ansi.StringWidth(barLabel(it))
				if msg.X >= x && msg.X < x+w {
					if it.key == quitKey {
						return a, tea.Quit
					}
					a.open(it.key)
					break
				}
				x += w + 1
			}
			return a, nil
		}
	}
	switch msg.(type) {
	case tea.KeyPressMsg, tea.MouseMsg: // input goes to the active module only
		if a.active < 0 {
			return a, nil
		}
		var cmd tea.Cmd
		a.slots[a.active].m, cmd = a.slots[a.active].m.Update(msg)
		return a, cmd
	}
	return a, a.broadcast(msg) // timers such as cursor blink reach every module
}

// open switches to the module bound to key and reports whether one is.
func (a *app) open(key string) bool {
	for i, s := range a.slots {
		if s.key == key {
			a.active = i
			return true
		}
	}
	return false
}

func (a app) broadcast(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range a.slots {
		var cmd tea.Cmd
		a.slots[i].m, cmd = a.slots[i].m.Update(msg)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func barLabel(s slot) string { return strings.ToUpper(s.key) + " " + s.name }

func (a app) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	if a.width == 0 {
		return v // size not known yet
	}
	var bar []string
	for _, it := range a.barItems() {
		key, name, _ := strings.Cut(barLabel(it), " ")
		style := barStyle
		if a.active >= 0 && it.key == a.slots[a.active].key {
			style = activeStyle
		}
		bar = append(bar, fkeyStyle.Render(key)+style.Render(" "+name))
	}
	body := logoView(a.width, a.height-barRows)
	if a.active >= 0 {
		body = a.slots[a.active].m.View()
	}
	line := strings.Repeat(" ", barPad) + strings.Join(bar, barStyle.Render(" "))
	v.SetContent(body + "\n\n" + barStyle.Width(a.width).Render(ansi.Truncate(line, a.width, "")))
	return v
}
