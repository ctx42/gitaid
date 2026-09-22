[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE.md)
[![Go Reference](https://pkg.go.dev/badge/github.com/ctx42/gitaid.svg)](https://pkg.go.dev/github.com/ctx42/gitaid)

# gitaid

A small Go library that wraps the `git` command line and turns its output into
typed results and errors — for tools that automate git.

## Overview

`gitaid` gives you focused Go functions for the git operations release tooling
and repository automation reach for most: inspecting working-tree state, reading
history and tags, generating changelogs, and driving commits, tags, and pushes.
**It is not meant to be a complete git wrapper** — it implements just enough to
get information about commits and tags in a project's repository (and to drive
the handful of mutations release tooling needs), not to expose git's full
command surface.

Every function shells out to the installed `git` binary — there is no cgo and no
reimplementation of git. What sets it apart from calling `os/exec` yourself is
error handling: `gitaid` translates git's human-readable stderr into a small set
of **sentinel errors** you can match with `errors.Is`, so your code branches on
`ErrNotRepo` or `ErrEmptyRepo` instead of scraping message strings.

## Features

- **Repository inspection** — `IsRepo`, `IsEmpty`, `IsClean`,
  `WorkTreeStatus`, `ProjectName`, `ProjectOrigin`.
- **History and versioning** — `FirstHash`, `LatestHash`, `RevDate`,
  `ClosestTag`, `Describe`, `ChangeLog`.
- **Mutations** — `Init`, `AddRemote`, `Add`, `AddAll`, `Commit`, `Tag`, `Push`.
- **Fetch one file without cloning** — `GetFile` streams a single file from a
  remote branch or tag via `git archive`.
- **Typed sentinel errors** — branch on `errors.Is` instead of parsing stderr.
- **Context-aware** — every operation takes a `context.Context`.

## Prerequisites

- Go 1.26 or newer.
- The `git` binary on `PATH`. `GetFile` additionally requires `tar`.

## Installation

```shell
go get github.com/ctx42/gitaid
```

```go
import "github.com/ctx42/gitaid/pkg/gitaid"
```

## Usage

All functions take a `context.Context` and a repository directory. An empty
string for the directory means the current working directory.

Check whether a directory is a git repository, matching the sentinel error to
distinguish "not a repo" from a real failure:

<!-- gmdoceg:pkg/gitaid/ExampleIsRepo -->
```go
ctx := context.Background()

// The empty string means the current working directory.
switch err := gitaid.IsRepo(ctx, ""); {
case err == nil:
	fmt.Println("current directory is a git repository")
case errors.Is(err, gitaid.ErrNotRepo):
	fmt.Println("not a git repository")
default:
	log.Fatal(err)
}
```

Produce a version stamp for the current state of the repository — the
closest tag, how far HEAD has moved past it, and whether the working tree
is dirty:

<!-- gmdoceg:pkg/gitaid/ExampleDescribe -->
```go
ctx := context.Background()

// The empty string means the current working directory. Prints the
// closest tag alone ("v1.2.0"), or with the distance from HEAD
// ("v1.2.0-3-g9ab3d41"); a dirty tree appends "-dev".
name, err := gitaid.Describe(ctx, "")
if err != nil {
	log.Fatal(err)
}
fmt.Println(name)
```

Restrict which tags are considered, so a tag like `nightly` cannot shadow
the release tag a build should carry:

<!-- gmdoceg:pkg/gitaid/ExampleDescribe_withMatch -->
```go
ctx := context.Background()

// Only tags like "v1.2.0" are considered, so "nightly" is skipped -
// though the commit count still spans the commit it points at.
name, err := gitaid.Describe(ctx, "", gitaid.WithMatch("v[0-9]*"))
if err != nil {
	log.Fatal(err)
}
fmt.Println(name)
```

Generate a changelog from commit summaries since a given revision:

<!-- gmdoceg:pkg/gitaid/ExampleChangeLog -->
```go
ctx := context.Background()

// Commit summaries added since the v1.0.0 tag, oldest first. Pass "" as
// the revision to get the changelog since the first commit.
entries, err := gitaid.ChangeLog(ctx, "", "v1.0.0")
if err != nil {
	log.Fatal(err)
}
for _, line := range entries {
	fmt.Println("-", line)
}
```

Fetch a single file from a remote branch without cloning the repository:

<!-- gmdoceg:pkg/gitaid/ExampleGetFile -->
```go
ctx := context.Background()

// Fetch a single file from a remote branch without cloning the repo.
err := gitaid.GetFile(
	ctx,
	"git@github.com:ctx42/gitaid.git",
	"master",
	"go.mod",
	"/tmp/gitaid.go.mod",
)
if err != nil {
	log.Fatal(err)
}
```

## Error handling

Functions return one of these sentinel errors when git reports a recognized
condition. Match them with `errors.Is`:

| Error          | Meaning                                                 |
|----------------|---------------------------------------------------------|
| `ErrNotRepo`   | Directory is not a git repository.                      |
| `ErrEmptyRepo` | Repository is initialized but has no commits.           |
| `ErrUnkRev`    | Revision is unknown.                                    |
| `ErrNoRemote`  | Repository has no remote configured.                    |
| `ErrUnkTag`    | Searched tag does not exist.                            |
| `ErrUnkFile`   | File or directory does not exist in the repository.     |
| `ErrGit`       | git exited with an otherwise unrecognized error.        |

Any git error that maps to none of the above is returned verbatim as a plain
`error` carrying git's message.
