// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"context"
	"strings"
	"testing"
	"time"

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

func Test_withTimeout(t *testing.T) {
	t.Run("deadline already set", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeout(t.Context(), time.Hour)
		t.Cleanup(cxl)
		want, _ := ctx.Deadline()

		// --- When ---
		have, hCxl := withTimeout(ctx, time.Second)

		// --- Then ---
		t.Cleanup(hCxl)
		tim, ok := have.Deadline()
		assert.True(t, ok)
		assert.Equal(t, want, tim)
	})

	t.Run("no deadline", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()

		// --- When ---
		have, hCxl := withTimeout(ctx, time.Minute)

		// --- Then ---
		t.Cleanup(hCxl)
		tim, ok := have.Deadline()
		assert.True(t, ok)
		assert.Within(t, time.Now().Add(time.Minute), "1s", tim)
	})
}
