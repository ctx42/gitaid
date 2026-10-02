// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/iokit"
)

func Test_firstLine(t *testing.T) {
	t.Run("first line", func(t *testing.T) {
		// --- Given ---
		r := strings.NewReader("first\nsecond\nthird\n")

		// --- When ---
		have := firstLine(r)

		// --- Then ---
		assert.Equal(t, "first", have)
	})

	t.Run("empty reader", func(t *testing.T) {
		// --- Given ---
		r := strings.NewReader("")

		// --- When ---
		have := firstLine(r)

		// --- Then ---
		assert.Equal(t, "", have)
	})

	t.Run("trims both sides", func(t *testing.T) {
		// --- Given ---
		r := strings.NewReader("  first    \nsecond\n")

		// --- When ---
		have := firstLine(r)

		// --- Then ---
		assert.Equal(t, "first", have)
	})

	t.Run("read error", func(t *testing.T) {
		// --- Given ---
		er := iokit.ErrReader(strings.NewReader("first\n"), 0)

		// --- When ---
		have := firstLine(er)

		// --- Then ---
		assert.Equal(t, "", have)
	})
}
