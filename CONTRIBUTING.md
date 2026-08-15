# Contributing

Thanks for your interest in flashdiff! This document covers what you need to
build, test, and propose changes.

## Requirements

- **Go 1.25+** — that's the only hard requirement. Everything else is optional.
- `devenv.nix` is provided for convenience but is entirely optional; you do not
  need Nix or devenv to contribute.

## Build, test, lint

```sh
go build ./...        # build
go test ./...         # run tests
gofmt -l .            # should print nothing (run `gofmt -w .` to fix)
go vet ./...          # static checks (golangci-lint runs a superset in CI)
```

Please make sure the tree builds, tests pass, and `gofmt`/`go vet` are clean
before opening a PR.

## Commit style

This project uses [Conventional Commits](https://www.conventionalcommits.org/).
Keep commits small and focused — one logical change per commit:

```
feat: add --watch-debounce flag
fix: render pane titles when terminal is 1 row shorter
refactor: extract ignore-set matching from watcher
chore: bump bubbletea to v1.3.10
docs: document diff modes in README
```

Release notes are generated from commit messages, so a clear message means a
clear changelog.

## Pull requests

1. Fork and create a branch from `main`.
2. Make your change with focused commits (see above).
3. Add or update tests where it makes sense.
4. Open a PR against `main` and describe the *why*, not just the *what*.

Bug reports and feature ideas are welcome as
[issues](https://github.com/subosito/flashdiff/issues). A short description of
the behaviour you expected vs. what you saw (plus your OS and `flashdiff
--version`) is usually enough to get started.

## Releases

Version is defined in the root [`VERSION`](VERSION) file (bare semver, no `v`).
When cutting a release, bump that file and tag `v` + the same number. The Nix
flake and the embedded `--version` output both read it.
