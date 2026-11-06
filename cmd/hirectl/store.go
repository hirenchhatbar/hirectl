package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// dataDir returns $HIRECTL_DIR, or hirectl in the user config dir.
func dataDir() (string, error) {
	if d := os.Getenv("HIRECTL_DIR"); d != "" {
		return d, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "hirectl"), nil
}

// loadJSON reads a JSON file, such as a list or config.json; a missing file means T's zero value.
func loadJSON[T any](path string) (T, error) {
	var v T
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

// saveJSON writes a JSON file atomically: temp file, then rename.
func saveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// lockFile is the data dir's lock, held while a command reads, merges and writes shared files.
const lockFile = "hirectl.lock"

// lockDir takes an exclusive lock on dir's lockFile, waiting while another process holds it, and
// returns its release. The OS drops the lock if the process dies. shortcut: flock is Unix only, use
// LockFileEx if hirectl ever builds for Windows.
func lockDir(dir string) (unlock func(), err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, lockFile), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", f.Name(), err)
	}
	return func() { f.Close() }, nil // closing releases the lock
}
