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

// suggestionsFile holds every discovered board's jobs, title, location and techs only, for run to pick each feed's boards from.
const suggestionsFile = "suggestions.json"

const (
	crawlList = "https://index.commoncrawl.org/collinfo.json" // Common Crawl's crawls, newest first
	crawlData = "https://data.commoncrawl.org/"
)

// discoverCmd is `hirectl discover [--boards a,b] [--sources greenhouse,lever] [--delay 1s]`: try boards
// and store their jobs in suggestions.json, printing each board found. Without --boards it tries the
// boards in the latest Common Crawl plus those already stored, which takes long as requests are paced.
// Board errors go to errOut as they happen, with a progress bar when errOut is a terminal.
func discoverCmd(ctx context.Context, dir string, args []string, out, errOut io.Writer) error {
	fs := newFlagSet("discover")
	boardsFlag := fs.String("boards", "", `comma-separated boards, source:name; a bare name is tried on every source; names are lowercased with spaces removed
e.g. --boards "greenhouse:stripe,Acme Corp" tries greenhouse:stripe, and acmecorp on every source
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
	slices.Sort(sources)
	prog := newProgress(out, errOut)
	defer prog.close()
	path, statePath := filepath.Join(dir, suggestionsFile), filepath.Join(dir, discoverFile)
	stored, err := loadJSON[[]Job](path)
	if err != nil {
		return err
	}
	var given []string
	for _, b := range parseList(*boardsFlag) {
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
			given = append(given, s+":"+slug)
		}
	}
	slices.Sort(given)
	given = slices.Compact(given)

	st := discoverState{Sources: sources, Boards: given}
	states, err := loadJSON[[]discoverState](statePath)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(states, st.same); i >= 0 {
		st.Todo = states[i].Todo
	}
	boards := given
	switch {
	case len(st.Todo) > 0:
		boards = st.Todo
		fmt.Fprintf(errOut, "hirectl: resuming the interrupted discover, %d boards left\n", len(boards))
	case len(given) == 0:
		if boards, err = onlineBoards(ctx, &http.Client{Timeout: 10 * time.Minute}, sources, prog); err != nil {
			return err
		}
		for _, j := range stored {
			if name, _, _ := splitBoard(j.Board); slices.Contains(sources, name) {
				boards = append(boards, j.Board)
			}
		}
		slices.Sort(boards)
		boards = slices.Compact(boards)
	}

	// save merges the jobs of the boards probed so far into suggestions.json as it is now, as another
	// discover may have saved since, and records the boards left, failed ones included, for an interrupted
	// discover to resume from, none when done. It runs every checkpointEvery, so a killed one loses little.
	res, done := make([]result, len(boards)), make([]bool, len(boards))
	var mu sync.Mutex
	save := func(finished bool) error {
		var probed []string
		var pres []result
		st.Todo = nil
		for i, b := range boards {
			if done[i] && !res[i].failed() {
				probed, pres = append(probed, b), append(pres, res[i])
			} else if !finished {
				st.Todo = append(st.Todo, b)
			}
		}
		unlock, err := lockDir(dir) // another discover waits, then merges into what this one wrote
		if err != nil {
			return err
		}
		defer unlock()
		cur, err := loadJSON[[]Job](path)
		if err != nil {
			return err
		}
		jobs, _ := merge(cur, probed, pres)
		if err := saveJSON(path, jobs); err != nil {
			return err
		}
		states, err := loadJSON[[]discoverState](statePath)
		if err != nil {
			return err
		}
		if states = slices.DeleteFunc(states, st.same); len(st.Todo) > 0 {
			states = append(states, st)
		}
		return saveJSON(statePath, states)
	}
	saved := time.Now()
	prog.start("", len(boards))
	probe(ctx, &http.Client{Timeout: requestTimeout}, *delay, boards, prog, func(i int, r result) {
		mu.Lock()
		defer mu.Unlock()
		res[i], done[i] = r, true
		if time.Since(saved) >= checkpointEvery {
			saved = time.Now()
			if err := save(false); err != nil {
				prog.fail(err)
			}
		}
	})
	errs := []error{save(ctx.Err() == nil)} // finished unless interrupted: the next discover starts afresh
	if failed := len(slices.DeleteFunc(slices.Clone(res), func(r result) bool { return !r.failed() })); failed > 0 {
		errs = append(errs, fmt.Errorf("%d of %d boards failed", failed, len(boards)))
	}
	return errors.Join(errs...)
}

// discoverFile holds, per --sources and --boards, the boards an interrupted discover has left, which
// a discover with the same options resumes from.
const discoverFile = "discover.json"

type discoverState struct {
	Sources []string `json:"sources"`
	Boards  []string `json:"boards"` // --boards given, none for Common Crawl's
	Todo    []string `json:"todo"`
}

// same reports whether o ran with s's options.
func (s discoverState) same(o discoverState) bool {
	return slices.Equal(s.Sources, o.Sources) && slices.Equal(s.Boards, o.Boards)
}

// checkpointEvery is how often discover saves its progress.
const checkpointEvery = 30 * time.Second

// result is one board's probe outcome.
type result struct {
	jobs []Job
	err  error
}

// failed reports an error other than a board that does not exist.
func (r result) failed() bool { return r.err != nil && !errors.Is(r.err, errNotFound) }

// probe fetches boards, one goroutine per integration, each paced by delay, and tags each job with
// the techs in its title and description, dropping the description to bound memory. It prints each
// board with jobs to prog as found, and each failure, and passes each outcome to done, from several goroutines.
func probe(ctx context.Context, client *http.Client, delay time.Duration, boards []string, prog *progress, done func(i int, r result)) {
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
				r := &result{}
				prog.at(boards[i])
				r.jobs, r.err = p.fetch(ctx, boards[i], true)
				for k, j := range r.jobs {
					r.jobs[k].Tech, r.jobs[k].text = techIn(j.Title+"\n"+j.text), ""
				}
				switch {
				case r.failed():
					prog.fail(fmt.Errorf("%s: %w", boards[i], r.err))
				case r.err == nil && len(r.jobs) > 0:
					printBoard(prog, boards[i], r.jobs)
				}
				done(i, *r)
				prog.step()
			}
		})
	}
	wg.Wait()
}

// merge replaces the stored jobs of each probed board with those found, keeping title, location and techs only.
// A board that no longer exists drops out; one that failed otherwise keeps its stored jobs, counted in failed.
func merge(stored []Job, boards []string, res []result) (jobs []Job, failed int) {
	keep := map[string]bool{} // boards whose stored jobs stay
	for _, j := range stored {
		keep[j.Board] = true
	}
	for i, b := range boards {
		switch r := res[i]; {
		case r.failed():
			failed++
		case r.err == nil:
			keep[b] = false
			for _, j := range r.jobs {
				jobs = append(jobs, Job{Board: b, Title: j.Title, Location: j.Location, Tech: j.Tech})
			}
		default: // not found
			keep[b] = false
		}
	}
	for _, j := range stored {
		if keep[j.Board] {
			jobs = append(jobs, j)
		}
	}
	slices.SortStableFunc(jobs, func(a, b Job) int { return strings.Compare(a.Board, b.Board) })
	return jobs, failed
}

// matching groups the jobs in catalog matching f by board, on f's sources only, all when it has none.
func matching(catalog []Job, f Feed) map[string][]Job {
	match := map[string][]Job{}
	for _, j := range catalog {
		if src, _, _ := splitBoard(j.Board); (len(f.Sources) == 0 || slices.Contains(f.Sources, src)) && f.matches(j) {
			match[j.Board] = append(match[j.Board], j)
		}
	}
	return match
}

// topBoards returns up to n boards of match, most jobs first, ties by name.
func topBoards(match map[string][]Job, n int) []string {
	boards := slices.SortedFunc(maps.Keys(match), func(a, b string) int {
		return cmp.Or(len(match[b])-len(match[a]), strings.Compare(a, b))
	})
	return boards[:min(n, len(boards))]
}

// suggest offers every integration as a pick for f's Sources, noting its boards and jobs matching f,
// and as detail its top n boards, which run fetches when it is the only source checked.
func suggest(catalog []Job, f Feed, n int) (string, []pick) {
	f.Sources = nil
	match := matching(catalog, f)
	picks := make([]pick, 0, len(integrations))
	for _, s := range integrationNames() {
		on, jobs := map[string][]Job{}, 0
		for b, js := range match {
			if src, _, _ := splitBoard(b); src == s {
				on[b], jobs = js, jobs+len(js)
			}
		}
		var top []string
		for _, b := range topBoards(on, n) {
			top = append(top, fmt.Sprintf("%s (%d)", b, len(on[b])))
		}
		detail := "No matching boards"
		if len(top) > 0 {
			detail = "Top boards: " + strings.Join(top, " · ")
		}
		picks = append(picks, pick{value: s, note: fmt.Sprintf("%d boards, %d jobs", len(on), jobs), detail: detail})
	}
	what := "any role"
	if len(f.Skills) > 0 {
		what = strings.Join(f.Skills, ", ")
	}
	if len(f.Locations) > 0 {
		what += " in " + strings.Join(f.Locations, " or ")
	}
	return fmt.Sprintf("Sources for %s: %d boards match, run fetches the top %d on the checked sources (none checked: all)", what, len(match), n), picks
}

// onlineBoards lists the boards of sources, sorted: an integration's own list when it has one, else
// those in the latest Common Crawl.
func onlineBoards(ctx context.Context, client *http.Client, sources []string, prog *progress) ([]string, error) {
	var boards, crawled []string
	for _, s := range sources {
		in := integrations[s]
		if in.boards == nil {
			crawled = append(crawled, s)
			continue
		}
		names, err := in.boards(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("%s boards: %w", s, err)
		}
		for _, n := range names {
			boards = append(boards, s+":"+n)
		}
	}
	if len(crawled) > 0 {
		cb, err := crawlBoards(ctx, client, crawled, prog)
		if err != nil {
			return nil, fmt.Errorf("common crawl: %w", err)
		}
		boards = append(boards, cb...)
	}
	slices.Sort(boards)
	return boards, nil
}

// crawlBoards lists the boards of sources whose pages are in the latest Common Crawl. It skips the
// often overloaded search API and reads the crawl's sorted URL index directly: cluster.idx (~100 MB)
// maps URL key ranges to compressed blocks, of which only those holding the sources' hosts are fetched.
// https://commoncrawl.org/get-started
func crawlBoards(ctx context.Context, client *http.Client, sources []string, prog *progress) ([]string, error) {
	prog.start("Common Crawl: reading index", 0)
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
	type hostOf struct{ source, host string } // an integration and one of its hosts entries
	prefixes := map[string]hostOf{}           // URL key prefix, as in "io,greenhouse,boards)/", to its host
	for _, s := range sources {
		for _, h := range integrations[s].hosts {
			prefixes[urlKey(h)] = hostOf{s, h}
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
	prog.start("Common Crawl: reading blocks", len(blocks))
	found := map[string]bool{}
	for _, b := range blocks {
		body, err := get(ctx, client, base+b.file, fmt.Sprintf("%d-%d", b.offset, b.offset+b.length-1))
		if err != nil {
			return nil, err
		}
		err = blockKeys(body, func(key string) {
			for p, h := range prefixes {
				if strings.HasPrefix(key, p) {
					if c := crawlCompany(integrations[h.source], h.host, key); c != "" {
						found[h.source+":"+c] = true
					}
				}
			}
		})
		body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.file, err)
		}
		prog.step()
	}
	return slices.Sorted(maps.Keys(found)), nil
}

// urlKey turns a hosts entry into Common Crawl's URL key prefix (SURT): jobs.lever.co is "co,lever,jobs)/",
// *.recruitee.com, any subdomain, is "com,recruitee,".
func urlKey(host string) string {
	if domain, ok := strings.CutPrefix(host, "*."); ok {
		return strings.TrimSuffix(urlKey(domain), ")/") + ","
	}
	parts := strings.Split(host, ".")
	slices.Reverse(parts)
	return strings.Join(parts, ",") + ")/"
}

// crawlCompany returns the company of in whose board holds the page with URL key key, found under
// in's hosts entry host, or "" for none.
func crawlCompany(in *integration, host, key string) string {
	surt, path, _ := strings.Cut(key, ")")
	parts := strings.Split(surt, ",")
	slices.Reverse(parts)
	page := strings.Join(parts, ".")
	if in.company != nil {
		return in.company(page, path)
	}
	if domain, ok := strings.CutPrefix(host, "*."); ok {
		if c := strings.TrimSuffix(page, "."+domain); !strings.Contains(c, ".") {
			return c
		}
		return ""
	}
	return firstSegment(path)
}

// firstSegment returns the first segment of a URL path and query: "/acme/1?x" is "acme".
func firstSegment(path string) string {
	path = strings.TrimPrefix(path, "/")
	return path[:strings.IndexAny(path+"/", "/?")]
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
