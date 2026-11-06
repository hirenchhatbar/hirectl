package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// progress carries a command's output: Write passes results to out, fail reports an error on errOut at
// once, and when errOut is a terminal a progress bar stays on its last line. It is safe for concurrent use.
type progress struct {
	mu          sync.Mutex
	out, errOut io.Writer
	tty         bool   // draw the bar
	label       string // the step under way
	done, total int    // steps; total 0 shows the label alone
}

func newProgress(out, errOut io.Writer) *progress {
	return &progress{out: out, errOut: errOut, tty: isTerminal(errOut)}
}

// isTerminal reports whether w is a character device, such as a terminal; /dev/null is one too, harmlessly.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// start begins a phase of total steps, 0 when unknown, under label.
func (p *progress) start(label string, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.label, p.done, p.total = label, 0, total
	p.draw()
}

// at names the step under way.
func (p *progress) at(label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.label = label
	p.draw()
}

// step marks a step done.
func (p *progress) step() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done++
	p.draw()
}

// fail prints err on errOut as `hirectl: <err>`.
func (p *progress) fail(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	fmt.Fprintln(p.errOut, "hirectl:", err)
	p.draw()
}

// Write writes a result to out, above the bar.
func (p *progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	defer p.draw()
	return p.out.Write(b)
}

// close removes the bar for good.
func (p *progress) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clear()
	p.tty = false
}

const barWidth = 30

// draw redraws the bar line. shortcut: a label wider than the terminal wraps and breaks the redraw; truncate to the terminal width if labels get long.
func (p *progress) draw() {
	if !p.tty {
		return
	}
	line := p.label
	if p.total > 0 {
		n := barWidth * min(p.done, p.total) / p.total
		line = fmt.Sprintf("[%s%s] %d/%d %s", strings.Repeat("=", n), strings.Repeat(" ", barWidth-n), p.done, p.total, p.label)
	}
	fmt.Fprint(p.errOut, "\r\x1b[K"+line)
}

// clear erases the bar line.
func (p *progress) clear() {
	if p.tty {
		fmt.Fprint(p.errOut, "\r\x1b[K")
	}
}
