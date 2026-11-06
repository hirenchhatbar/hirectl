package main

import (
	"cmp"
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// jobSorts are the jobs pane's orders, cycled with S; the first is the default.
var jobSorts = []jobSort{
	{"newest", newestFirst},
	{"board", func(a, b Job) int { return strings.Compare(a.Board, b.Board) }},
	{"title", func(a, b Job) int { return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)) }},
	{"score", func(a, b Job) int { return cmp.Compare(b.Score, a.Score) }},
	{"level", func(a, b Job) int { return cmp.Compare(levelRank(a.Level), levelRank(b.Level)) }},
}

// levelRank orders levels fewest years first, unknown last.
func levelRank(name string) int {
	if i := slices.IndexFunc(levels, func(l jobLevel) bool { return l.name == name }); i >= 0 {
		return i
	}
	return len(levels)
}

type jobSort struct {
	name string
	cmp  func(a, b Job) int
}

// jobAges are the jobs pane's posted-since filters, cycled with d; 0 keeps all.
var jobAges = []time.Duration{0, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}

// jobScores are the jobs pane's least-score filters, cycled with r: 3 is a skill named often or a tech
// tagged, 10 a skill in the title.
var jobScores = []int{0, 3, 10}

// jobThumbs are the jobs pane's thumb filters, cycled with t: any, liked, disliked, unrated.
var jobThumbs = []string{"", "up", "down", "none"}

// thumbIcons show a job's thumb, by its likes.json value.
var thumbIcons = map[string]string{"up": "👍", "down": "👎", "": "  "}

// likesFile keeps each job's thumb, "up" or "down", by job URL, across feeds; unrated jobs are absent.
const likesFile = "likes.json"

// The jobs pane's text inputs, by index.
const (
	searchInput = iota
	locationInput
)

// jobViewsFile keeps each feed's jobs pane settings, by feed ID.
// shortcut: entries of deleted or renamed feeds stay, and a new feed taking an old ID inherits its settings;
// prune them if that ever matters.
const jobViewsFile = "jobviews.json"

// jobView is a jobs pane's search, filters and sort, saved per feed and applied when it opens again.
type jobView struct {
	Search   string   `json:"search,omitempty"`
	Location string   `json:"location,omitempty"`
	Sources  []string `json:"sources,omitempty"` // checked sources, none for all
	Days     int      `json:"days,omitempty"`    // posted within, 0 for any
	Score    int      `json:"score,omitempty"`   // least score, a jobScores value
	Level    string   `json:"level,omitempty"`   // a levels name, blank for any
	Thumb    string   `json:"thumb,omitempty"`   // a jobThumbs value, blank for any
	Sort     string   `json:"sort,omitempty"`    // a jobSorts name, blank for the first
}

// jobsHead and jobsFoot are the jobs pane's lines above and below the job rows.
const jobsHead, jobsFoot = 5, 2

// jobsPane shows one feed's stored jobs in the right pane: fuzzy search, filters and sorting.
type jobsPane struct {
	title      string
	all, shown []Job
	sources    []string                      // each source the jobs come from, sorted
	checked    []string                      // sources to show, in sources order; none shows all
	picking    bool                          // the sources checklist is open
	pickAt     int                           // selected checklist row
	wasChecked []string                      // checked before the checklist opened, restored by esc
	age, sort  int                           // indexes into jobAges and jobSorts
	score      int                           // index into jobScores
	level      int                           // levels index plus 1, 0 for any
	thumb      int                           // index into jobThumbs
	likes      map[string]string             // thumbs by job URL, as in likesFile
	rate       func(url, thumb string) error // optional: saves a job's thumb, "" to clear it
	snap       string                        // URL of the job shown in the snapshot, "" when closed
	snapTop    int                           // first description line shown
	snapErr    string                        // why the snapshot's description failed to load
	loading    string                        // URL whose description is being fetched, "" for none
	descs      map[string]string             // fetched descriptions by job URL, plain text
	spin       spinner.Model
	inputs     []textinput.Model
	typing     int     // input being typed in, -1 for none
	saved      jobView // settings as last saved, so only changes are written
	before     string  // the typed input's value before typing, restored by esc
	cursor     int
	offset     int
	width      int
	height     int
}

