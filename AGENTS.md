# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working
with code in this repository.

## Overview

`gitkit` is a small Go library (module `github.com/ctx42/gitaid`) that wraps the
`git` binary. Every exported function shells out to `git` (via `os/exec`) and
translates git's stderr into typed sentinel errors. There is no long-lived git
process or CGo bindings — each call runs `git` once in a target directory.

## Commands

```bash
go build ./...                                  # build
go test ./...                                   # run all tests
go test ./pkg/gitkit -run Test_ChangeLog        # run a single test (by top-level func)
go test ./pkg/gitkit -run Test_IsRepo/success   # run a single subtest
golangci-lint run                               # lint (config lives in tmp/.golangci.yml)
```

Tests exec the real `git` (and `tar`) binaries against throwaway repos created
in `t.TempDir()`, so `git` must be installed and on `PATH`.

## Architecture

The entire library is the single package `pkg/gitkit`:

- `gmgit.go` — the public API. Each function takes `(ctx, repo, ...)` where
  `repo` is the working directory (empty string means the current directory).
  Functions build a git argument slice and call the unexported `runGitCmd`,
  which runs `git`, trims stdout, and on failure passes stderr's first line to
  `gitErrorOr`.
- `helpers.go` — `exitStatus`, extracting a process exit code from an error.

### Two conventions that matter

1. **Sentinel errors + `gitErrorOr`.** All error states are the package-level
   `Err*` sentinels declared at the top of `gmgit.go` (`ErrNotRepo`,
   `ErrEmptyRepo`, `ErrUnkRev`, `ErrNoTags`, `ErrNoRemote`, `ErrUnkTag`,
   `ErrUnkFile`, `ErrNotClean`, `ErrGit`). `gitErrorOr` maps git's English
   stderr text to these via substring matching. When adding a function or
   handling a new git failure, add a `case` there rather than returning ad-hoc
   errors — callers rely on `errors.Is`. Because the mapping keys on git's
   human-readable messages, it is inherently git-version-sensitive.

2. **`runGitCmd` is the only exec path** (except `GetFile`, which pipes
   `git archive | tar -xO` to fetch a single file from a remote without a full
   clone, and manages its own timeout/`WaitDelay`). Prefer routing new commands
   through `runGitCmd`.

## Testing conventions

- Table/subtest style with explicit `// --- Given --- / --- When --- / --- Then ---`
  section comments. Match this layout in new tests.
- Assertions use `github.com/ctx42/testing` (`assert`, `tester`). Note
  `assert.ErrorIs(t, want, err)` takes the **expected error first**.
- Test repos are built with `prjkit` helpers (e.g. `prjkit.New(t, t.TempDir())`,
  `.GitInitAddAll()`, `.CreateFileWith(...)`, `.Exe("git", ...)`) from
  `bitbucket.org/vrinf/skw-kit`.
- `TestMain` (in `all_test.go`) wires up `selfkit`, which lets a test re-invoke
  the compiled test binary as a fake subprocess via flags like `--exitCode`,
  `--toStdout`. This is how `exitStatus` is tested against a real
  `*exec.ExitError`.

## Style

- `.editorconfig`: LF, 4-space indent, 80-column max line length, and **no**
  final newline inserted.
- Lint config is `tmp/.golangci.yml` (gitignored path); `cyclop` caps
  cyclomatic complexity at 13 for non-test code — `gitErrorOr` carries a
  `//nolint: cyclop`.

## Notes

- The branch `move-to-ctx42` is mid-migration from `bitbucket.org/vrinf/skw-kit`
  to `github.com/ctx42/*` dependencies; `go.mod`/`go.sum` may be in flux and
  `go test` can fail on a missing dependency until the migration settles.
- Version lives in `VER`; release notes in `CHANGELOG.md`.
