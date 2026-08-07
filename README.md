# flashdiff

**Watch file diffs live in your terminal.** flashdiff is a minimal TUI that
monitors a directory and shows you a live diff the moment any file is written —
ideal alongside code generators, formatters, migrations, codemods, and any tool
that rewrites files while you watch.

It complements `git diff`: instead of a post-hoc report, you get an instant
feedback loop while changes are still happening.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles), and
[Lip Gloss](https://github.com/charmbracelet/lipgloss). The theme is
[Catppuccin Mocha](https://github.com/catppuccin/catppuccin) via
[Chroma](https://github.com/alecthomas/chroma), which also provides syntax
highlighting for unchanged lines in the diff view.

---

## Install

```sh
go install github.com/subosito/flashdiff@latest
```

Or build from source:

```sh
git clone https://github.com/subosito/flashdiff.git
cd flashdiff
go build -o flashdiff .
```

## Usage

```sh
flashdiff [flags] [path]
```

`path` defaults to the current directory. flashdiff snapshots every watched
file at startup, then diffs each subsequent write against the last-known
content.

### Flags

| Flag | Description |
|------|-------------|
| `-i`, `--include` | Glob of files to include (repeatable), e.g. `-i '**/*.go'` |
| `-e`, `--exclude` | Glob of files to exclude (repeatable) |
| `--no-vcs` | Do not respect `.gitignore` / `.ignore` files |
| `--version` | Print version and exit |
| `-h`, `--help` | Show help |

### Examples

```sh
# Watch the current directory
flashdiff

# Watch only Go files under a project
flashdiff -i '**/*.go' ~/code/myapp

# Watch but exclude generated output
flashdiff -e 'dist/**' -e '*.tmp' ./src
```

---

## Interface

```
 ◉ flashdiff  /path/to/project                       filter: /___
 128 tracked · 3 changes · last 2s ago
 FILES                      │ DIFF main.go +12 -3 · compact · words
 ───────────────────────────┼──────────────────────────────────────────
 ● main.go                 M│  12   func main() {
 ✚ internal/new.go         A│  13 -     run()
 ✖ old.txt                 D│  13 +     run(ctx)
                            │  14   }
                            │  ⋮ 96 unchanged lines
 k/↑ up · j/↓ down · tab switch pane · d diff mode · ? help · q quit
```

Panes have no box borders — the focused pane is indicated by a colored
**underline** beneath its title (blue when focused, muted gray when not).

- **Header** — brand, the watched path, and the live filter input.
- **Stats bar** — tracked-file count, total changes, time since last change,
  uptime, and the selected file's `+adds`/`-dels`.
- **FILES** — every changed file, newest first. Icons: `●` modified,
  `✚` new, `✖` deleted, `◆` binary.
- **DIFF** — the selected file's diff with line numbers, additions in green,
  deletions in red, word-level highlighting, and Catppuccin Mocha syntax
  highlighting on unchanged lines.
- **Footer** — contextual key hints.

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓`, `k` / `↑` | Move file selection |
| `tab` | Switch pane focus |
| `enter` / `l` | Focus the diff pane |
| `d` | Cycle diff mode: unified → compact → split |
| `u` | Toggle word-level highlighting |
| `/` | Filter the file list |
| `g` / `G` | Jump to top / bottom |
| `r` | Rescan the watched tree |
| `c` | Clear change history |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

The divider between panes is draggable, files can be clicked, and the mouse
wheel scrolls whichever pane is under the cursor.

## Diff modes

- **compact** *(default)* — unified, but long runs of unchanged lines collapse
  into a `⋮ N unchanged lines` marker, so you focus on what changed.
- **unified** — classic inline `+`/`-` diff with full context.
- **split** — side-by-side old (left) vs new (right).

Press `d` to cycle (compact → unified → split). Press `u` to toggle word-level
highlighting, which marks the exact tokens that changed within a modified line.

## How it works

1. **Baseline** — at startup, flashdiff walks the tree (honoring
   `.gitignore`/`.ignore`, skip-lists, and your include/exclude globs) and
   caches each file's content.
2. **Watch** — a recursive `fsnotify` watcher streams events; rapid writes are
   debounced and batched.
3. **Diff** — on each event the file is re-read (with a settle check so partial
   writes don't corrupt the baseline), compared against the cached content with
   a line-level diff, and the cache is updated.
4. **Render** — the newest change floats to the top of the list and its diff is
   shown, with word-level segments computed for paired changed lines.

Binary files (NUL-byte heuristic) and files over 1 MiB are shown as a
placeholder rather than diffed.

## Development

```sh
go build ./...   # build
go test ./...    # tests
go vet ./...     # vet
```

## License

MIT
