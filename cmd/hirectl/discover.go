package main

import (
	"bufio"
	"cmp"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// suggestionsFile holds every discovered board's jobs, title, location and techs only, for the feed form to suggest boards from.
const suggestionsFile = "suggestions.json"

const (
	crawlList = "https://index.commoncrawl.org/collinfo.json" // Common Crawl's crawls, newest first
	crawlData = "https://data.commoncrawl.org/"
)

// discoverCmd is `hirectl discover [--boards a,b] [--sources greenhouse,lever] [--delay 1s]`: try boards
// and store their jobs in suggestions.json, printing each board found. Without --boards it tries the
// boards in the latest Common Crawl plus those already stored, which takes long as requests are paced.
func discoverCmd(ctx context.Context, dir string, args []string, out io.Writer) error {
	fs := newFlagSet("discover")
	boardsFlag := fs.String("boards", "", `comma-separated boards, source:name; a bare name is tried on every source; names are lowercased with spaces removed
e.g. --boards "greenhouse:stripe,Acme Corp" tries greenhouse:stripe, greenhouse:acmecorp and lever:acmecorp
(default: boards in the latest Common Crawl, plus those stored)`)
	sourcesFlag := fs.String("sources", "", sourcesUsage())
	delay := fs.Duration("delay", time.Second, "minimum wait between requests to the same integration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("discover: unexpected argument %q", fs.Arg(0))
	}
	sources := parseList(*sourcesFlag)
	if err := checkSources(sources); err != nil {
		return err
	}
	if len(sources) == 0 {
		sources = integrationNames()
	}
	path := filepath.Join(dir, suggestionsFile)
	stored, err := loadJSON[[]Job](path)
	if err != nil {
		return err
	}

	var boards []string
	if given := parseList(*boardsFlag); len(given) > 0 {
		for _, b := range given {
			names, slug := sources, b
			if name, s, found := strings.Cut(b, ":"); found {
				if err := checkSources([]string{strings.TrimSpace(name)}); err != nil {
					return err
				}
				names, slug = []string{strings.TrimSpace(name)}, s
			}
			if slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(slug), " ", "")); slug == "" {
				return fmt.Errorf("board %q: want source:name or name", b)
			}
			for _, s := range names {
				boards = append(boards, s+":"+slug)
			}
		}
	} else {
		if boards, err = crawlBoards(ctx, &http.Client{Timeout: 10 * time.Minute}, sources); err != nil {
			return fmt.Errorf("common crawl: %w", err)
		}
		for _, j := range stored {
			if name, _, _ := splitBoard(j.Board); slices.Contains(sources, name) {
				boards = append(boards, j.Board)
			}
		}
	}
	slices.Sort(boards)
	boards = slices.Compact(boards)

	res := probe(ctx, &http.Client{Timeout: 30 * time.Second}, *delay, boards)
	for i, b := range boards {
		if res[i].err == nil && len(res[i].jobs) > 0 {
			printBoard(out, b, res[i].jobs)
		}
	}
	jobs, errs := merge(stored, boards, res)
	if err := saveJSON(path, jobs); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// result is one board's probe outcome.
type result struct {
	jobs []Job
	err  error
}

// probe fetches boards, one goroutine per integration, each paced by delay, and tags each job with
// the techs in its title and description, dropping the description to bound memory.
// The results are in boards order.
func probe(ctx context.Context, client *http.Client, delay time.Duration, boards []string) []result {
	res := make([]result, len(boards))
	byName := map[string][]int{} // integration name to its boards' indexes
	for i, b := range boards {
		name, _, _ := splitBoard(b)
		byName[name] = append(byName[name], i)
	}
	var wg sync.WaitGroup
	for _, idx := range byName {
		wg.Go(func() {
			p := newPacer(client, delay)
			for _, i := range idx {
				res[i].jobs, res[i].err = p.fetch(ctx, boards[i], true)
				for k, j := range res[i].jobs {
					res[i].jobs[k].Tech, res[i].jobs[k].text = techIn(j.Title+"\n"+j.text), ""
				}
			}
		})
	}
	wg.Wait()
	return res
}

// merge replaces the stored jobs of each probed board with those found, keeping title, location and techs only.
// A board that no longer exists drops out; one that failed otherwise keeps its stored jobs.
func merge(stored []Job, boards []string, res []result) ([]Job, []error) {
	keep := map[string]bool{} // boards whose stored jobs stay
	for _, j := range stored {
		keep[j.Board] = true
	}
	var jobs []Job
	var errs []error
	for i, b := range boards {
		switch r := res[i]; {
		case r.err == nil:
			keep[b] = false
			for _, j := range r.jobs {
				jobs = append(jobs, Job{Board: b, Title: j.Title, Location: j.Location, Tech: j.Tech})
			}
		case errors.Is(r.err, errNotFound):
			keep[b] = false
		default:
			errs = append(errs, fmt.Errorf("%s: %w", b, r.err))
		}
	}
	for _, j := range stored {
		if keep[j.Board] {
			jobs = append(jobs, j)
		}
	}
	slices.SortStableFunc(jobs, func(a, b Job) int { return strings.Compare(a.Board, b.Board) })
	return jobs, errs
}

// autoPicks is how many top suggestions start checked on a feed with no boards yet.
const autoPicks = 10

