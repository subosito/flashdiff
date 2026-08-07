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
      --version      print version and exit
  -h, --help         show this help

Keys:
  j/k, up/down       select file              enter/l      focus diff
  tab                cycle panes              d            cycle diff mode
  u                  toggle word diff         /            filter files
  g/G                top / bottom             r            rescan
  c                  clear history            ?            help
  q                  quit
`

var version = "dev"

type config struct {
	path       string
	includes   []string
	excludes   []string
	respectVCS bool
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
	fs.BoolVar(&noVCS, "no-vcs", false, "ignore .gitignore files")
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
		fmt.Printf("flashdiff %s\n", version)
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
	return cfg, nil
}
