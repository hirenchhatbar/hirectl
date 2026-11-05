package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

const (
	version = "0.0.1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hirectl:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := dataDir()
	if err != nil {
		return err
	}
	sources, err := newSlot("f2", "Sources", dir, sourceKind)
	if err != nil {
		return err
	}
	feeds, err := newSlot("f3", "Feeds", dir, feedKind)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(newApp(sources, feeds)).Run()
	return err
}

// newSlot loads k's items from dir and binds their list module to key.
func newSlot[T record[T]](key, name, dir string, k kind[T]) (slot, error) {
	path := filepath.Join(dir, k.file)
	items, err := loadJSON[T](path)
	if err != nil {
		return slot{}, err
	}
	return slot{key: key, name: name, m: newListModel(k, path, items)}, nil
}