func newJobsPane(title string, jobs []Job, v jobView, likes map[string]string, width, height int) jobsPane {
	if likes == nil {
		likes = map[string]string{}
	}
	p := jobsPane{title: title, all: jobs, typing: -1, likes: likes, descs: map[string]string{}, spin: spinner.New(spinner.WithSpinner(spinner.Dot))}
	for _, j := range jobs {
		if src, _, _ := strings.Cut(j.Board, ":"); !slices.Contains(p.sources, src) {
			p.sources = append(p.sources, src)
		}
	}
	slices.Sort(p.sources)
	for _, in := range [][2]string{{"Search", "/ to type: title, board, location"}, {"Location", "l to type, e.g. Remote India, Berlin"}} {
		ti := textinput.New()
		ti.Prompt = fmt.Sprintf(labelFmt, in[0])
		ti.Placeholder = in[1]
		p.inputs = append(p.inputs, ti)
	}
	p.inputs[searchInput].SetValue(v.Search)
	p.inputs[locationInput].SetValue(v.Location)
	p.checked = slices.DeleteFunc(slices.Clone(p.sources), func(s string) bool { return !slices.Contains(v.Sources, s) }) // sources gone from the jobs drop out
	p.age = max(slices.Index(jobAges, time.Duration(v.Days)*24*time.Hour), 0)
	p.score = max(slices.Index(jobScores, v.Score), 0)
	p.level = (levelRank(v.Level) + 1) % (len(levels) + 1)
	p.thumb = max(slices.Index(jobThumbs, v.Thumb), 0)
	p.sort = max(slices.IndexFunc(jobSorts, func(s jobSort) bool { return s.name == v.Sort }), 0)
	p.saved = p.settings()
	p.resize(width, height)
	p.refresh()
	return p
}

// settings is the pane's current search, filters and sort.
func (p jobsPane) settings() jobView {
	return jobView{
		Search:   p.inputs[searchInput].Value(),
		Location: p.inputs[locationInput].Value(),
		Sources:  p.checked,
		Days:     int(jobAges[p.age] / (24 * time.Hour)),
		Score:    jobScores[p.score],
		Level:    p.levelName(),
		Thumb:    jobThumbs[p.thumb],
		Sort:     jobSorts[p.sort].name,
	}
}

// levelName is the level filter's level, blank for any.
func (p jobsPane) levelName() string {
	if p.level == 0 {
		return ""
	}
	return levels[p.level-1].name
}

// resize fits the pane in width×height cells.
func (p *jobsPane) resize(width, height int) {
	p.width, p.height = width, height
	for i := range p.inputs {
		p.inputs[i].SetWidth(max(width-12, 10))
	}
	p.move(p.cursor)
}

func (p jobsPane) rows() int { return max(p.height-jobsHead-jobsFoot, 1) }

func (p *jobsPane) move(to int) {
	p.cursor = max(min(to, len(p.shown)-1), 0)
	p.offset = max(min(p.offset, p.cursor), p.cursor-p.rows()+1, 0)
}

// refresh recomputes the shown jobs from the filters, best search match first, then by the sort.
func (p *jobsPane) refresh() {
	jobs := postedSince(slices.Clone(p.all), jobAges[p.age])
	locs := parseList(p.inputs[locationInput].Value())
	jobs = slices.DeleteFunc(jobs, func(j Job) bool {
		src, _, _ := strings.Cut(j.Board, ":")
		return len(p.checked) > 0 && !slices.Contains(p.checked, src) || j.Score < jobScores[p.score] ||
			p.level > 0 && j.Level != "" && j.Level != p.levelName() || // unknown levels stay
			p.thumb > 0 && cmp.Or(p.likes[j.URL], "none") != jobThumbs[p.thumb] ||
			len(locs) > 0 && !slices.ContainsFunc(locs, func(l string) bool { return inLocation(j.Location, l) })
	})
	type hit struct {
		Job
		score int
	}
	query := strings.Fields(p.inputs[searchInput].Value())
	var hits []hit
	for _, j := range jobs {
		if s, ok := fuzzy(query, j.Title+" "+j.Board+" "+j.Location); ok {
			hits = append(hits, hit{j, s})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return cmp.Or(cmp.Compare(b.score, a.score), jobSorts[p.sort].cmp(a.Job, b.Job)) })
	p.shown = nil
	for _, h := range hits {
		p.shown = append(p.shown, h.Job)
	}
	p.move(p.cursor)
}

