package main

import (
	"path/filepath"
	"strings"
)

func toSlash(p string) string {
	if filepath.Separator == '/' {
		return p
	}
	return strings.ReplaceAll(p, string(filepath.Separator), "/")
}

// defaultSkips are directory names never watched or scanned.
var defaultSkips = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".idea": true, ".vscode": true, "__pycache__": true,
	".next": true, ".cache": true, "coverage": true, ".direnv": true,
	".devenv": true,
}

// ignoreSet decides whether a path is excluded from watching. Patterns
// support "*", "?", and "**" and match against the slash-separated
// relative path (or any suffix segment for bare names).
type ignoreSet struct {
	includes []string
	excludes []string
}

func newIgnoreSet(includes, excludes []string) *ignoreSet {
	return &ignoreSet{includes: includes, excludes: excludes}
}

// dirSkippedByInclude reports whether a directory cannot contain any
// included file. Ancestor directories of potential matches must NOT be
// skipped, so we only skip when every include pattern is "shallow"
// (cannot match below this dir). Conservative: rarely skip.
func (s *ignoreSet) dirSkippedByInclude(rel string) bool {
	if len(s.includes) == 0 {
		return false
	}
	rel = toSlash(rel)
	for _, p := range s.includes {
		p = strings.TrimPrefix(toSlash(p), "/")
		// A pattern can still match under rel if:
		//  - it contains "**" (spans any depth), or
		//  - rel is a path-prefix of the pattern's literal leading dirs, or
		//  - the pattern has more segments than rel.
		if strings.Contains(p, "**") {
			return false
		}
		patSegs := strings.Split(p, "/")
		relSegs := strings.Split(rel, "/")
		if len(patSegs) > len(relSegs) {
			return false
		}
		// pattern is at-or-above rel depth; it may match rel itself or
		// shallower, but not below unless it globs the dir name.
		if matchGlob(p, rel) {
			return false
		}
	}
	return true
}

func (s *ignoreSet) addExclude(pattern string) {
	s.excludes = append(s.excludes, pattern)
}

// excludesOnly reports whether rel matches an exclude glob (ignoring
// includes). Used for directory pruning decisions.
func excludesOnly(s *ignoreSet, rel string) bool {
	rel = toSlash(rel)
	for _, p := range s.excludes {
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}

func (s *ignoreSet) ignored(rel string) bool {
	rel = toSlash(rel)
	if len(s.includes) > 0 {
		ok := false
		for _, p := range s.includes {
			if matchGlob(p, rel) {
				ok = true
				break
			}
		}
		if !ok {
			return true
		}
	}
	for _, p := range s.excludes {
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}

// matchGlob matches pattern against a slash-separated path. A pattern
// without a slash matches if it matches any single path segment (like a
// bare gitignore name). "**" spans multiple segments.
func matchGlob(pattern, path string) bool {
	pattern = toSlash(strings.TrimSpace(pattern))
	path = toSlash(path)
	if pattern == "" {
		return false
	}
	anchored := strings.HasPrefix(pattern, "/")
	if anchored {
		pattern = pattern[1:]
	}
	if !strings.Contains(pattern, "/") && !anchored {
		// bare name: match any single path segment (gitignore semantics)
		for _, seg := range strings.Split(path, "/") {
			if ok, _ := filepath.Match(pattern, seg); ok {
				return true
			}
		}
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

func matchSegments(pat, path []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(path); i++ {
				if matchSegments(pat[1:], path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		ok, err := filepath.Match(pat[0], path[0])
		if err != nil || !ok {
			return false
		}
		pat, path = pat[1:], path[1:]
	}
	return len(path) == 0
}
