// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/iokit"
)

func Test_exitStatus(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		// --- When ---
		code := exitStatus(nil)

		// --- Then ---
		assert.Equal(t, 0, code)
	})

	t.Run("has ExitStatus method", func(t *testing.T) {
		// --- Given ---
		tmp := TError{Err: "test error", ExStatus: 123}

		// --- When ---
		code := exitStatus(tmp)

		// --- Then ---
		assert.Equal(t, 123, code)
	})

	t.Run("is ExitError instance", func(t *testing.T) {
		// --- Given ---
		cmd := exec.Command(os.Args[0], "--exitCode", "99")
		err := cmd.Run()

		// --- When ---
		code := exitStatus(err)

		// --- Then ---
		assert.Equal(t, 99, code)
	})

	t.Run("exit code 0", func(t *testing.T) {
		// --- Given ---
		sout, eout := iokit.WetBuffer(t), iokit.DryBuffer(t)
		cmd := exec.Command(os.Args[0], "--exitCode", "0", "--toStdout", "abc")
		cmd.Stdout = sout
		cmd.Stderr = eout
		cmd.Env = exekit.MaybeAddGoCovDir(os.Environ(), os.Args, t.TempDir)

		err := cmd.Run()

		// --- When ---
		code := exitStatus(err)

		// --- Then ---
		assert.Equal(t, 0, code)
		assert.Equal(t, "|sout: abc|", sout.String())
	})

	t.Run("unknown error", func(t *testing.T) {
		// --- When ---
		code := exitStatus(ErrTest)

		// --- Then ---
		assert.Equal(t, 1, code)
	})
}
