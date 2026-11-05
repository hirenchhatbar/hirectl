package main

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const sidebarWidth = 30 // includes the border

// labelFmt pads field labels so values line up in the form and the read-only view.
const labelFmt = "%-10s"

// fieldTop is the screen row of the first form field: border, heading, blank line.
const fieldTop = 3

type mode int

const (
	modeLogo   mode = iota // right pane shows the logo
	modeView               // right pane shows the selected item read-only
	modeForm               // right pane edits an item
	modeDelete             // right pane asks to confirm a delete
)

var (
	paneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	selStyle  = lipgloss.NewStyle().Reverse(true)
	offStyle  = lipgloss.NewStyle().Faint(true)
	headStyle = lipgloss.NewStyle().Bold(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F"))
)

// record is an item a list module manages, such as Feed or Source.
type record[T any] interface {
	label() string    // sidebar and delete-prompt text
	values() []string // field values in kind.fields order
	on() bool
	withOn(bool) T
}

// kind describes one record type to the list module.
type kind[T record[T]] struct {
	name   string // "Feed"
	file   string // JSON file in dataDir
	fields []string
	hints  []string                  // input placeholders, by field
	blank  T                         // defaults for a new item
	parse  func([]string) (T, error) // form values to a validated item
}

// listModel is a module: items in a sidebar, right pane for logo/view/form/delete.
type listModel[T record[T]] struct {
	kind          kind[T]
	path          string
	items         []T
	cursor        int // selected item
	offset        int // first visible sidebar row
	mode          mode
	editing       int // item index being edited, -1 for a new item
	inputs        []textinput.Model
	focus         int
	status        string // error shown in the bottom bar until the next key
	width, height int
}

func newListModel[T record[T]](k kind[T], path string, items []T) listModel[T] {
	m := listModel[T]{kind: k, path: path, items: items}
	for i, name := range k.fields {
		in := textinput.New()
		in.Prompt = fmt.Sprintf(labelFmt, name)
		in.Placeholder = k.hints[i]
		m.inputs = append(m.inputs, in)
	}
	return m
}

// listFocused reports whether keys and clicks drive the sidebar.
func (m listModel[T]) listFocused() bool { return m.mode == modeLogo || m.mode == modeView }

// listRows is how many items fit in the sidebar.
func (m listModel[T]) listRows() int { return max(m.height-3, 1) }

// commit saves items and only then makes them current, so a failed save loses nothing on disk.
func (m *listModel[T]) commit(items []T) bool {
	if err := saveJSON(m.path, items); err != nil {
		m.status = "save failed: " + err.Error()
		return false
	}
	m.items = items
	m.cursor = min(m.cursor, max(len(items)-1, 0))
	return true
}

func (m *listModel[T]) move(to int) {
	m.cursor = max(min(to, len(m.items)-1), 0)
	m.offset = max(min(m.offset, m.cursor), m.cursor-m.listRows()+1)
}

func (m *listModel[T]) openForm(index int) tea.Cmd {
	m.mode, m.editing, m.focus = modeForm, index, 0
	it := m.kind.blank
	if index >= 0 {
		it = m.items[index]
	}
	for i, v := range it.values() {
		m.inputs[i].SetValue(v)
		m.inputs[i].Blur()
	}
	return m.inputs[0].Focus()
}

func (m *listModel[T]) focusField(i int) tea.Cmd {
	m.inputs[m.focus].Blur()
	m.focus = (i + len(m.inputs)) % len(m.inputs)
	return m.inputs[m.focus].Focus()
}

func (m *listModel[T]) saveForm() {
	var vals []string
	for _, in := range m.inputs {
		vals = append(vals, in.Value())
	}
	it, err := m.kind.parse(vals)
	if err != nil {
		m.status = err.Error()
		return
	}
	items := slices.Clone(m.items)
	if m.editing >= 0 {
		items[m.editing] = it.withOn(items[m.editing].on())
	} else {
		items = append(items, it)
	}
	if m.commit(items) {
		m.mode = modeView
		if m.editing < 0 {
			m.move(len(items) - 1)
		}
	}
}

func (m listModel[T]) Update(msg tea.Msg) (module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		for i := range m.inputs {
			m.inputs[i].SetWidth(max(m.width-sidebarWidth-14, 10))
		}
		m.move(m.cursor)
		return m, nil
	case tea.KeyPressMsg:
		m.status = ""
		switch m.mode {
		case modeForm:
			return m.updateForm(msg)
		case modeDelete:
			if msg.String() == "y" {
				if m.commit(slices.Delete(slices.Clone(m.items), m.cursor, m.cursor+1)) {
					m.move(m.cursor)
				}
			}
			m.mode = modeLogo
			return m, nil
		}
		return m.updateList(msg)
	case tea.MouseClickMsg:
		return m.click(msg)
	case tea.MouseWheelMsg:
		if m.listFocused() {
			switch msg.Button {
			case tea.MouseWheelUp:
				m.move(m.cursor - 1)
			case tea.MouseWheelDown:
				m.move(m.cursor + 1)
			}
		}
		return m, nil
	}
	if m.mode == modeForm { // cursor blink and other input messages
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m listModel[T]) updateList(msg tea.KeyPressMsg) (module, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "j", "down":
		m.move(m.cursor + 1)
	case "k", "up":
		m.move(m.cursor - 1)
	case "g", "home":
		m.move(0)
	case "G", "end":
		m.move(len(m.items) - 1)
	case "a":
		return m, m.openForm(-1)
	case "l", "right", "enter":
		if len(m.items) > 0 {
			m.mode = modeView
		}
	case "h", "left", "esc":
		m.mode = modeLogo
	case "e":
		if len(m.items) > 0 {
			return m, m.openForm(m.cursor)
		}
	case "space":
		if len(m.items) > 0 {
			items := slices.Clone(m.items)
			items[m.cursor] = items[m.cursor].withOn(!items[m.cursor].on())
			m.commit(items)
		}
	case "d":
		if len(m.items) > 0 {
			m.mode = modeDelete
		}
	}
	return m, nil
}

