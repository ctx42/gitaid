// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/ctx42/gitaid/pkg/gitaid"
)

func ExampleIsRepo() {
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
}

func ExampleDescribe() {
	ctx := context.Background()

	// The empty string means the current working directory. Always a valid
	// SemVer: the closest version tag alone ("v1.2.0"), or with the
	// distance from HEAD ("v1.2.0-3-g9ab3d41"); a dirty tree appends
	// "-dirty". With no version tag reachable the distance is counted from
	// "v0.0.0".
	name, err := gitaid.Describe(ctx, "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(name)
}

func ExampleDescribe_withMatch() {
	ctx := context.Background()

	// Only tags like "v1.2.0" are considered, so "nightly" is skipped -
	// though the commit count still spans the commit it points at.
	name, err := gitaid.Describe(ctx, "", gitaid.WithMatch("v[0-9]*"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(name)
}

func ExampleDerive() {
	ctx := context.Background()

	// A clean checkout on tag "v0.4.0" is the release "v0.4.0". Three commits
	// later with a dirty tree, a patch bump gives "v0.4.1-dev.3.dirty+g<hash>",
	// which sorts above v0.4.0 and below v0.4.1.
	ver, err := gitaid.Derive(ctx, "", gitaid.BumpPatch)
	if err != nil {
		log.Fatal(err)
	}
	if ver.Release {
		fmt.Println("release", ver.Rev)
		return
	}
	fmt.Println("development build", ver.Rev)
}

func ExampleMessages() {
	ctx := context.Background()

	// Full messages since the v1.0.0 tag, oldest first, so a footer such as
	// "BREAKING CHANGE:" can pick the bump for Derive.
	msgs, err := gitaid.Messages(ctx, "", "v1.0.0..HEAD")
	if err != nil {
		log.Fatal(err)
	}
	bump := gitaid.BumpPatch
	for _, msg := range msgs {
		if strings.Contains(msg, "BREAKING CHANGE:") {
			bump = gitaid.BumpMajor
		}
	}
	fmt.Println(bump)
}

func ExampleChangeLog() {
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
}

func ExampleGetFile() {
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
}
