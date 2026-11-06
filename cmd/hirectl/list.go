package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// jobsOpts are the `hirectl list jobs` options.
type jobsOpts struct {
	since    time.Duration // drop jobs posted longer ago; 0 keeps all
	byPosted bool          // newest first, in place of by feed and board
}

// lists maps each `hirectl list` subcommand to what it prints from dir.
var lists = map[string]func(dir string, out io.Writer, o jobsOpts) error{
	"feeds": func(dir string, out io.Writer, _ jobsOpts) error {
		feeds, err := loadJSON[[]Feed](filepath.Join(dir, feedKind.file))
		for _, f := range feeds {
			state := "enabled"
			if !f.Enabled {
				state = "disabled"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", f.ID, state, f.Title, strings.Join(f.Sources, ","))
		}
		return err
	},
	"jobs": func(dir string, out io.Writer, o jobsOpts) error {
		files, err := jobFiles(dir)
		var all []Job
		for _, p := range files {
			jobs, err := loadJSON[[]Job](p)
			if err != nil {
				return err
			}
			all = append(all, jobs...)
		}
		all = postedSince(all, o.since)
		if o.byPosted {
			slices.SortStableFunc(all, newestFirst)
		}
		for _, j := range all {
			printJob(out, j)
		}
		return err
	},
	"boards": func(dir string, out io.Writer, _ jobsOpts) error {
		jobs, err := loadJSON[[]Job](filepath.Join(dir, suggestionsFile))
		byBoard := map[string][]Job{}
		for _, j := range jobs {
			byBoard[j.Board] = append(byBoard[j.Board], j)
		}
		for _, b := range slices.Sorted(maps.Keys(byBoard)) {
			printBoard(out, b, byBoard[b])
		}
		return err
	},
	"sources": func(dir string, out io.Writer, _ jobsOpts) error {
		jobs, err := loadJSON[[]Job](filepath.Join(dir, suggestionsFile))
		boards := boardsBySource(jobs)
		for _, s := range integrationNames() {
			fmt.Fprintf(out, "%s\t%d boards\n", s, len(boards[s]))
		}
		return err
	},
}

// listCmd is `hirectl list <feeds|jobs|boards|sources> [--since 7d] [--sort posted]`: print stored data tab-separated.
func listCmd(_ context.Context, dir string, args []string, out, _ io.Writer) error {
	names := strings.Join(slices.Sorted(maps.Keys(lists)), ", ")
	var o jobsOpts
	fs := newFlagSet("list")
	options := fs.Usage
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: hirectl list <what>, what is one of: %s\n", names)
		options()
	}
	fs.Func("since", "jobs only: drop jobs posted longer ago than this, in days (7d) or a Go duration (12h); undated jobs stay", func(s string) (err error) {
		o.since, err = parseSince(s)
		return err
	})
	fs.Func("sort", "jobs only: board (default, by feed and board) or posted (newest first, undated last)", func(s string) error {
		if s != "board" && s != "posted" {
			return fmt.Errorf("want board or posted")
		}
		o.byPosted = s == "posted"
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	what := fs.Arg(0)
	if fs.NArg() > 0 { // options may follow the subcommand too
		if err := fs.Parse(fs.Args()[1:]); err != nil {
			return err
		}
	}
	if what == "" || fs.NArg() != 0 || lists[what] == nil {
		return fmt.Errorf("list: want one of %s", names)
	}
	if what != "jobs" && o != (jobsOpts{}) {
		return fmt.Errorf("list: --since and --sort apply to jobs only")
	}
	return lists[what](dir, out, o)
}

// parseSince parses a --since age: whole days as "7d", else a Go duration such as "12h".
func parseSince(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if days, ok := strings.CutSuffix(s, "d"); ok {
		var n int
		n, err = strconv.Atoi(days)
		d = time.Duration(n) * 24 * time.Hour
	}
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("want a positive age such as 7d or 12h")
	}
	return d, nil
}

// postedSince drops, in place, the jobs posted longer than since ago; undated jobs stay, as their age is
// unknown. 0 keeps all. Shared by list jobs and the TUI jobs pane.
func postedSince(jobs []Job, since time.Duration) []Job {
	if since <= 0 {
		return jobs
	}
	cut := time.Now().Add(-since)
	return slices.DeleteFunc(jobs, func(j Job) bool { return !j.Posted.IsZero() && j.Posted.Before(cut) })
}

// newestFirst orders jobs by posting date, newest first; undated jobs, the zero time, come last.
func newestFirst(a, b Job) int { return b.Posted.Compare(a.Posted) }

// printJob prints j as `feed-id  board  title  location  url  posted` (posted as 2006-01-02,
// blank when unknown), shared by run and list jobs.
func printJob(out io.Writer, j Job) {
	var date string
	if !j.Posted.IsZero() {
		date = j.Posted.Format(time.DateOnly)
	}
	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\n", j.Feed, j.Board, j.Title, j.Location, j.URL, date)
}

// printBoard prints a board, its job count and main techs, shared by discover and list boards.
func printBoard(out io.Writer, board string, jobs []Job) {
	fmt.Fprintf(out, "%s\t%d jobs\t%s\n", board, len(jobs), strings.Join(topTech(jobs, 4), ","))
}

// boardsBySource groups the boards of jobs by source, shared by list sources and cross-check.
func boardsBySource(jobs []Job) map[string]map[string]bool {
	boards := map[string]map[string]bool{}
	for _, j := range jobs {
		src, _, _ := splitBoard(j.Board)
		if boards[src] == nil {
			boards[src] = map[string]bool{}
		}
		boards[src][j.Board] = true
	}
	return boards
}