// fuzzy scores how well each word of query matches text: its letters in order, ignoring case, scoring more
// for letters in a row and at word starts. ok is false when a word does not match; no words match all.
func fuzzy(query []string, text string) (score int, ok bool) {
	t := []rune(strings.ToLower(text))
	for _, w := range query {
		i, prev := 0, -2
		for _, r := range strings.ToLower(w) {
			for i < len(t) && t[i] != r {
				i++
			}
			if i == len(t) {
				return 0, false
			}
			score++
			if i == prev+1 {
				score += 5
			}
			if i == 0 || isSep(t[i-1]) {
				score += 3
			}
			prev = i
			i++
		}
	}
	return score, true
}

// count is how many jobs the filters show and how many there are.
func (p jobsPane) count() [2]int { return [2]int{len(p.shown), len(p.all)} }

// busy reports whether an input, the sources checklist or the snapshot has the keys, so esc is the pane's own.
func (p jobsPane) busy() bool { return p.typing >= 0 || p.picking || p.snap != "" }

// rateJob sets the thumb of the job at url, or clears it when it already has that thumb.
func (p *jobsPane) rateJob(url, thumb string) error {
	if p.likes[url] == thumb {
		thumb = ""
	}
	if p.rate != nil {
		if err := p.rate(url, thumb); err != nil {
			return fmt.Errorf("save thumb: %w", err)
		}
	}
	if thumb == "" {
		delete(p.likes, url)
	} else {
		p.likes[url] = thumb
	}
	p.refresh()
	return nil
}

// thumbKeys are the keys that rate a job, to their thumb.
var thumbKeys = map[string]string{"+": "up", "-": "down"}

// snapJob is the job shown in the snapshot, found by URL so a list change keeps it.
func (p jobsPane) snapJob() Job {
	i := slices.IndexFunc(p.all, func(j Job) bool { return j.URL == p.snap })
	if i < 0 {
		return Job{URL: p.snap}
	}
	return p.all[i]
}

// openSnap shows the selected job's snapshot, fetching its description unless fetched before.
func (p *jobsPane) openSnap() tea.Cmd {
	if len(p.shown) == 0 {
		return nil
	}
	j := p.shown[p.cursor]
	p.snap, p.snapTop, p.snapErr = j.URL, 0, ""
	if _, ok := p.descs[j.URL]; ok || p.loading == j.URL {
		return nil
	}
	p.loading = j.URL
	return tea.Batch(func() tea.Msg {
		descs, err := describe(context.Background(), j)
		return descsMsg{j.URL, descs, err}
	}, p.spin.Tick)
}

// descsMsg carries the descriptions fetched for the job at url: its own, and those of the other jobs on
// its board fetched with it.
type descsMsg struct {
	url   string
	descs map[string]string
	err   error
}

// describe fetches job's description, with those of the other jobs on its board when the site gives
// them in one go, by URL. A var, so tests stub it.
var describe = func(ctx context.Context, j Job) (map[string]string, error) {
	name, company, _ := splitBoard(j.Board)
	in := integrations[name]
	if in == nil {
		return nil, fmt.Errorf("unknown integration %q", name)
	}
	client := &http.Client{Timeout: requestTimeout}
	if in.describe != nil {
		text, err := in.describe(ctx, client, company, j.URL)
		return map[string]string{j.URL: text}, err
	}
	jobs, err := in.fetch(ctx, client, company, true)
	descs := map[string]string{}
	for _, b := range jobs {
		descs[b.URL] = b.text
	}
	return descs, err
}

