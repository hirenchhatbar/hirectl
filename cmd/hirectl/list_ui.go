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

// record is an item a list module manages, such as Feed.
type record[T any] interface {
	label() string    // sidebar and delete-prompt text
	key() string      // unique among a list's items
	values() []string // field values in kind.fields order
	on() bool
	withOn(bool) T
}

// pick is a value the form offers as a checkbox for its last field, a comma list.
type pick struct {
	value  string // e.g. greenhouse:stripe
	note   string // column after value, e.g. "4 of 120 jobs"
	tags   string // last column, e.g. "Go, Java"
	detail string // shown under the list while this pick is selected
	on     bool   // checked when the checklist opens on an empty last field
}

// kind describes one record type to the list module.
type kind[T record[T]] struct {
	name       string // "Feed"
	file       string // JSON file in dataDir
	fields     []string
	hints      []string                        // input placeholders, by field
	blank      T                               // defaults for a new item
	parse      func([]string) (T, error)       // form values to a validated item
	picks      func([]string) (string, []pick) // optional: checklist heading and picks for the last field, from the form values
	derive     func([]string)                  // optional: sets form fields computed from others, e.g. feed ID from title
	deriveHint string                          // ctrl+g key hint for derive, e.g. "ID from title"
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
	focus         int    // input index, or len(inputs) for the picks checklist
	picks         []pick // loaded when the checklist gets focus
	picksHead     string // heading of picks
	pickAt        int    // selected pick
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
	m.mode, m.editing, m.focus, m.picks = modeForm, index, 0, nil
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

// stepFocus moves focus d stops on, wrapping around. The picks checklist, if kind has one, comes right
// before the last field it fills, so tabbing to that field opens it first.
func (m *listModel[T]) stepFocus(d int) tea.Cmd {
	var stops []int
	for i := range m.inputs {
		if i == len(m.inputs)-1 && m.kind.picks != nil {
			stops = append(stops, len(m.inputs))
		}
		stops = append(stops, i)
	}
	at := slices.Index(stops, m.focus)
	return m.focusField(stops[(at+d+len(stops))%len(stops)])
}

// focusField focuses input i, or the picks checklist for i == len(inputs).
func (m *listModel[T]) focusField(i int) tea.Cmd {
	if !m.onPicks() {
		m.inputs[m.focus].Blur()
	}
	m.focus = i
	if m.onPicks() { // reload, as the other fields may have changed
		m.picksHead, m.picks = m.kind.picks(m.formValues())
		m.pickAt = 0
		if last := &m.inputs[len(m.inputs)-1]; last.Value() == "" {
			var on []string
			for _, p := range m.picks {
				if p.on {
					on = append(on, p.value)
				}
			}
			last.SetValue(strings.Join(on, ", "))
		}
		return nil
	}
	return m.inputs[m.focus].Focus()
}

func (m listModel[T]) onPicks() bool { return m.focus == len(m.inputs) }

// togglePick adds v to the last field's comma list, or removes it when there.
func (m *listModel[T]) togglePick(v string) {
	last := &m.inputs[len(m.inputs)-1]
	vals := parseList(last.Value())
	if i := slices.Index(vals, v); i >= 0 {
		vals = slices.Delete(vals, i, i+1)
	} else {
		vals = append(vals, v)
	}
	last.SetValue(strings.Join(vals, ", "))
}

func (m listModel[T]) formValues() []string {
	var vals []string
	for _, in := range m.inputs {
		vals = append(vals, in.Value())
	}
	return vals
}

func (m *listModel[T]) saveForm() {
	it, err := m.kind.parse(m.formValues())
	if err != nil {
		m.status = err.Error()
		return
	}
	for i, o := range m.items {
		if i != m.editing && o.key() == it.key() {
			m.status = fmt.Sprintf("%s %q already exists", strings.ToLower(m.kind.name), it.key())
			return
		}
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
	if m.mode == modeForm && !m.onPicks() { // cursor blink and other input messages
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
	case "ctrl+g":
		if m.kind.derive != nil {
			v := m.formValues()
			m.kind.derive(v)
			m.setForm(v)
		}
		return m, nil
	case "tab":
		return m, m.stepFocus(1)
	case "shift+tab":
		return m, m.stepFocus(-1)
	}
	if m.onPicks() {
		switch msg.String() {
		case "j", "down":
			m.pickAt = max(min(m.pickAt+1, len(m.picks)-1), 0)
		case "k", "up":
			m.pickAt = max(m.pickAt-1, 0)
		case "space":
			if len(m.picks) > 0 {
				m.togglePick(m.picks[m.pickAt].value)
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "down":
		return m, m.stepFocus(1)
	case "up":
		return m, m.stepFocus(-1)
	}
	old := m.formValues()
	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	m.follow(old)
	return m, cmd
}

// follow re-derives a new item's derived fields after a keystroke, while they still hold what old derives to
// and the field typed in is not one of them; editing keeps them, as files may refer to them (feed ID).
func (m *listModel[T]) follow(old []string) {
	if m.kind.derive == nil || m.editing >= 0 {
		return
	}
	was := slices.Clone(old)
	m.kind.derive(was)
	cur := m.formValues()
	typed := cur[m.focus]
	m.kind.derive(cur)
	if slices.Equal(was, old) && cur[m.focus] == typed {
		m.setForm(cur)
	}
}

// setForm puts v into the form's inputs, leaving unchanged ones (and their cursors) alone.
func (m *listModel[T]) setForm(v []string) {
	for i := range m.inputs {
		if v[i] != m.inputs[i].Value() {
			m.inputs[i].SetValue(v[i])
		}
	}
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
		if m.kind.picks != nil {
			lines = append(lines, "")
			lines = append(lines, m.picksView(inner, h-2-len(lines))...)
		}
		body = strings.Join(lines, "\n")
		keys = "tab/↑↓ field  enter/ctrl+s save  esc cancel"
		if m.onPicks() {
			keys = "j/k move  space toggle  tab field  enter/ctrl+s save  esc cancel"
		}
		if m.kind.derive != nil {
			keys = "ctrl+g " + m.kind.deriveHint + "  " + keys
		}
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

// picksView renders the picks checklist in at most rows lines of width: heading, aligned rows scrolled to
// keep the selection in view, then the selected pick's detail.
func (m listModel[T]) picksView(width, rows int) []string {
	if !m.onPicks() {
		return []string{headStyle.Render("Suggested boards") + offStyle.Render("  tab from Location to review")}
	}
	lines := []string{ansi.Truncate(headStyle.Render(m.picksHead), width, "…")}
	if len(m.picks) == 0 {
		return append(lines, offStyle.Render("No discovered boards. Run hirectl discover."))
	}
	valW, noteW := 0, 0
	for _, p := range m.picks {
		valW, noteW = max(valW, ansi.StringWidth(p.value)), max(noteW, ansi.StringWidth(p.note))
	}
	rows = max(rows-3, 1) // heading, blank, detail
	chosen := parseList(m.inputs[len(m.inputs)-1].Value())
	start := max(m.pickAt-rows+1, 0)
	for i := start; i < len(m.picks) && i < start+rows; i++ {
		p := m.picks[i]
		box := "[ ] "
		if slices.Contains(chosen, p.value) {
			box = "[x] "
		}
		row := fmt.Sprintf("%s%-*s  %*s  ", box, valW, p.value, noteW, p.note)
		if i == m.pickAt {
			row = selStyle.Render(ansi.Truncate(row+p.tags, width, "…"))
		} else {
			row = ansi.Truncate(row+offStyle.Render(p.tags), width, "…")
		}
		lines = append(lines, row)
	}
	return append(lines, "", offStyle.Render(ansi.Truncate(m.picks[m.pickAt].detail, width, "…")))
}
