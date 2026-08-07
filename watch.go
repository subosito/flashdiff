package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceWindow = 60 * time.Millisecond

// fileEventMsg is sent to the bubbletea program when a file changed on
// disk. deleted is true when the file no longer exists.
type fileEventMsg struct {
	rel     string
	deleted bool
}

// watch manages an fsnotify watcher with debouncing. Events are
// delivered to the bubbletea program through send.
type watcher struct {
	fsn    *fsnotify.Watcher
	root   string
	ign    *ignoreSet
	vcs    *ignorer
	send   func(fileEventMsg)
	cancel chan struct{}

	mu       sync.Mutex
	pending  map[string]bool // rel -> deleted
	debounce *time.Timer
}

func startWatcher(root string, ign *ignoreSet, respectVCS bool, send func(fileEventMsg)) (*watcher, error) {
	fsn, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	vcs, err := loadIgnorer(root, respectVCS)
	if err != nil {
		fsn.Close()
		return nil, err
	}
	w := &watcher{
		fsn:     fsn,
		root:    root,
		ign:     ign,
		vcs:     vcs,
		send:    send,
		cancel:  make(chan struct{}),
		pending: make(map[string]bool),
	}
	if err := w.addDirs(root); err != nil {
		fsn.Close()
		return nil, err
	}
	go w.loop()
	return w, nil
}

func (w *watcher) addDirs(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(w.root, path)
		if rerr != nil {
			return nil
		}
		if rel != "." && (defaultSkips[d.Name()] || w.ign.dirSkippedByInclude(rel) || excludesOnly(w.ign, rel) || w.vcs.ignored(rel, true)) {
			return filepath.SkipDir
		}
		if err := w.fsn.Add(path); err != nil {
			return nil // unreadable dir: skip, don't fail the walk
		}
		return nil
	})
}

func (w *watcher) loop() {
	for {
		select {
		case <-w.cancel:
			return
		case ev, ok := <-w.fsn.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fsn.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *watcher) handle(ev fsnotify.Event) {
	rel, err := filepath.Rel(w.root, ev.Name)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	rel = toSlash(rel)

	// Newly created directories must be added recursively so their
	// files get watched.
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if !defaultSkips[info.Name()] && !w.ign.ignored(rel) && !w.vcs.ignored(rel, true) {
				w.addDirs(ev.Name)
			}
			return
		}
	}
	if w.ign.ignored(rel) || w.vcs.ignored(rel, false) {
		return
	}

	deleted := ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)
	if !deleted {
		if _, err := os.Stat(ev.Name); err != nil {
			deleted = true
		}
	}

	w.mu.Lock()
	// Record the final observed existence, not a merge: a create after a
	// remove means the file exists again; a remove is authoritative.
	w.pending[rel] = deleted
	if w.debounce == nil {
		w.debounce = time.AfterFunc(debounceWindow, w.flush)
	}
	w.mu.Unlock()
}

func (w *watcher) flush() {
	w.mu.Lock()
	batch := w.pending
	w.pending = make(map[string]bool)
	w.debounce = nil
	w.mu.Unlock()
	for rel, deleted := range batch {
		w.send(fileEventMsg{rel: rel, deleted: deleted})
	}
}

func (w *watcher) close() {
	close(w.cancel)
	w.mu.Lock()
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.mu.Unlock()
	w.fsn.Close()
}
