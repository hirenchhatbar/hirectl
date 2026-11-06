package main

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListCmd(t *testing.T) {
	dir := t.TempDir()
	if err := saveJSON(filepath.Join(dir, feedKind.file), []Feed{{ID: "go", Title: "Go", Sources: []string{"lever", "greenhouse"}}}); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour) // yesterday, printed as a date
	if err := saveJSON(jobFile(dir, "go"), []Job{
		{Feed: "go", Board: "lever:a", Title: "Dev", URL: "u"},
		{Feed: "go", Board: "lever:a", Title: "Old", URL: "o", Posted: day.Add(-30 * 24 * time.Hour)},
		{Feed: "go", Board: "lever:b", Title: "New", URL: "n", Posted: day},
	}); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(dir, suggestionsFile), []Job{{Board: "lever:b", Tech: []string{"Go"}}, {Board: "lever:a"}, {Board: "greenhouse:a"}, {Board: "lever:b", Tech: []string{"Go", "PHP"}}}); err != nil {
		t.Fatal(err)
	}
	dev, old, recent := "go\tlever:a\tDev\t\tu\t\n", "go\tlever:a\tOld\t\to\t"+day.Add(-30*24*time.Hour).Format(time.DateOnly)+"\n",
		"go\tlever:b\tNew\t\tn\t"+day.Format(time.DateOnly)+"\n"
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"feeds"}, "go\tdisabled\tGo\tlever,greenhouse\n"},
		{[]string{"jobs"}, dev + old + recent},
		{[]string{"jobs", "--sort", "posted"}, recent + old + dev},
		{[]string{"--since", "7d", "jobs"}, dev + recent},
		{[]string{"jobs", "--since", "72h", "--sort", "posted"}, recent + dev},
		{[]string{"boards"}, "greenhouse:a\t1 jobs\t\nlever:a\t1 jobs\t\nlever:b\t2 jobs\tGo,PHP\n"},
		{[]string{"sources"}, "ashby\t0 boards\nbreezy\t0 boards\ngem\t0 boards\ngreenhouse\t1 boards\nlever\t2 boards\nlever-eu\t0 boards\npersonio\t0 boards\nrecruitee\t0 boards\nrippling\t0 boards\nsmartrecruiters\t0 boards\nworkable\t0 boards\nworkday\t0 boards\n"},
	} {
		var out strings.Builder
		if err := listCmd(context.Background(), dir, tc.args, &out, io.Discard); err != nil || out.String() != tc.want {
			t.Errorf("%v: got %q, %v; want %q", tc.args, out.String(), err, tc.want)
		}
	}
	for _, args := range [][]string{nil, {"nope"}, {"jobs", "x"}, {"jobs", "--since", "0d"}, {"jobs", "--since", "week"}, {"jobs", "--sort", "title"}, {"feeds", "--sort", "posted"}} {
		if err := listCmd(context.Background(), dir, args, &strings.Builder{}, io.Discard); err == nil {
			t.Errorf("%v: want error", args)
		}
	}
}
