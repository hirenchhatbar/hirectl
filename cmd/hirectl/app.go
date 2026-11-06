package main

import (
	"cmp"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// module is one screen of the app, opened by its function key.
type module interface {
	Update(tea.Msg) (module, tea.Cmd)
	View() string
	typing() bool // a text input has focus, so q types rather than quits
}

// closeMsg asks the app to close the active module and show home; see closeModule.
type closeMsg struct{}

// closeModule is the cmd a module returns on Esc at its top level.
func closeModule() tea.Msg { return closeMsg{} }

// refreshMsg asks every module to reload the data files that commands such as run and discover write.
type refreshMsg struct{}

// refreshEvery is the auto-refresh interval, 0 for off; set from config.json by the config module,
// which bumps refreshGen on a change so the old timer's ticks are dropped.
var (
	refreshEvery time.Duration
	refreshGen   int
)

// refreshTick is an auto-refresh timer firing, from timer generation refreshGen.
type refreshTick int

// scheduleRefresh starts the auto-refresh timer, if on.
func scheduleRefresh() tea.Cmd {
	if refreshEvery <= 0 {
		return nil
	}
	gen := refreshGen
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return refreshTick(gen) })
}

type slot struct {
	key, name string // key as tea reports it, e.g. "f2"
	m         module
}

// app is the root model: the active module, plus the function-key bar. Home is the first module (Feeds).
type app struct {
	slots         []slot
	active        int // index into slots
	width, height int
}

const (
	quitKey    = "f10"
	refreshKey = "f5"
	barPad     = 1 // columns before the first function key
	barRows    = 2 // blank spacer row plus the function-key bar
)

var logoColors = []string{"#FF5F87", "#FF87AF", "#D787FF", "#87AFFF", "#5FD7FF"}

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

func newApp(slots ...slot) app { return app{slots: slots} }

func (a app) Init() tea.Cmd { return scheduleRefresh() }

// barItems lists the function-key bar entries by key number: the modules, refresh and quit.
func (a app) barItems() []slot {
	items := append(slices.Clone(a.slots), slot{key: refreshKey, name: "Refresh"}, slot{key: quitKey, name: "Quit"})
	slices.SortStableFunc(items, func(x, y slot) int { return cmp.Or(cmp.Compare(len(x.key), len(y.key)), strings.Compare(x.key, y.key)) })
	return items
}

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case closeMsg:
		a.active = 0
		return a, nil
	case refreshTick:
		if int(msg) != refreshGen {
			return a, nil // from a timer the config module replaced
		}
		return a, tea.Batch(a.broadcast(refreshMsg{}), scheduleRefresh())
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, a.broadcast(tea.WindowSizeMsg{Width: msg.Width, Height: msg.Height - barRows})
	case tea.KeyPressMsg:
		if k := msg.String(); k == "ctrl+c" || k == quitKey || k == "q" && !a.slots[a.active].m.typing() {
			return a, tea.Quit
		}
		if msg.String() == refreshKey {
			return a, a.broadcast(refreshMsg{})
		}
		if a.open(msg.String()) {
			return a, nil
		}
	case tea.MouseClickMsg:
		if msg.Y == a.height-1 {
			x := barPad
			for _, it := range a.barItems() {
				w := ansi.StringWidth(barLabel(it))
				if msg.X >= x && msg.X < x+w {
					switch it.key {
					case quitKey:
						return a, tea.Quit
					case refreshKey:
						return a, a.broadcast(refreshMsg{})
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
	v.BackgroundColor, v.ForegroundColor = screenColors()
	if a.width == 0 {
		return v // size not known yet
	}
	var bar []string
	for _, it := range a.barItems() {
		key, name, _ := strings.Cut(barLabel(it), " ")
		style := barStyle
		if it.key == a.slots[a.active].key {
			style = activeStyle
		}
		bar = append(bar, fkeyStyle.Render(key)+style.Render(" "+name))
	}
	body := a.slots[a.active].m.View()
	if s, ok := a.slots[0].m.(interface{ sidebar() string }); ok && a.active != 0 && body != "" {
		// every other module opens right of home's sidebar: put it beside the module's panes, above its key bar
		i := strings.LastIndex(body, "\n")
		body = lipgloss.JoinHorizontal(lipgloss.Top, s.sidebar(), body[:i]) + body[i:]
	}
	line := strings.Repeat(" ", barPad) + strings.Join(bar, barStyle.Render(" "))
	v.SetContent(body + "\n\n" + barStyle.Width(a.width).Render(ansi.Truncate(line, a.width, "")))
	return v
}
