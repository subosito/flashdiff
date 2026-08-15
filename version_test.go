package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestVERSIONFileHasNoVPrefix(t *testing.T) {
	v := strings.TrimSpace(versionFile)
	if v == "" {
		t.Fatal("VERSION embed empty")
	}
	if strings.HasPrefix(v, "v") {
		t.Fatalf("VERSION should be bare semver without v prefix, got %q", v)
	}
}

func TestApplyBuildIdentityFromVERSION(t *testing.T) {
	prevV, prevC, prevD := version, commit, date
	t.Cleanup(func() { version, commit, date = prevV, prevC, prevD })

	version, commit, date = "dev", "", ""
	applyBuildIdentity()

	want := "v" + strings.TrimSpace(versionFile)
	if version != want {
		t.Fatalf("version=%q want %q", version, want)
	}
}

func TestIsReleaseVersion(t *testing.T) {
	ok := []string{"v0.2.1", "v1.0.0", "v1.2.3-rc.1"}
	bad := []string{"(devel)", "v0.0.0-20260815123456-abcdef", "dev", "0.2.1", "", "v0.2.1-0.20260815123456-abcdef"}
	for _, s := range ok {
		if !isReleaseVersion(s) {
			t.Errorf("%q should be a release version", s)
		}
	}
	for _, s := range bad {
		if isReleaseVersion(s) {
			t.Errorf("%q should NOT be a release version", s)
		}
	}
}

func TestContrastSurfaceDiffersFromDarkBG(t *testing.T) {
	bg := lipgloss.Color("#1e1e2e")
	surf := contrastSurface(bg)
	// Compare via RGBA so different Color implementations still work.
	br, bgc, bb, _ := bg.RGBA()
	sr, sg, sb, _ := surf.RGBA()
	if br == sr && bgc == sg && bb == sb {
		t.Fatal("surface must differ from dark background")
	}
	// Light bg should also get a distinct surface.
	light := lipgloss.Color("#eff1f5")
	ls := contrastSurface(light)
	lr, lg, lb, _ := light.RGBA()
	sr, sg, sb, _ = ls.RGBA()
	if lr == sr && lg == sg && lb == sb {
		t.Fatal("surface must differ from light background")
	}
}
