package main

import (
	_ "embed"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

//go:embed VERSION
var versionFile string

// releaseVersionRe matches a tagged module release (optionally with a
// pre-release suffix). Pseudo-versions (vX.Y.Z-yyyymmddhhmmss-abcdef) are
// rejected by isReleaseVersion.
var releaseVersionRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

// isReleaseVersion reports whether s looks like a real release tag rather
// than a Go module pseudo-version or "(devel)".
//
// Pseudo-versions look like:
//
//	vX.Y.Z-yyyymmddhhmmss-abcdefabcdef
//	vX.Y.Z-0.yyyymmddhhmmss-abcdefabcdef
//	vX.Y.Z-N.yyyymmddhhmmss-abcdefabcdef
func isReleaseVersion(s string) bool {
	if !releaseVersionRe.MatchString(s) {
		return false
	}
	// A 14-digit UTC timestamp anywhere after the first hyphen marks a
	// pseudo-version (see https://go.dev/ref/mod#pseudo-versions).
	for i := 0; i+14 <= len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			continue
		}
		ok := true
		for j := 0; j < 14; j++ {
			if s[i+j] < '0' || s[i+j] > '9' {
				ok = false
				break
			}
		}
		if ok {
			return false
		}
	}
	return true
}

// applyBuildIdentity fills version/commit/date when ldflags did not
// (plain `go build` / `go install`). Prefer, in order:
//  1. -ldflags -X main.version=... (goreleaser, nix)
//  2. Go module version when it is a real release tag (go install @vX.Y.Z)
//  3. embedded VERSION file (repo source of truth, without "v" prefix)
//
// Keep VERSION in sync with the latest release tag (v0.2.1 → 0.2.1).
func applyBuildIdentity() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if commit == "" || date == "" {
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					if commit == "" {
						commit = s.Value
					}
				case "vcs.time":
					if date == "" {
						if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
							date = t.UTC().Format("2006-01-02")
						} else {
							date = s.Value
						}
					}
				}
			}
		}
		// go install github.com/…@v1.2.3 → Main.Version is "v1.2.3".
		// Local checkouts often report pseudo-versions; ignore those so the
		// embedded VERSION file remains the identity for source builds.
		if (version == "" || version == "dev") && isReleaseVersion(bi.Main.Version) {
			version = bi.Main.Version
		}
	}

	if version == "" || version == "dev" {
		v := strings.TrimSpace(versionFile)
		v = strings.TrimPrefix(v, "v")
		if v != "" {
			version = "v" + v
		}
	}

	// Shorten full SHAs for display.
	if len(commit) > 12 {
		commit = commit[:12]
	}
}
