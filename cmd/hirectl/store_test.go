package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSaveLoadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "feeds.json")

	got, err := loadJSON[[]Feed](path)
	if err != nil || got != nil {
		t.Fatalf("missing file: got %v, %v; want nil, nil", got, err)
	}

	want := []Feed{
		{ID: "go-remote", Title: "Go remote", Skills: []string{"Golang", "PHP"}, Locations: []string{"Remote"}, Sources: []string{"greenhouse"}, Enabled: true},
		{ID: "php-rajkot", Title: "PHP Rajkot", Locations: []string{"Rajkot Gujarat India", "Remote India"}, Sources: []string{"lever", "greenhouse"}},
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

func TestLockDir(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lockDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan struct{})
	go func() {
		u, err := lockDir(dir) // a second descriptor, as another process would take
		if err != nil {
			t.Error(err)
		}
		u()
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("second lock taken while held")
	case <-time.After(50 * time.Millisecond):
	}
	unlock()
	<-got
}
