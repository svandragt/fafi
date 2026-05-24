# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

fafi is a self-hosted bookmark indexer and search tool. A single Go binary starts an HTTP server, fetches readable content for each bookmark, and exposes FTS5 full-text search over titles + extracted text.

## Architecture

`main.go` wires three boot steps then serves HTTP:
1. `bootEnvironment` — loads `.env` (godotenv)
2. `bootDatabase` — opens SQLite, runs migration via `bookmark.BmDb.CreateTable`, seeds sample rows
3. Goroutine: optional `integration.ImportFirefoxProfile` → `bootIndexer` (fetches missing content concurrently via `indexQueue`)
4. `bootServer` — `http.ListenAndServe`, renders `pub/index.html` (embedded via `//go:embed`)

Packages:
- `bookmark/` — domain model, SQLite access (`BmDb` global), readability extraction, FTS5 queries, content-type probing (`probe.go`)
- `integration/` — external sources (Firefox `places.sqlite` import)
- `sander/` — utility helpers: env+CLI arg resolution (`GetArgFromEnvWithDefault` — CLI flag overrides env), debug state, file/string helpers
- `pub/` — embedded HTML templates

## Schema versioning

Tracked via `PRAGMA user_version`; `bookmark.schemaVersion` is the latest the binary writes.

- **v1** (legacy, pre-versioning): FTS5 `bookmarks(url, title, text, isScraped, dateAdded)` + sibling `bookmark_meta(url, content_type)` table. Detected when `user_version=0` AND a `bookmarks` table exists.
- **v2**: FTS5 `bookmarks(url, title, text, content_type, isScraped UNINDEXED, dateAdded UNINDEXED)` — no sibling table. `UNINDEXED` skips tokenization for columns never used with `MATCH`.

Migration v1 → v2 only runs on `FAFI_RESET_INDEX=1` (since it forces a full re-fetch). Fresh databases are always created at v2. Read/write paths branch on `Database.version`. Future cleanup: once enough time has passed, drop v1 branches and either auto-migrate on boot or refuse to start with a clear message.

Config: every `FAFI_FOO_BAR` env var has a `--foo-bar` CLI equivalent (resolved through `sander.GetArgFromEnvWithDefault`). See README for the full list.

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

**Always check lint passes before pushing.**

A git pre-push hook is committed in `.githooks/pre-push`. Activate it once per clone:

```bash
git config core.hooksPath .githooks
```

A Claude Code hook also enforces this automatically on `git push` (see `~/.claude/settings.json`).

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