// suggest lists every board in catalog, those with most jobs matching f first, under a heading that
// says what was matched. Each pick notes matching and total jobs and the board's main techs; its detail
// samples the matching titles.
func suggest(catalog []Job, f Feed) (string, []pick) {
	all := map[string][]Job{}   // board to its jobs
	match := map[string][]Job{} // board to its jobs matching f
	for _, j := range catalog {
		all[j.Board] = append(all[j.Board], j)
		if f.matches(j) {
			match[j.Board] = append(match[j.Board], j)
		}
	}
	n := func(b string) int { return len(match[b]) }
	boards := slices.SortedFunc(maps.Keys(all), func(a, b string) int { return cmp.Or(n(b)-n(a), strings.Compare(a, b)) })
	picks := make([]pick, 0, len(boards))
	for i, b := range boards {
		var titles []string
		for _, j := range match[b][:min(3, n(b))] {
			titles = append(titles, j.Title)
		}
		detail := "No matching roles"
		if n(b) > 0 {
			detail = "Matching roles: " + strings.Join(titles, " · ")
		}
		if more := n(b) - len(titles); more > 0 {
			detail += fmt.Sprintf(" · +%d more", more)
		}
		picks = append(picks, pick{
			value:  b,
			note:   fmt.Sprintf("%d of %d jobs", n(b), len(all[b])),
			tags:   strings.Join(topTech(all[b], 4), ", "),
			detail: detail,
			on:     i < autoPicks && n(b) > 0,
		})
	}
	what := "any role"
	if len(f.Skills) > 0 {
		what = strings.Join(f.Skills, ", ")
	}
	if f.Location != "" {
		what += " in " + f.Location
	}
	return fmt.Sprintf("Suggested boards for %s: %d of %d boards", what, len(match), len(all)), picks
}

// crawlBoards lists the boards of sources whose pages are in the latest Common Crawl. It skips the
// often overloaded search API and reads the crawl's sorted URL index directly: cluster.idx (~100 MB)
// maps URL key ranges to compressed blocks, of which only those holding the sources' hosts are fetched.
// https://commoncrawl.org/get-started
func crawlBoards(ctx context.Context, client *http.Client, sources []string) ([]string, error) {
	var crawls []struct {
		ID string `json:"id"`
	}
	if err := getJSON(ctx, client, crawlList, &crawls); err != nil {
		return nil, err
	}
	if len(crawls) == 0 {
		return nil, errors.New("no crawls listed")
	}
	base := crawlData + "cc-index/collections/" + crawls[0].ID + "/indexes/"
	prefixes := map[string]string{} // URL key prefix, as in "io,greenhouse,boards)/", to integration name
	for _, s := range sources {
		for _, h := range integrations[s].hosts {
			prefixes[urlKey(h)] = s
		}
	}

	idx, err := get(ctx, client, base+"cluster.idx", "")
	if err != nil {
		return nil, err
	}
	blocks, err := indexBlocks(idx, slices.Collect(maps.Keys(prefixes)))
	idx.Close()
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, b := range blocks {
		body, err := get(ctx, client, base+b.file, fmt.Sprintf("%d-%d", b.offset, b.offset+b.length-1))
		if err != nil {
			return nil, err
		}
		err = blockKeys(body, func(key string) {
			for p, s := range prefixes {
				if rest, ok := strings.CutPrefix(key, p); ok {
					if c := rest[:strings.IndexAny(rest+"/", "/?")]; c != "" {
						found[s+":"+c] = true
					}
				}
			}
		})
		body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.file, err)
		}
	}
	return slices.Sorted(maps.Keys(found)), nil
}

// urlKey turns a host into Common Crawl's URL key prefix (SURT): jobs.lever.co is "co,lever,jobs)/".
func urlKey(host string) string {
	parts := strings.Split(host, ".")
	slices.Reverse(parts)
	return strings.Join(parts, ",") + ")/"
}

// block is a byte range of one gzip member in a Common Crawl cdx-*.gz file.
type block struct {
	file           string
	offset, length int64
}

// indexBlocks reads cluster.idx, lines of "<url key> <timestamp>\t<file>\t<offset>\t<length>\t<n>" sorted
// by key, each starting a block that runs to the next line's key, and returns the blocks that may hold
// keys starting with any of prefixes.
func indexBlocks(r io.Reader, prefixes []string) ([]block, error) {
	var blocks []block
	add := func(b block) {
		if !slices.Contains(blocks, b) {
			blocks = append(blocks, b)
		}
	}
	var prevKey string
	var prev block
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if len(f) < 4 {
			continue
		}
		key, _, _ := strings.Cut(f[0], " ")
		off, err1 := strconv.ParseInt(f[2], 10, 64)
		n, err2 := strconv.ParseInt(f[3], 10, 64)
		if err := errors.Join(err1, err2); err != nil {
			return nil, fmt.Errorf("cluster.idx: %w", err)
		}
		b := block{f[1], off, n}
		for _, p := range prefixes {
			if prev.file != "" && prevKey < p && key >= p { // the block before p's first key may hold its first keys
				add(prev)
			}
			if strings.HasPrefix(key, p) {
				add(b)
			}
		}
		prevKey, prev = key, b
	}
	for _, p := range prefixes { // the last block runs to the end of the index
		if prev.file != "" && prevKey < p {
			add(prev)
		}
	}
	return blocks, sc.Err()
}

// blockKeys calls fn with the URL key of each line in a gzipped cdx block: "<url key> <timestamp> <json>".
func blockKeys(r io.Reader, fn func(key string)) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		key, _, _ := strings.Cut(sc.Text(), " ")
		fn(key)
	}
	return sc.Err()
}
