package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	version = "0.0.1"
)

const usage = `Usage:
  hirectl                                                          open the TUI
  hirectl --version                                                print the version
  hirectl run      [--feeds a,b]  [--sources s,t] [--delay 1s]  fetch feeds' boards, store new matching jobs
  hirectl discover [--boards a,b] [--sources s,t] [--delay 1s]  find boards, store their jobs as suggestions
  hirectl list     <feeds|jobs|boards|sources>                  print stored feeds, jobs, suggested boards, or sources

Examples:
  hirectl run                                          all enabled feeds, all sources
  hirectl run --feeds go-remote --sources greenhouse   one feed by ID, Greenhouse boards only
  hirectl discover --boards stripe                     tries greenhouse:stripe and lever:stripe
  hirectl discover --boards greenhouse:stripe          tries greenhouse:stripe only
  hirectl discover --sources lever --delay 2s          Common Crawl boards on Lever, slower pace
  hirectl list jobs                                    every stored job, one per line

Run "hirectl <command> --help" for a command's options.
`

func main() {
	if err := run(); err != nil && !errors.Is(err, flag.ErrHelp) { // the flag package already printed the help
		fmt.Fprintln(os.Stderr, "hirectl:", err)
		os.Exit(1)
	}
}

func run() error {
	cmds := map[string]func(context.Context, string, []string, io.Writer) error{"run": runCmd, "discover": discoverCmd, "list": listCmd}
	if len(os.Args) > 1 {
		switch a := os.Args[1]; {
		case slices.Contains([]string{"-h", "-help", "--help", "help"}, a):
			fmt.Print(usage)
			return nil
		case slices.Contains([]string{"-v", "-version", "--version", "version"}, a):
			fmt.Println("hirectl", version)
			return nil
		case cmds[a] == nil:
			return fmt.Errorf("unknown command %q, see hirectl --help", a)
		}
	}
	dir, err := dataDir()
	if err != nil {
		return err
	}
	if len(os.Args) > 1 {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return cmds[os.Args[1]](ctx, dir, os.Args[2:], os.Stdout)
	}
	catalog, err := loadJSON[[]Job](filepath.Join(dir, suggestionsFile))
	if err != nil {
		return err
	}
	fk := feedKind
	fk.picks = func(v []string) (string, []pick) {
		f, _ := fk.parse(v) // a feed still being filled in is invalid, yet matches fine
		return suggest(catalog, f)
	}
	feeds, err := newSlot("f2", "Feeds", dir, fk)
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, configFile)
	cfg, err := loadJSON[Config](cfgPath)
	if err != nil {
		return err
	}
	conf := configModel{path: cfgPath, cfg: cfg}
	if err := applyTheme(cmp.Or(cfg.Theme, defaultTheme)); err != nil {
		conf.status = err.Error() // shown on the config page; the theme falls back
	}
	_, err = tea.NewProgram(newApp(feeds, slot{key: "f9", name: "Config", m: conf})).Run()
	return err
}

// newSlot loads k's items from dir and binds their list module to key.
func newSlot[T record[T]](key, name, dir string, k kind[T]) (slot, error) {
	path := filepath.Join(dir, k.file)
	items, err := loadJSON[[]T](path)
	if err != nil {
		return slot{}, err
	}
	return slot{key: key, name: name, m: newListModel(k, path, items)}, nil
}

// newFlagSet is a command's flag set whose help lists options as --name; the flag package prints -name.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.Usage = func() {
		var b strings.Builder
		out := fs.Output()
		fs.SetOutput(&b)
		fs.PrintDefaults()
		fs.SetOutput(out)
		fmt.Fprintf(out, "Usage of hirectl %s:\n%s", name, regexp.MustCompile(`(?m)^  -`).ReplaceAllString(b.String(), "  --"))
	}
	return fs
}
