package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "feeds.json")

	got, err := loadJSON[[]Feed](path)
	if err != nil || got != nil {
		t.Fatalf("missing file: got %v, %v; want nil, nil", got, err)
	}

	want := []Feed{
		{ID: "go-remote", Title: "Go remote", Skills: []string{"Golang", "PHP"}, Location: "Remote", Boards: []string{"greenhouse:stripe"}, Enabled: true},
		{ID: "php-rajkot", Title: "PHP Rajkot", Location: "Rajkot, Gujarat, India", Boards: []string{"lever:netflix", "greenhouse:acme"}},
	}
	if err := saveJSON(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err = loadJSON[[]Feed](path); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
