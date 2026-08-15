package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const usage = `flashdiff — watch file diffs live in your terminal

Usage:
  flashdiff [flags] [path]

Arguments:
  path               directory to watch (default: current directory)

Flags:
  -i, --include      glob of files to include (repeatable), e.g. -i '**/*.go'
  -e, --exclude      glob of files to exclude (repeatable)
      --no-vcs       do not respect .gitignore / .ignore files
      --theme        color theme (default: catppuccin-mocha)
                     catppuccin-mocha, catppuccin-macchiato, catppuccin-frappe,
                     catppuccin-latte, dracula, gruvbox, monokai, nord,
                     solarized-dark, solarized-light, tokyonight-night,
                     tokyonight-storm, onedark, doom-one
      --version      print version and exit
  -h, --help         show this help

Keys:
  j/k, up/down       select file              enter/l      focus diff
  tab                cycle panes              d            cycle diff mode
  u                  toggle word diff         /            filter files
  g/G                top / bottom             r            rescan
  c                  clear history            \\            icon sidebar
  ?                  help                     q            quit
`

// Build metadata, injected via -ldflags at release time (see .goreleaser.yaml).
// version is the tagged release (e.g. "v1.2.3"); commit and date hold the
// source revision and build date. All default to "dev"/"" for `go install` and
// local builds.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

type config struct {
	path       string
	includes   []string
	excludes   []string
	respectVCS bool
	theme      string
}

type strList []string

func (s *strList) String() string { return strings.Join(*s, ",") }
func (s *strList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func parseArgs(args []string) (config, error) {
	cfg := config{path: ".", respectVCS: true}
	fs := flag.NewFlagSet("flashdiff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var includes, excludes strList
	fs.Var(&includes, "i", "include glob (repeatable)")
	fs.Var(&includes, "include", "include glob (repeatable)")
	fs.Var(&excludes, "e", "exclude glob (repeatable)")
	fs.Var(&excludes, "exclude", "exclude glob (repeatable)")

	var noVCS, showVersion, help bool
	var theme string
	fs.BoolVar(&noVCS, "no-vcs", false, "ignore .gitignore files")
	fs.StringVar(&theme, "theme", "catppuccin-mocha", "color theme (e.g. catppuccin-mocha, dracula, gruvbox, nord, monokai)")
	fs.BoolVar(&showVersion, "version", false, "print version")
	fs.BoolVar(&help, "h", false, "show help")
	fs.BoolVar(&help, "help", false, "show help")

	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if help {
		fmt.Print(usage)
		os.Exit(0)
	}
	if showVersion {
		applyBuildIdentity()
		fmt.Printf("flashdiff %s\n", version)
		if commit != "" {
			fmt.Printf("  commit: %s\n", commit)
		}
		if date != "" {
			fmt.Printf("  built:  %s\n", date)
		}
		os.Exit(0)
	}
	if fs.NArg() > 1 {
		return cfg, fmt.Errorf("expected at most one path argument")
	}
	if fs.NArg() == 1 {
		cfg.path = fs.Arg(0)
	}
	cfg.includes = includes
	cfg.excludes = excludes
	cfg.respectVCS = !noVCS
	cfg.theme = theme
	if !knownThemes[theme] {
		return cfg, fmt.Errorf("unknown theme %q; see --help for available themes", theme)
	}
	return cfg, nil
}