// Tags and their line breaks, for plainText.
var (
	tagRe   = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)[^>]*>`)
	blankRe = regexp.MustCompile(`\n[ \t]*(\n[ \t]*)+`)
	breaks  = map[string]string{"br": "\n", "/p": "\n\n", "/div": "\n", "/li": "\n", "li": "• ", "/ul": "\n", "/ol": "\n", "/tr": "\n",
		"/h1": "\n\n", "/h2": "\n\n", "/h3": "\n\n", "/h4": "\n\n", "/h5": "\n\n", "/h6": "\n\n"}
)

// plainText turns a job description, HTML or plain, into text: tags dropped, block ends as line breaks,
// entities decoded, runs of blank lines as one.
func plainText(s string) string {
	s = tagRe.ReplaceAllStringFunc(s, func(t string) string {
		m := tagRe.FindStringSubmatch(t)
		return breaks[m[1]+strings.ToLower(m[2])]
	})
	s = strings.ReplaceAll(html.UnescapeString(s), "\u00a0", " ")
	return strings.TrimSpace(blankRe.ReplaceAllString(s, "\n\n"))
}

// toggle checks source s, or unchecks it when checked, keeping checked in sources order.
func (p *jobsPane) toggle(s string) {
	was := p.checked
	p.checked = slices.DeleteFunc(slices.Clone(p.sources), func(x string) bool { return slices.Contains(was, x) == (x == s) })
}

// update handles a key; esc while not busy is the list module's, closing the pane.
func (p jobsPane) update(msg tea.KeyPressMsg) (jobsPane, tea.Cmd, error) {
	if p.snap != "" {
		switch k := msg.String(); k {
		case "j", "down":
			p.snapTop = min(p.snapTop+1, max(len(p.snapLines())-p.snapRows(), 0))
		case "k", "up":
			p.snapTop = max(p.snapTop-1, 0)
		case "+", "-":
			return p, nil, p.rateJob(p.snap, thumbKeys[k])
		case "enter":
			return p, nil, openURL(p.snap)
		case "space", "esc", "left", "h":
			p.snap = ""
		}
		return p, nil, nil
	}
	if p.picking {
		switch msg.String() {
		case "j", "down":
			p.pickAt = min(p.pickAt+1, len(p.sources)-1)
		case "k", "up":
			p.pickAt = max(p.pickAt-1, 0)
		case "space":
			if len(p.sources) > 0 {
				p.toggle(p.sources[p.pickAt])
			}
		case "esc":
			p.checked = p.wasChecked
			fallthrough
		case "enter":
			p.picking = false
		}
		p.refresh()
		return p, nil, nil
	}
	if p.typing >= 0 {
		in := &p.inputs[p.typing]
		switch msg.String() {
		case "esc":
			in.SetValue(p.before)
			fallthrough
		case "enter":
			in.Blur()
			p.typing = -1
			p.refresh()
			return p, nil, nil
		}
		var cmd tea.Cmd
		*in, cmd = in.Update(msg)
		p.refresh()
		return p, cmd, nil
	}
	switch msg.String() {
	case "j", "down":
		p.move(p.cursor + 1)
	case "k", "up":
		p.move(p.cursor - 1)
	case "g", "home":
		p.move(0)
	case "G", "end":
		p.move(len(p.shown) - 1)
	case "/", "l":
		p.typing = searchInput
		if msg.String() == "l" {
			p.typing = locationInput
		}
		p.before = p.inputs[p.typing].Value()
		return p, p.inputs[p.typing].Focus(), nil
	case "s":
		p.picking, p.pickAt, p.wasChecked = true, 0, p.checked
	case "d":
		p.age = (p.age + 1) % len(jobAges)
	case "r":
		p.score = (p.score + 1) % len(jobScores)
	case "y":
		p.level = (p.level + 1) % (len(levels) + 1)
	case "S":
		p.sort = (p.sort + 1) % len(jobSorts)
	case "t":
		p.thumb = (p.thumb + 1) % len(jobThumbs)
	case "+", "-":
		if len(p.shown) > 0 {
			return p, nil, p.rateJob(p.shown[p.cursor].URL, thumbKeys[msg.String()])
		}
		return p, nil, nil
	case "space":
		return p, p.openSnap(), nil
	case "c": // clear search, filters and sort
		saved, rate, descs := p.saved, p.rate, p.descs
		p = newJobsPane(p.title, p.all, jobView{}, p.likes, p.width, p.height)
		p.saved, p.rate, p.descs = saved, rate, descs
		return p, nil, nil
	case "enter":
		if len(p.shown) > 0 {
			return p, nil, openURL(p.shown[p.cursor].URL)
		}
		return p, nil, nil
	default:
		return p, nil, nil
	}
	p.refresh()
	return p, nil, nil
}

// tick takes fetched descriptions and spinner ticks, and passes other non-key messages, such as cursor
// blink, to the input being typed in.
func (p jobsPane) tick(msg tea.Msg) (jobsPane, tea.Cmd) {
	switch msg := msg.(type) {
	case descsMsg:
		for u, d := range msg.descs {
			p.descs[u] = plainText(d)
		}
		if msg.url == p.loading {
			p.loading = ""
		}
		_, got := msg.descs[msg.url]
		switch {
		case msg.url != p.snap:
		case msg.err != nil:
			p.snapErr = msg.err.Error()
		case !got:
			p.snapErr = "no longer on its board"
		}
		return p, nil
	case spinner.TickMsg:
		if p.loading == "" {
			return p, nil // stops the spinner
		}
		var cmd tea.Cmd
		p.spin, cmd = p.spin.Update(msg)
		return p, cmd
	}
	if p.typing < 0 {
		return p, nil
	}
	var cmd tea.Cmd
	p.inputs[p.typing], cmd = p.inputs[p.typing].Update(msg)
	return p, cmd
}

// view renders the pane and its key bar.
func (p jobsPane) view() (body, keys string) {
	age := "any"
	if d := jobAges[p.age]; d > 0 {
		age = fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	sources := "all"
	if len(p.checked) > 0 {
		sources = strings.Join(p.checked, ", ")
	}
	score := "any"
	if n := jobScores[p.score]; n > 0 {
		score = fmt.Sprintf("%d+", n)
	}
	level := cmp.Or(p.levelName(), "any")
	thumb := cmp.Or(thumbIcons[jobThumbs[p.thumb]], jobThumbs[p.thumb])
	if p.thumb == 0 {
		thumb = "any"
	}
	lines := []string{
		headStyle.Render("Jobs: "+p.title) + offStyle.Render(fmt.Sprintf("  %d of %d", len(p.shown), len(p.all))),
		p.inputs[searchInput].View(),
		p.inputs[locationInput].View(),
		fmt.Sprintf(labelFmt+"%s   Posted %s   Score %s   YOE %s   Thumb %s   Sort %s", "Sources", sources, age, score, level, thumb, jobSorts[p.sort].name),
		"",
	}
	if p.picking {
		return strings.Join(append(lines, p.picksView()...), "\n"), "j/k move  space toggle  enter keep  esc undo  none checked shows all  q quit"
	}
	companyW, boardW, locW := 0, 0, 0
	for _, j := range p.shown {
		companyW, boardW, locW = max(companyW, ansi.StringWidth(company(j.Board))), max(boardW, ansi.StringWidth(j.Board)), max(locW, ansi.StringWidth(j.Location))
	}
	companyW, boardW, locW = min(companyW, 20), min(boardW, 24), min(locW, 24)
	titleW := max(p.width-10-3-3-2-companyW-boardW-locW-14, 10)
	for i := p.offset; i < len(p.shown) && i < p.offset+p.rows(); i++ {
		j := p.shown[i]
		var date string
		if !j.Posted.IsZero() {
			date = j.Posted.Format(time.DateOnly)
		}
		row := ansi.Truncate(cell(date, 10)+"  "+fmt.Sprintf("%3d", j.Score)+"  "+cell(j.Level, 3)+"  "+thumbIcons[p.likes[j.URL]]+"  "+cell(j.Title, titleW)+"  "+cell(company(j.Board), companyW)+"  "+cell(j.Board, boardW)+"  "+cell(j.Location, locW), p.width, "")
		if i == p.cursor {
			row = selStyle.Render(row)
		}
		lines = append(lines, row)
	}
	switch {
	case len(p.all) == 0:
		lines = append(lines, offStyle.Render("no jobs stored, hirectl run fetches them"))
	case len(p.shown) == 0:
		lines = append(lines, offStyle.Render("no jobs match"))
	default:
		lines = append(lines, slices.Repeat([]string{""}, p.rows()-min(len(p.shown)-p.offset, p.rows())+1)...)
		lines = append(lines, offStyle.Render(ansi.Truncate(p.shown[p.cursor].URL, p.width, "…")))
	}
	keys = "space snapshot  +/- 👍/👎  / search  l location  s sources  d posted  r score  y yoe  t thumb  S sort  c clear  enter open  j/k move  ← sidebar  esc back  q quit"
	if p.typing >= 0 {
		keys = "type to filter  enter keep  esc undo"
	}
	if p.snap != "" {
		return p.snapView(lines), "j/k scroll  +/- 👍/👎  enter open  space/esc/← close  q quit"
	}
	return strings.Join(lines, "\n"), keys
}

// snapRows is how many description lines the snapshot shows: the pane less the box's margins, border
// and job lines.
func (p jobsPane) snapRows() int { return max(p.height-2-2-snapHead, 1) }

// snapHead is the snapshot's lines above the description.
const snapHead = 5

// snapWidth is the width inside the snapshot box's border and padding.
func (p jobsPane) snapWidth() int { return max(p.width-4-4, 10) }

// snapLines is the snapshot's description, wrapped to its width, or a line on its loading or failure.
func (p jobsPane) snapLines() []string {
	d, ok := p.descs[p.snap]
	switch {
	case p.snapErr != "":
		return []string{errStyle.Render(p.snapErr)}
	case p.loading == p.snap:
		return []string{p.spin.View() + " fetching " + p.snapJob().Board + "…"}
	case !ok:
		return nil
	case d == "":
		return []string{offStyle.Render("no description from " + p.snapJob().Board)}
	}
	return strings.Split(lipgloss.NewStyle().Width(p.snapWidth()).Render(d), "\n")
}

// snapView draws the snapshot as a box floating over the jobs list lines, 2 cells in from each side.
func (p jobsPane) snapView(under []string) string {
	j := p.snapJob()
	w := p.snapWidth()
	var date string
	if !j.Posted.IsZero() {
		date = j.Posted.Format(time.DateOnly)
	}
	head := []string{
		headStyle.Render(ansi.Truncate(j.Title, w, "…")),
		ansi.Truncate(strings.Join(slices.DeleteFunc([]string{company(j.Board), j.Board, j.Location}, func(s string) bool { return s == "" }), " · "), w, "…"),
		ansi.Truncate(fmt.Sprintf("Posted %s   Score %d   YOE %s   Thumb %s", cmp.Or(date, "unknown"), j.Score, cmp.Or(j.Level, "unknown"), cmp.Or(strings.TrimSpace(thumbIcons[p.likes[j.URL]]), "none")), w, ""),
		offStyle.Render(ansi.Truncate(j.URL, w, "…")),
		"",
	}
	desc := p.snapLines()
	desc = desc[min(p.snapTop, len(desc)):]
	desc = desc[:min(len(desc), p.snapRows())]
	box := strings.Split(paneStyle.Padding(0, 1).Width(w+4).Height(p.height-2).Render(strings.Join(append(head, desc...), "\n")), "\n")
	for len(under) < p.height {
		under = append(under, "")
	}
	for i, b := range box {
		if i+1 < len(under) {
			under[i+1] = ansi.Truncate(under[i+1], 2, "") + strings.Repeat(" ", 2-min(ansi.StringWidth(under[i+1]), 2)) + b + ansi.TruncateLeft(under[i+1], 2+ansi.StringWidth(b), "")
		}
	}
	return strings.Join(under[:p.height], "\n")
}

// picksView lists the sources as checkboxes with their job counts, scrolled to keep the selection in view.
func (p jobsPane) picksView() []string {
	if len(p.sources) == 0 {
		return []string{offStyle.Render("no sources, no jobs stored")}
	}
	count := map[string]int{}
	nameW := 0
	for _, j := range p.all {
		src, _, _ := strings.Cut(j.Board, ":")
		count[src]++
	}
	for _, s := range p.sources {
		nameW = max(nameW, ansi.StringWidth(s))
	}
	var lines []string
	rows := p.rows() + jobsFoot
	for i := max(p.pickAt-rows+1, 0); i < len(p.sources) && len(lines) < rows; i++ {
		s := p.sources[i]
		box := "[ ] "
		if slices.Contains(p.checked, s) {
			box = "[x] "
		}
		row := ansi.Truncate(fmt.Sprintf("%s%-*s  %d jobs", box, nameW, s, count[s]), p.width, "…")
		if i == p.pickAt {
			row = selStyle.Render(row)
		}
		lines = append(lines, row)
	}
	return lines
}

// company is a board's company: its name, or a Workday board's tenant (workday:nvidia.wd5/site is nvidia).
func company(board string) string {
	src, name, _ := strings.Cut(board, ":")
	if src == "workday" {
		name, _, _ = strings.Cut(name, ".")
	}
	return name
}

// cell truncates or pads s to w columns.
func cell(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-ansi.StringWidth(s))
}

// openURL opens a job's web page in the default browser, without waiting for it. A var, so tests stub it.
var openURL = func(u string) error {
	if p, err := url.Parse(u); err != nil || p.Scheme != "http" && p.Scheme != "https" { // URLs come from job sites
		return fmt.Errorf("not a web URL: %q", u)
	}
	name, args := "xdg-open", []string{u}
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", u}
	}
	cmd := exec.Command(name, args...) // nil stdio is /dev/null, so the browser cannot draw over the TUI
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open %s: %w", u, err)
	}
	go cmd.Wait() //nolint:errcheck // reaps the opener; its exit status is of no use
	return nil
}
