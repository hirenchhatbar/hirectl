package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestPlan(t *testing.T) {
	feeds := []Feed{
		{ID: "go", Boards: []string{"greenhouse:a", "lever:b"}, Enabled: true},
		{ID: "php", Boards: []string{"lever:c"}, Enabled: true},
		{ID: "off", Boards: []string{"lever:d"}},
	}
	for _, tc := range []struct {
		name           string
		feeds, sources []string
		want           map[string][]string // feed ID to boards
		wantErr        bool
	}{
		{name: "all", want: map[string][]string{"go": {"greenhouse:a", "lever:b"}, "php": {"lever:c"}}},
		{name: "one feed", feeds: []string{"php"}, want: map[string][]string{"php": {"lever:c"}}},
		{name: "one source", sources: []string{"greenhouse"}, want: map[string][]string{"go": {"greenhouse:a"}}},
		{name: "feed and source", feeds: []string{"go", "php"}, sources: []string{"lever"}, want: map[string][]string{"go": {"lever:b"}, "php": {"lever:c"}}},
		{name: "feed lacks source", feeds: []string{"php"}, sources: []string{"greenhouse"}, wantErr: true},
		{name: "unknown feed", feeds: []string{"nope"}, wantErr: true},
		{name: "disabled feed", feeds: []string{"off"}, wantErr: true},
		{name: "unknown source", sources: []string{"indeed"}, wantErr: true},
	} {
		tasks, err := plan(feeds, tc.feeds, tc.sources)
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

func TestRunCmdDelay(t *testing.T) {
	var calls []time.Time
	integrations["fake"] = &integration{fetch: func(context.Context, *http.Client, string, bool) ([]Job, error) {
		calls = append(calls, time.Now())
		return nil, nil
	}}
	defer delete(integrations, "fake")
	dir := t.TempDir()
	if err := saveJSON(filepath.Join(dir, feedKind.file), []Feed{{ID: "go", Boards: []string{"fake:a", "fake:b"}, Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	const delay = 50 * time.Millisecond
	if err := runCmd(context.Background(), dir, []string{"-delay", delay.String()}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[1].Sub(calls[0]) < delay {
		t.Fatalf("calls %v, want 2 at least %v apart", calls, delay)
	}
}
