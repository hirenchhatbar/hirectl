package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// lists maps each `hirectl list` subcommand to what it prints from dir.
var lists = map[string]func(dir string, out io.Writer) error{
	"feeds": func(dir string, out io.Writer) error {
		feeds, err := loadJSON[[]Feed](filepath.Join(dir, feedKind.file))
		for _, f := range feeds {
			state := "enabled"
			if !f.Enabled {
				state = "disabled"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", f.ID, state, f.Title, strings.Join(f.Boards, ","))
		}
		return err
	},
	"jobs": func(dir string, out io.Writer) error {
		jobs, err := loadJSON[[]Job](filepath.Join(dir, jobsFile))
		for _, j := range jobs {
			printJob(out, j)
		}
		return err
	},
	"boards": func(dir string, out io.Writer) error {
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
	"sources": func(_ string, out io.Writer) error {
		fmt.Fprintln(out, strings.Join(integrationNames(), "\n"))
		return nil
	},
}

// listCmd is `hirectl list <feeds|jobs|boards|sources>`: print stored data tab-separated.
func listCmd(_ context.Context, dir string, args []string, out io.Writer) error {
	names := strings.Join(slices.Sorted(maps.Keys(lists)), ", ")
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprintf(fs.Output(), "Usage: hirectl list <what>, what is one of: %s\n", names) }
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 || lists[fs.Arg(0)] == nil {
		return fmt.Errorf("list: want one of %s", names)
	}
	return lists[fs.Arg(0)](dir, out)
}

// printJob prints j as `feed-id  board  title  location  url`, shared by run and list jobs.
func printJob(out io.Writer, j Job) {
	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", j.Feed, j.Board, j.Title, j.Location, j.URL)
}

// printBoard prints a board, its job count and main techs, shared by discover and list boards.
func printBoard(out io.Writer, board string, jobs []Job) {
	fmt.Fprintf(out, "%s\t%d jobs\t%s\n", board, len(jobs), strings.Join(topTech(jobs, 4), ","))
}
