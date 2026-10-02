// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"
)

// firstLine returns the first trimmed line from a reader.
func firstLine(r io.Reader) string {
	scn := bufio.NewScanner(r)
	scn.Split(bufio.ScanLines)
	for scn.Scan() {
		return strings.TrimSpace(scn.Text())
	}
	return "" // A scan error yields the empty string, same as no input.
}

// withTimeout returns ctx unchanged when it already has a deadline, which may
// be longer than dflt, and ctx bounded by dflt otherwise.
func withTimeout(
	ctx context.Context,
	dflt time.Duration,
) (context.Context, context.CancelFunc) {

	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, dflt)
}
