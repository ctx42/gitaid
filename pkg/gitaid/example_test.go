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
