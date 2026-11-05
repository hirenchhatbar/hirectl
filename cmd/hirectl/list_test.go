package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestListCmd(t *testing.T) {
	dir := t.TempDir()
	if err := saveJSON(filepath.Join(dir, feedKind.file), []Feed{{ID: "go", Title: "Go", Boards: []string{"lever:a", "lever:b"}}}); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(dir, jobsFile), []Job{{Feed: "go", Board: "lever:a", Title: "Dev", URL: "u"}}); err != nil {
		t.Fatal(err)
	}
	if err := saveJSON(filepath.Join(dir, suggestionsFile), []Job{{Board: "lever:b", Tech: []string{"Go"}}, {Board: "lever:a"}, {Board: "greenhouse:a"}, {Board: "lever:b", Tech: []string{"Go", "PHP"}}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"feeds"}, "go\tdisabled\tGo\tlever:a,lever:b\n"},
		{[]string{"jobs"}, "go\tlever:a\tDev\t\tu\n"},
		{[]string{"boards"}, "greenhouse:a\t1 jobs\t\nlever:a\t1 jobs\t\nlever:b\t2 jobs\tGo,PHP\n"},
		{[]string{"sources"}, "greenhouse\nlever\n"},
	} {
		var out strings.Builder
		if err := listCmd(context.Background(), dir, tc.args, &out); err != nil || out.String() != tc.want {
			t.Errorf("%v: got %q, %v; want %q", tc.args, out.String(), err, tc.want)
		}
	}
	for _, args := range [][]string{nil, {"nope"}, {"jobs", "x"}} {
		if err := listCmd(context.Background(), dir, args, &strings.Builder{}); err == nil {
			t.Errorf("%v: want error", args)
		}
	}
}
