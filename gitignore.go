package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ignorer applies .gitignore / .ignore style rules found under root.
// It is intentionally small: bare names, trailing-slash dir rules,
// "*" / "?" / "**" globs, "!" negation, and per-directory scoping for
// nested ignore files.
type ignorer struct {
	rules []ignoreRule
}

type ignoreRule struct {
	dir      string // directory (relative, slash form) the rule applies under
	pattern  string
	negate   bool
	dirOnly  bool
	matchAny bool // bare name: match any segment
}

var ignoreFileNames = []string{".gitignore", ".ignore"}

func loadIgnorer(root string, respectVCS bool) (*ignorer, error) {
	ig := &ignorer{}
	if !respectVCS {
		return ig, nil
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() != "." && defaultSkips[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		for _, cand := range ignoreFileNames {
			if name == cand {
				ig.loadFile(root, path)
				break
			}
		}
		return nil
	})
	return ig, err
}

func (ig *ignorer) loadFile(root, path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()

	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return
	}
	dir := ""
	if rel != "." {
		dir = toSlash(rel)
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rule := ignoreRule{dir: dir}
		if strings.HasPrefix(line, "!") {
			rule.negate = true
			line = line[1:]
		}
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		rule.matchAny = !strings.Contains(line, "/")
		rule.pattern = line
		ig.rules = append(ig.rules, rule)
	}
}

// ignored reports whether rel (slash-separated relative path) is
// ignored. Later rules override earlier ones (gitignore semantics).
func (ig *ignorer) ignored(rel string, isDir bool) bool {
	rel = toSlash(rel)
	ignored := false
	for _, r := range ig.rules {
		if r.dirOnly && !isDir {
			continue
		}
		target := rel
		if r.dir != "" {
			if rel == r.dir {
				continue
			}
			if !strings.HasPrefix(rel, r.dir+"/") {
				continue
			}
			target = rel[len(r.dir)+1:]
		}
		if r.matchAny {
			matched := false
			for _, seg := range strings.Split(target, "/") {
				if ok, _ := filepath.Match(r.pattern, seg); ok {
					matched = true
					break
				}
			}
			if matched {
				ignored = !r.negate
			}
			continue
		}
		if matchGlob(r.pattern, target) {
			ignored = !r.negate
		}
	}
	return ignored
}
