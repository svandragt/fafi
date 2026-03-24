# fafi development notes

## Build

```bash
GOTOOLCHAIN=local go build --tags fts5 -o tmp/fafi2 fafi2
```

The `fts5` tag enables the SQLite FTS5 full-text search extension (required).
`GOTOOLCHAIN=local` prevents Go from attempting to auto-download a newer toolchain.

## Test

```bash
GOTOOLCHAIN=local go test -race ./...
```

Always run with `-race` to catch data races. Follow **red-green TDD**: write the failing test first, then make it pass.

## Lint

```bash
GOTOOLCHAIN=local golangci-lint run
```

**Always check lint passes before pushing.** A pre-push hook enforces this automatically (see `~/.claude/settings.json`).

Configuration is in `.golangci.yml`:
- `run.go: "1.24"` — targets Go 1.24 semantics; prevents false typecheck errors from Go stdlib files with `//go:build go1.26` constraints
- `build-tags: [fts5]` — ensures lint sees the same code as the build

## Go version

- Module requires `go 1.24.7` (see `go.mod`)
- Local toolchain: `go1.24.7`
- Use `GOTOOLCHAIN=local` in all local commands to prevent auto-download attempts

## Workflow learnings

- `actions/setup-go@v5` has built-in caching — no need for a separate `actions/cache` step
- `go mod tidy && git diff --exit-code go.mod go.sum` in CI catches drift that a silent tidy masks
- Single-element `strategy.matrix` adds overhead with no benefit; set `go-version` directly
- `go build` exits non-zero on failure — a separate file-existence check is redundant
- The `for {}` loop in `bootIndexer` was unconditionally terminated (`SA4004`) — removed the dead wrapper
- Goroutines capturing an outer `err` variable after a void function call is a logic bug — always check what a function actually returns
