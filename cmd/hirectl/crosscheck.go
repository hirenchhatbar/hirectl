package main

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// crossCheckCmd is `hirectl cross-check [--sources s,t]`: per source, compare the boards stored in
// suggestions.json with the boards online (see onlineBoards), printing
// `source  <n> stored  <m> online  <k> not stored`.
func crossCheckCmd(ctx context.Context, dir string, args []string, out, errOut io.Writer) error {
	fs := newFlagSet("cross-check")
	sourcesFlag := fs.String("sources", "", sourcesUsage())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("cross-check: unexpected argument %q", fs.Arg(0))
	}
	sources := parseList(*sourcesFlag)
	if err := checkSources(sources); err != nil {
		return err
	}
	if len(sources) == 0 {
		sources = integrationNames()
	}
	stored, err := loadJSON[[]Job](filepath.Join(dir, suggestionsFile))
	if err != nil {
		return err
	}
	prog := newProgress(out, errOut)
	defer prog.close()
	online, err := onlineBoards(ctx, &http.Client{Timeout: 10 * time.Minute}, sources, prog)
	if err != nil {
		return err
	}
	printCrossCheck(prog, sources, stored, online)
	return nil
}

// printCrossCheck prints, for each of sources in order, its boards in stored, in online, and in online but
// not stored, then after a blank line the discover command to run for each source with boards not stored,
// most first, ties by name.
func printCrossCheck(out io.Writer, sources []string, stored []Job, online []string) {
	have, crawled := boardsBySource(stored), map[string]int{}
	missing := map[string]int{}
	for _, b := range online {
		src, _, _ := splitBoard(b)
		crawled[src]++
		if !have[src][b] {
			missing[src]++
		}
	}
	for _, s := range sources {
		fmt.Fprintf(out, "%s\t%d stored\t%d online\t%d not stored\n", s, len(have[s]), crawled[s], missing[s])
	}
	todo := slices.DeleteFunc(slices.Clone(sources), func(s string) bool { return missing[s] == 0 })
	slices.SortStableFunc(todo, func(a, b string) int { return cmp.Or(missing[b]-missing[a], strings.Compare(a, b)) })
	for i, s := range todo {
		if i == 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "hirectl discover --sources %s\n", s)
	}
}
