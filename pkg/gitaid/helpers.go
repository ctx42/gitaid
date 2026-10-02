// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"bufio"
	"io"
	"strings"
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
