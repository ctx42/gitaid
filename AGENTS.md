# AGENTS.md

This file provides guidance to coding agents working with code in this
repository.

## Overview

`gitaid` is a small Go library (module `github.com/ctx42/gitaid`) that wraps
the `git` binary. Every exported function shells out to `git` (via `os/exec`)
and translates git's stderr into typed sentinel errors. There is no long-lived
git process or CGo binding — each call runs `git` once in a target directory.

## Commands

```bash
go build ./...                                # build
go test ./... -race                           # all tests, as CI runs them
go test ./pkg/gitaid -run Test_ChangeLog      # one top-level test
go test ./pkg/gitaid -run Test_IsRepo/success # one subtest
golangci-lint run -c tmp/.golangci.yml        # lint (config is gitignored)
```

Tests exec the real `git` binary against throwaway repos created in
`t.TempDir()`, so `git` must be installed and on `PATH`. CI
(`.github/workflows/go.yml`) runs `go test -v -race ./...`.

## Architecture

The entire library is the single package `pkg/gitaid`:

- `gitkit.go` — the package godoc, the `Err*` sentinels, and the public API.
  Each function takes `(ctx, repo, ...)` where `repo` is the working directory
  (empty string means the current directory). Functions build a git argument
  slice and call the unexported `runGitCmd`, which runs `git`, trims stdout,
  and on failure passes the `fatal:`/`error:` line of stderr (`gitMessage`) to
  `gitErrorOr`.
- `helpers.go` — domain-agnostic helpers (`firstLine`).

### Conventions that matter

1. **Sentinel errors + `gitErrorOr`.** Error states are the package-level
   `Err*` sentinels at the top of `gitkit.go` (`ErrNotRepo`, `ErrEmptyRepo`,
   `ErrUnkRev`, `ErrNoTags`, `ErrNoRemote`, `ErrUnkTag`, `ErrUnkFile`,
   `ErrNotFile`, `ErrNotClean`, `ErrDetached`, `ErrBadArg`, `ErrGit`, plus
   `ErrBadBump`). `gitErrorOr` maps git's English stderr text to these via
   substring matching; an unmapped message becomes `ErrGit` wrapping the exec
   error. When handling a new git failure, add a `case` there rather than
   returning ad-hoc errors — callers rely on `errors.Is`. Because the mapping
   keys on git's human-readable messages, it is inherently
   git-version-sensitive; `gitCommand` pins `LC_ALL=C` so those messages are
   never translated.

2. **No caller value reaches git as an option.** Revisions, tags, remotes, and
   paths pass through `noOption` (returns `ErrBadArg` for a leading `-`), or
   follow `--` when a leading `-` is a legitimate path (`Add`).

3. **`runGitCmd` is the only exec path** (except `GetFile`, which streams
   `git archive` through `archive/tar` to fetch a single file from a remote
   without a full clone, and manages its own timeout/`WaitDelay`). Prefer
   routing new commands through `runGitCmd`; anything that must exec git
   builds the command with `gitCommand`.

4. **`Describe` and `Derive` share `describe`**, which returns the tag, count,
   hash, and dirty flag as fields; never re-parse the rendered `Describe`
   string. The versioning rules are in `docs/versioning.md`.

## Testing conventions

- Subtests carry explicit `// --- Given ---`, `// --- When ---`, and
  `// --- Then ---` section comments; the When result is `have`, expected
  values `want`, and error-path subtests are named `error - <condition>`.
  Test functions follow the declaration order in `gitkit.go`; table tests
  carry a `_tabular` suffix.
- Assertions use `github.com/ctx42/testing` (`assert`, `must`, `tester`). Note
  `assert.ErrorIs(t, want, err)` takes the **expected error first**.
- Test repos are built with `prjkit` helpers (e.g. `prjkit.New(t, dir)`,
  `.GitInitAddAll()`, `.CreateFileWith(...)`, `.Exe("git", ...)`) from
  `github.com/ctx42/testkit`.
- Shared test helpers live in `all_test.go`: `Bare` (a bare repo to push to or
  fetch from) and `tarOf`/`tarFile`/`tarDir` (in-memory tar streams).

## Style

- `.editorconfig`: LF, final newline, tabs for Go; Go lines stay within 80
  columns (tab width 4).
- `cyclop` (in `tmp/.golangci.yml`) caps cyclomatic complexity at 13 for
  non-test code — `gitErrorOr` carries a `//nolint:cyclop`.

## Notes

- Version lives in `VER`; release notes in `CHANGELOG.md`.
