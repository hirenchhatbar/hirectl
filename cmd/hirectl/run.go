package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"time"
)

const jobsFile = "jobs.json"

// task is one feed and the boards to fetch for it.
type task struct {
	feed   Feed
	boards []string
}

// plan cross-checks the --feeds IDs and --sources names and picks each enabled feed's boards
// whose integration is selected; empty names select all.
func plan(feeds []Feed, feedIDs, sources []string) ([]task, error) {
	if err := checkSources(sources); err != nil {
		return nil, err
	}
	for _, n := range feedIDs {
		i := slices.IndexFunc(feeds, func(f Feed) bool { return f.ID == n })
		if i < 0 {
			return nil, fmt.Errorf("unknown feed %q", n)
		}
		if !feeds[i].Enabled {
			return nil, fmt.Errorf("feed %q is disabled", n)
		}
	}
	var tasks []task
	for _, f := range feeds {
		named := slices.Contains(feedIDs, f.ID)
		if !f.Enabled || len(feedIDs) > 0 && !named {
			continue
		}
		var boards []string
		for _, b := range f.Boards {
			if name, _, _ := splitBoard(b); len(sources) == 0 || slices.Contains(sources, name) {
				boards = append(boards, b)
			}
		}
		if len(boards) == 0 {
			if named {
				return nil, fmt.Errorf("feed %q has no board on the selected sources", f.ID)
			}
			continue
		}
		tasks = append(tasks, task{f, boards})
	}
	return tasks, nil
}

// runCmd is `hirectl run [--feeds a,b] [--sources greenhouse,lever] [--delay 1s]`: fetch each board,
// keep jobs matching the feed, append new ones to jobs.json and print them tab-separated.
func runCmd(ctx context.Context, dir string, args []string, out io.Writer) error {
	fs := newFlagSet("run")
	feedsFlag := fs.String("feeds", "", `comma-separated feed IDs, as shown in the TUI
e.g. --feeds go-remote,backend-nl (default: all enabled)`)
	sourcesFlag := fs.String("sources", "", sourcesUsage())
	delay := fs.Duration("delay", time.Second, "minimum wait between requests to the same integration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("run: unexpected argument %q", fs.Arg(0))
	}
	feeds, err := loadJSON[[]Feed](filepath.Join(dir, feedKind.file))
	if err != nil {
		return err
	}
	tasks, err := plan(feeds, parseList(*feedsFlag), parseList(*sourcesFlag))
	if err != nil {
		return err
	}
	jobsPath := filepath.Join(dir, jobsFile)
	stored, err := loadJSON[[]Job](jobsPath)
	if err != nil {
		return err
	}
	seen := map[[2]string]bool{}
	for _, j := range stored {
		seen[[2]string{j.Feed, j.URL}] = true
	}

	p := newPacer(&http.Client{Timeout: 30 * time.Second}, *delay)
	fetched := map[string][]Job{} // a board shared by feeds is fetched once
	var found []Job
	var errs []error
	for _, t := range tasks {
		for _, b := range t.boards {
			jobs, ok := fetched[b]
			if !ok {
				if jobs, err = p.fetch(ctx, b, true); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", b, err))
				}
				fetched[b] = jobs
			}
			for _, j := range jobs {
				key := [2]string{t.feed.ID, j.URL}
				if seen[key] || !t.feed.matches(j) {
					continue
				}
				seen[key] = true
				j.Feed, j.Board = t.feed.ID, b
				found = append(found, j)
				printJob(out, j)
			}
		}
	}
	if len(found) > 0 {
		// shortcut: jobs.json only grows, prune old jobs once it gets large
		if err := saveJSON(jobsPath, append(stored, found...)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