func (m listModel[T]) updateForm(msg tea.KeyPressMsg) (module, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeLogo
		if m.editing >= 0 {
			m.mode = modeView
		}
		return m, nil
	case "enter", "ctrl+s":
		m.saveForm()
		return m, nil
	case "tab", "down":
		return m, m.focusField(m.focus + 1)
	case "shift+tab", "up":
		return m, m.focusField(m.focus - 1)
	}
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m listModel[T]) click(msg tea.MouseClickMsg) (module, tea.Cmd) {
	if msg.Button != tea.MouseLeft {
		return m, nil
	}
	if msg.X < sidebarWidth && m.listFocused() {
		row := m.offset + msg.Y - 1
		if msg.Y < 1 || msg.Y > m.listRows() || row >= len(m.items) {
			return m, nil
		}
		if row == m.cursor { // clicking the selected item opens it
			m.mode = modeView
			return m, nil
		}
		m.move(row)
		return m, nil
	}
	if field := msg.Y - fieldTop; msg.X >= sidebarWidth && m.mode == modeForm && field >= 0 && field < len(m.inputs) {
		return m, m.focusField(field)
	}
	return m, nil
}

func (m listModel[T]) View() string {
	if m.width == 0 {
		return "" // size not known yet
	}
	h := m.height - 1 // last row is the key bar
	rightW := max(m.width-sidebarWidth, 20)
	inner := rightW - 2

	var rows []string
	for i := m.offset; i < len(m.items) && i < m.offset+m.listRows(); i++ {
		it := m.items[i]
		box := "[ ] "
		if it.on() {
			box = "[x] "
		}
		row := ansi.Truncate(box+it.label(), sidebarWidth-2, "…")
		row += strings.Repeat(" ", sidebarWidth-2-ansi.StringWidth(row))
		switch {
		case i == m.cursor:
			row = selStyle.Render(row)
		case !it.on():
			row = offStyle.Render(row)
		}
		rows = append(rows, row)
	}
	if len(m.items) == 0 {
		rows = append(rows, offStyle.Render("no "+strings.ToLower(m.kind.name)+"s, press a"))
	}
	left := paneStyle.Width(sidebarWidth).Height(h).Render(strings.Join(rows, "\n"))

	var body, keys string
	switch m.mode {
	case modeForm:
		heading := "Add "
		if m.editing >= 0 {
			heading = "Edit "
		}
		lines := []string{headStyle.Render(heading + strings.ToLower(m.kind.name)), ""}
		for _, in := range m.inputs {
			lines = append(lines, in.View())
		}
		body = strings.Join(lines, "\n")
		keys = "tab/↑↓ field  enter/ctrl+s save  esc cancel"
	case modeView:
		it := m.items[m.cursor]
		status := "off"
		if it.on() {
			status = "on"
		}
		lines := []string{headStyle.Render(m.kind.name), ""}
		names := append(slices.Clip(m.kind.fields), "Status")
		for i, v := range append(it.values(), status) {
			lines = append(lines, fmt.Sprintf(labelFmt, names[i])+v)
		}
		body = strings.Join(lines, "\n")
		keys = "e edit  space on/off  d delete  j/k move  esc/← back  q quit"
	case modeDelete:
		body = lipgloss.Place(inner, h-2, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("Delete %q?  y/n", m.items[m.cursor].label()))
		keys = "y delete  any other key cancel"
	default:
		body = logoView(inner, h-2)
		keys = "j/k move  →/enter view  a add  e edit  space on/off  d delete  q quit"
	}
	right := paneStyle.Width(rightW).Height(h).Render(body)

	bar := " " + keys
	if m.status != "" {
		bar = " " + errStyle.Render(m.status)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right) + "\n" + ansi.Truncate(bar, m.width, "…")
}
