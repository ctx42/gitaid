// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/pathkit"
	"github.com/ctx42/testkit/pkg/selfkit"
)

func TestMain(m *testing.M) {
	runTests, exitCode := selfkit.New().Run(os.Stdout, os.Stderr)
	if runTests {
		os.Exit(m.Run())
	}
	os.Exit(exitCode)
}

func Test_Bare(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// --- Given ---
		tspy := tester.New(t)
		tspy.ExpectTempDir(1)
		tspy.Close()

		// --- When ---
		repo := Bare(tspy)

		// --- Then ---
		assert.DirExist(t, repo)

		args := []string{"rev-parse", "--is-bare-repository"}
		out := exekit.New(t, exekit.WithWd(repo)).ExeStdout("git", args...)
		assert.Equal(t, "true\n", out)

		tspy.Finish()
		assert.NoDirExist(t, repo)
		assert.Equal(t, tspy.GetTempDir(0), repo)
	})

	t.Run("not existing directory", func(t *testing.T) {
		// --- Given ---
		dir := filepath.Join(t.TempDir(), "not_existing")

		tspy := tester.New(t)
		tspy.ExpectError()
		tspy.ExpectLogContain("no such file or directory")
		tspy.ExpectLogContain(dir)
		tspy.Close()

		// --- When ---
		repo := Bare(tspy, dir)

		// --- Then ---
		assert.Empty(t, repo)
	})

	t.Run("always returns absolute path", func(t *testing.T) {
		// --- Given ---
		wd := oskit.Chdir(t, t.TempDir())
		dir := pathkit.EvalSymlinks(t, oskit.MkdirAll(t, wd, "repo"))

		tspy := tester.New(t)
		tspy.Close()

		// --- When ---
		repo := Bare(tspy, "repo")

		// --- Then ---
		assert.Equal(t, dir, repo)
	})
}

// ErrTest is a sentinel error used in tests.
var ErrTest = errors.New("test error")

// Bare creates a bare git repository and returns the absolute path to it (even
// when the provided path elements are relative). When no path elements are
// provided, the repository is created in a directory from t.TempDir. It is
// used as a push/fetch remote in tests.
func Bare(t tester.T, elems ...string) string {
	t.Helper()
	var dir string
	var err error

	if len(elems) == 0 {
		dir = t.TempDir()
	} else {
		dir = filepath.Join(elems...)
		if !filepath.IsAbs(dir) {
			if dir, err = filepath.Abs(dir); err != nil {
				t.Error(err)
				return ""
			}
		}
	}

	eout := &bytes.Buffer{}
	cmd := exec.Command("git", "init", "--bare")
	cmd.Stdout, cmd.Stderr = io.Discard, eout
	cmd.Dir = dir
	if err = cmd.Run(); err != nil {
		t.Error(fmt.Errorf("%w: %s", err, eout.String()))
		return ""
	}
	return dir
}

// TError is a test structure implementing error and exitStatus interfaces.
type TError struct {
	Err      string
	ExStatus int
}

func (e TError) Error() string   { return e.Err }
func (e TError) ExitStatus() int { return e.ExStatus }
