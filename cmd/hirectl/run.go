package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const jobsDir = "jobs"

// jobFile is where run keeps a feed's matching jobs: jobs/<feed-id>.json.
func jobFile(dir, feed string) string { return filepath.Join(dir, jobsDir, feed+".json") }

// jobFiles lists every feed's job file in dir, sorted.
func jobFiles(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, jobsDir, "*.json"))
}

// prune removes everything in jobs/ but the files of feeds, such as those of deleted or renamed
// feeds, and the jobs/<feed-id>/ directories of older versions.
func prune(dir string, feeds []Feed) error {
	keep := map[string]bool{}
	for _, f := range feeds {
		keep[filepath.Base(jobFile(dir, f.ID))] = true
	}
	entries, err := os.ReadDir(filepath.Join(dir, jobsDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	errs := []error{err}
	for _, e := range entries {
		if !keep[e.Name()] {
			errs = append(errs, os.RemoveAll(filepath.Join(dir, jobsDir, e.Name())))
		}
	}
	return errors.Join(errs...)
}

// task is one feed and the boards to fetch for it.
type task struct {
	feed   Feed
	boards []string
}

// plan cross-checks the --feeds IDs and --sources names and picks each enabled feed's top n boards in
// catalog, by jobs matching the feed, on the feed's sources and the selected ones; empty names select all.
func plan(feeds []Feed, catalog []Job, n int, feedIDs, sources []string) ([]task, error) {
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
		match := matching(catalog, f)
		maps.DeleteFunc(match, func(b string, _ []Job) bool {
			name, _, _ := splitBoard(b)
			return len(sources) > 0 && !slices.Contains(sources, name)
		})
		boards := topBoards(match, n)
		if len(boards) == 0 {
			if named {
				return nil, fmt.Errorf("feed %q has no board with matching jobs on the selected sources, see hirectl list boards", f.ID)
			}
			continue
		}
		tasks = append(tasks, task{f, boards})
	}
	return tasks, nil
}

// runCmd is `hirectl run [--feeds a,b] [--sources greenhouse,lever] [--delay 1s]`: fetch each feed's top
// boards, keep jobs matching the feed in its job file and print those new to it tab-separated.
// Board errors go to errOut as they happen, with a progress bar when errOut is a terminal.
func runCmd(ctx context.Context, dir string, args []string, out, errOut io.Writer) error {
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
	catalog, err := loadJSON[[]Job](filepath.Join(dir, suggestionsFile))
	if err != nil {
		return err
	}
	if len(catalog) == 0 {
		return errors.New("no discovered boards, run hirectl discover first")
	}
	cfg, err := loadJSON[Config](filepath.Join(dir, configFile))
	if err != nil {
		return err
	}
	sources := parseList(*sourcesFlag)
	tasks, err := plan(feeds, catalog, cfg.boards(), parseList(*feedsFlag), sources)
	if err != nil {
		return err
	}
	prog := newProgress(out, errOut)
	defer prog.close()
	steps := 0
	for _, t := range tasks {
		steps += len(t.boards)
	}
	prog.start("", steps)
	p := newPacer(&http.Client{Timeout: requestTimeout}, *delay)
	fetched := map[string][]Job{} // a board shared by feeds is fetched once
	failed := map[string]bool{}
	var errs []error
	for _, t := range tasks {
		var jobs []Job
		for _, b := range t.boards {
			bj, ok := fetched[b]
			if !ok {
				prog.at(b)
				if bj, err = p.fetch(ctx, b, true); err != nil {
					prog.fail(fmt.Errorf("%s: %w", b, err))
					failed[b] = true
				}
				for k := range bj {
					bj[k].Board = b
				}
				fetched[b] = bj
			}
			prog.step()
			if !failed[b] {
				jobs = append(jobs, bj...)
			}
		}
		keep := func(b string) bool { // a failed board, or one off the selected sources, keeps its jobs in the file
			name, _, _ := splitBoard(b)
			return failed[b] || len(sources) > 0 && !slices.Contains(sources, name)
		}
		if err := t.store(dir, keep, jobs, prog); err != nil {
			errs = append(errs, err)
		}
	}
	if len(failed) > 0 {
		errs = append(errs, fmt.Errorf("%d of %d boards failed", len(failed), len(fetched)))
	}
	return errors.Join(append(errs, prune(dir, feeds))...)
}

// store replaces t's feed file with the jobs in jobs matching the feed, plus the file's jobs of boards
// keep reports, and prints each job whose URL was not in the file.
func (t task) store(dir string, keep func(board string) bool, jobs []Job, out io.Writer) error {
	path := jobFile(dir, t.feed.ID)
	old, err := loadJSON[[]Job](path)
	if err != nil {
		return err
	}
	seen := map[string]bool{}  // URLs in the file
	added := map[string]bool{} // URLs kept
	kept := []Job{}
	for _, j := range old {
		seen[j.URL] = true
		if keep(j.Board) && !added[j.URL] {
			added[j.URL] = true
			kept = append(kept, j)
		}
	}
	for _, j := range jobs {
		if added[j.URL] || !t.feed.matches(j) {
			continue
		}
		added[j.URL] = true
		j.Feed, j.Score, j.Level = t.feed.ID, t.feed.score(j), level(j)
		kept = append(kept, j)
		if !seen[j.URL] {
			printJob(out, j)
		}
	}
	slices.SortStableFunc(kept, func(a, b Job) int { return strings.Compare(a.Board, b.Board) })
	return saveJSON(path, kept)
}
