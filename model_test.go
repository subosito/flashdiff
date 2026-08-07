package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanRespectsDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/main.go", "package main")
	writeFile(t, dir+"/node_modules/x/index.js", "x")
	writeFile(t, dir+"/.git/config", "x")

	snaps, err := scan(dir, newIgnoreSet(nil, nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snaps["main.go"]; !ok {
		t.Error("main.go should be scanned")
	}
	if _, ok := snaps["node_modules/x/index.js"]; ok {
		t.Error("node_modules should be skipped")
	}
	if _, ok := snaps[".git/config"]; ok {
		t.Error(".git should be skipped")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	s := newFileStore()
	s.put("a.txt", snapshot{content: "hi"})
	snap, ok := s.get("a.txt")
	if !ok || snap.content != "hi" {
		t.Fatal("get/put round trip failed")
	}
	s.remove("a.txt")
	if _, ok := s.get("a.txt"); ok {
		t.Fatal("remove failed")
	}
}

func TestApplyFileEvent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/f.txt", "one\n")
	m, err := newModel(config{path: dir, respectVCS: false})
	if err != nil {
		t.Fatal(err)
	}
	defer m.watcher.close()

	// new file
	writeFile(t, dir+"/g.txt", "hello\n")
	m.applyFileEvent(fileEventMsg{rel: "g.txt"})
	if len(m.changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(m.changes))
	}
	if !m.changes[0].isNew {
		t.Error("g.txt should be marked new")
	}

	// modify it: expect a real diff
	writeFile(t, dir+"/g.txt", "hello\nworld\n")
	m.applyFileEvent(fileEventMsg{rel: "g.txt"})
	if len(m.changes) != 1 {
		t.Fatalf("expected still 1 change (upsert), got %d", len(m.changes))
	}
	if m.changes[0].diff.adds != 1 {
		t.Errorf("expected 1 add, got %d", m.changes[0].diff.adds)
	}

	// same content again: no-op
	m.applyFileEvent(fileEventMsg{rel: "g.txt"})
	if m.eventCount != 2 {
		t.Errorf("expected 2 events, got %d", m.eventCount)
	}

	// delete
	if err := os.Remove(dir + "/g.txt"); err != nil {
		t.Fatal(err)
	}
	m.applyFileEvent(fileEventMsg{rel: "g.txt", deleted: true})
	if !m.changes[0].deleted {
		t.Error("g.txt should be marked deleted")
	}
}

func TestWatcherDeliversEvent(t *testing.T) {
	dir := t.TempDir()
	got := make(chan fileEventMsg, 4)
	w, err := startWatcher(dir, newIgnoreSet(nil, nil), false, func(m fileEventMsg) { got <- m })
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()

	writeFile(t, dir+"/x.txt", "1")
	select {
	case ev := <-got:
		if ev.rel != "x.txt" || ev.deleted {
			t.Errorf("unexpected event %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for fs event")
	}
}
