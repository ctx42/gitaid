// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid_test

import (
	"context"
	"errors"
	"fmt"
	"log"

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

func ExampleDescribe() {
	ctx := context.Background()

	// The empty string means the current working directory. Prints the
	// closest tag alone ("v1.2.0"), or with the distance from HEAD
	// ("v1.2.0-3-g9ab3d41"); a dirty tree appends "-dev".
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
