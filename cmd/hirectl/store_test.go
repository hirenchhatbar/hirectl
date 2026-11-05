package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "feeds.json")

	got, err := loadJSON[Feed](path)
	if err != nil || got != nil {
		t.Fatalf("missing file: got %v, %v; want nil, nil", got, err)
	}

	want := []Feed{
		{Title: "Go remote", Skills: []string{"Golang", "PHP"}, Location: "Remote", Schedule: "@daily", Enabled: true},
		{Title: "PHP Rajkot", Location: "Rajkot, Gujarat, India", Schedule: "0 9 * * 1-5"},
	}
	if err := saveJSON(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err = loadJSON[Feed](path); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
