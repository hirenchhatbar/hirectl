package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPlan(t *testing.T) {
	catalog := []Job{
		{Board: "greenhouse:a", Title: "Go Dev"},
		{Board: "lever:b", Title: "Go Dev"}, {Board: "lever:b", Title: "Go Lead"},
		{Board: "lever:c", Title: "PHP Dev"},
		{Board: "lever:d", Title: "Go Dev"},
	}
	feeds := []Feed{
		{ID: "go", Skills: []string{"Go"}, Enabled: true},
		{ID: "php", Skills: []string{"PHP"}, Sources: []string{"lever"}, Enabled: true},
		{ID: "gh", Skills: []string{"Go"}, Sources: []string{"greenhouse"}, Enabled: true},
		{ID: "off", Skills: []string{"Go"}},
	}
	for _, tc := range []struct {
		name           string
		feeds, sources []string
		want           map[string][]string // feed ID to boards
		wantErr        bool
	}{
		{name: "all, top 2", want: map[string][]string{"go": {"lever:b", "greenhouse:a"}, "php": {"lever:c"}, "gh": {"greenhouse:a"}}},
		{name: "one feed", feeds: []string{"php"}, want: map[string][]string{"php": {"lever:c"}}},
		{name: "one source", sources: []string{"greenhouse"}, want: map[string][]string{"go": {"greenhouse:a"}, "gh": {"greenhouse:a"}}},
		{name: "feed and source", feeds: []string{"go", "php"}, sources: []string{"lever"}, want: map[string][]string{"go": {"lever:b", "lever:d"}, "php": {"lever:c"}}},
		{name: "feed lacks source", feeds: []string{"php"}, sources: []string{"greenhouse"}, wantErr: true},
		{name: "unknown feed", feeds: []string{"nope"}, wantErr: true},
		{name: "disabled feed", feeds: []string{"off"}, wantErr: true},
		{name: "unknown source", sources: []string{"indeed"}, wantErr: true},
	} {
		tasks, err := plan(feeds, catalog, 2, tc.feeds, tc.sources)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err = %v, want error %v", tc.name, err, tc.wantErr)
			continue
		}
		if tc.wantErr {
			continue
		}
		got := map[string][]string{}
		for _, tk := range tasks {
			got[tk.feed.ID] = tk.boards
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// fakeSource registers integration "fake", whose boards hold jobs[company], or fail with err when set.
func fakeSource(t *testing.T, jobs map[string][]Job, err error) {
	t.Helper()
	integrations["fake"] = &integration{fetch: func(_ context.Context, _ *http.Client, company string, _ bool) ([]Job, error) {
		return slices.Clone(jobs[company]), err
	}}
	t.Cleanup(func() { delete(integrations, "fake") })
}

// saveData writes feeds, the suggestions catalog and config.json into dir.
func saveData(t *testing.T, dir string, feeds []Feed, catalog []Job, cfg Config) {
	t.Helper()
	for p, v := range map[string]any{feedKind.file: feeds, suggestionsFile: catalog, configFile: cfg} {
		if err := saveJSON(filepath.Join(dir, p), v); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunCmdNoCatalog(t *testing.T) {
	dir := t.TempDir()
	saveData(t, dir, []Feed{{ID: "go", Enabled: true}}, nil, Config{})
	if err := runCmd(context.Background(), dir, nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "hirectl discover") {
		t.Fatalf("err = %v, want a hint to run discover", err)
	}
}

func TestRunCmdDelay(t *testing.T) {
	var calls []time.Time
	integrations["fake"] = &integration{fetch: func(context.Context, *http.Client, string, bool) ([]Job, error) {
		calls = append(calls, time.Now())
		return nil, nil
	}}
	defer delete(integrations, "fake")
	dir := t.TempDir()
	saveData(t, dir, []Feed{{ID: "go", Enabled: true}}, []Job{{Board: "fake:a"}, {Board: "fake:b"}}, Config{})
	const delay = 50 * time.Millisecond
	if err := runCmd(context.Background(), dir, []string{"-delay", delay.String()}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[1].Sub(calls[0]) < delay {
		t.Fatalf("calls %v, want 2 at least %v apart", calls, delay)
	}
}

func TestRunCmdStore(t *testing.T) {
	jobs := map[string][]Job{}
	fakeSource(t, jobs, nil)
	dir := t.TempDir()
	catalog := []Job{{Board: "fake:a", Location: "Remote"}, {Board: "fake:a", Location: "Berlin"}}
	feed := Feed{ID: "go", Locations: []string{"Remote"}, Enabled: true}
	run := func(f Feed) string {
		t.Helper()
		saveData(t, dir, []Feed{f}, catalog, Config{})
		var out strings.Builder
		if err := runCmd(context.Background(), dir, []string{"--delay", "0s"}, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	stored := func() []Job {
		t.Helper()
		got, err := loadJSON[[]Job](jobFile(dir, "go"))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	old := filepath.Join(dir, jobsDir, "go", "fake_a.json") // an older version's layout
	if err := saveJSON(old, []Job{}); err != nil {
		t.Fatal(err)
	}

	jobs["a"] = []Job{{Title: "A", Location: "Remote", URL: "1"}, {Title: "B", Location: "Berlin", URL: "2"}}
	if got := run(feed); got != "go\tfake:a\tA\tRemote\t1\t\n" {
		t.Fatalf("first run printed %q", got)
	}
	if _, err := os.Stat(filepath.Dir(old)); !os.IsNotExist(err) {
		t.Fatalf("older version's dir left: %v", err)
	}
	jobs["a"] = append(jobs["a"], Job{Title: "C", Location: "Remote", URL: "3"})
	if got := run(feed); got != "go\tfake:a\tC\tRemote\t3\t\n" {
		t.Fatalf("second run printed %q, want only the new job", got)
	}
	feed.Locations = []string{"Berlin"}
	if got := run(feed); got != "go\tfake:a\tB\tBerlin\t2\t\n" || len(stored()) != 1 {
		t.Fatalf("after edit printed %q, stored %+v; want only B", got, stored())
	}
	feed.ID = "go-eu"
	run(feed)
	if files, _ := jobFiles(dir); len(files) != 1 || files[0] != jobFile(dir, "go-eu") {
		t.Fatalf("after rename files %v, want only go-eu's", files)
	}
}

func TestRunCmdTopBoards(t *testing.T) {
	jobs := map[string][]Job{"a": {{Title: "A", URL: "1"}}, "b": {{Title: "B", URL: "2"}}}
	fakeSource(t, jobs, nil)
	dir := t.TempDir()
	feeds := []Feed{{ID: "go", Enabled: true}}
	run := func(top string) string {
		t.Helper()
		saveData(t, dir, feeds, []Job{{Board: "fake:" + top}, {Board: "fake:" + top}, {Board: "fake:a"}, {Board: "fake:b"}}, Config{Boards: 1})
		var out strings.Builder
		if err := runCmd(context.Background(), dir, []string{"--delay", "0s"}, &out, io.Discard); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := run("a"); got != "go\tfake:a\tA\t\t1\t\n" {
		t.Fatalf("top a printed %q, want only fake:a fetched", got)
	}
	if got := run("b"); got != "go\tfake:b\tB\t\t2\t\n" {
		t.Fatalf("top b printed %q, want only fake:b's job", got)
	}
	if got := run("a"); got != "" {
		t.Fatalf("a again printed %q, want nothing: its jobs stayed in the file", got)
	}
	if got, _ := loadJSON[[]Job](jobFile(dir, "go")); len(got) != 2 {
		t.Fatalf("stored %+v, want both boards' jobs", got)
	}
}

func TestRunCmdKeepsFailedBoard(t *testing.T) {
	fakeSource(t, nil, errors.New("down"))
	dir := t.TempDir()
	saveData(t, dir, []Feed{{ID: "go", Enabled: true}}, []Job{{Board: "fake:a"}}, Config{})
	path := jobFile(dir, "go")
	if err := saveJSON(path, []Job{{Board: "fake:a", Title: "A", URL: "1"}}); err != nil {
		t.Fatal(err)
	}
	var errOut strings.Builder
	if err := runCmd(context.Background(), dir, []string{"--delay", "0s"}, io.Discard, &errOut); err == nil || err.Error() != "1 of 1 boards failed" {
		t.Fatalf("err = %v, want 1 of 1 boards failed", err)
	}
	if got, want := errOut.String(), "hirectl: fake:a: down\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if got, _ := loadJSON[[]Job](path); len(got) != 1 {
		t.Fatalf("failed board's jobs now %+v, want kept", got)
	}
}
