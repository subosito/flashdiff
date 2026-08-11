package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxFileSize = 1 << 20 // 1 MiB; larger files are treated as binary
	maxCached   = 4096    // safety cap on tracked files
)

// snapshot holds the last-known content of one file.
type snapshot struct {
	content string
	binary  bool
}

// fileStore is the baseline content cache: diffs are computed between
// the cached previous content and the current on-disk content.
type fileStore struct {
	mu    sync.RWMutex
	files map[string]snapshot
}

func newFileStore() *fileStore {
	return &fileStore{files: make(map[string]snapshot)}
}

func (s *fileStore) get(rel string) (snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.files[rel]
	return snap, ok
}

func (s *fileStore) put(rel string, snap snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[rel] = snap
}

func (s *fileStore) remove(rel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, rel)
}

func (s *fileStore) reset(snaps map[string]snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = snaps
}

// readFile reads f and reports whether it is binary (non-UTF-8-ish).
// Writers are often mid-write when the fs event fires, so this retries
// briefly until size+mtime are stable, avoiding baselines built on
// partial content.
func readFile(path string) (content string, binary bool, err error) {
	const attempts = 3
	for i := 0; i < attempts; i++ {
		c, b, stable, rerr := readFileOnce(path)
		if rerr != nil {
			return "", false, rerr
		}
		if stable || i == attempts-1 {
			return c, b, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", false, nil // unreachable
}

func readFileOnce(path string) (content string, binary, stable bool, err error) {
	before, err := os.Stat(path)
	if err != nil {
		return "", false, false, err
	}
	if !before.Mode().IsRegular() || before.Size() > maxFileSize {
		return "", true, true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, false, err
	}
	after, err := os.Stat(path)
	if err != nil {
		return "", false, false, err
	}
	stable = before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
	if isBinary(data) {
		return "", true, stable, nil
	}
	return string(data), false, stable, nil
}

// isBinary reports whether data looks non-textual (NUL byte in the
// first 8 KiB, the same heuristic grep uses).
func isBinary(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	for _, b := range data[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// scan walks root and snapshots every file that passes the filters.
func scan(root string, ignores *ignoreSet, respectVCS bool) (map[string]snapshot, error) {
	snaps := make(map[string]snapshot)
	ign, err := loadIgnorer(root, respectVCS)
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		if d.IsDir() {
			if rel != "." && (defaultSkips[d.Name()] || ignores.dirSkippedByInclude(rel) || excludesOnly(ignores, rel) || ign.ignored(rel, true)) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "." || ignores.ignored(rel) || ign.ignored(rel, false) {
			return nil
		}
		if len(snaps) >= maxCached {
			return filepath.SkipAll
		}
		content, binary, rerr := readFile(path)
		if rerr == nil && binary {
			return nil // skip binary files; they aren't diffable
		}
		if rerr == nil {
			snaps[rel] = snapshot{content: content, binary: binary}
		}
		return nil
	})
	return snaps, err
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
