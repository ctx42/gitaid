// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/pathkit"
)

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

	t.Run("error - not existing directory", func(t *testing.T) {
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

// tarOf returns a tar stream holding hdrs in order, each regular file with the
// content its tarFile header was given.
func tarOf(t tester.T, hdrs ...*tar.Header) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	twr := tar.NewWriter(buf)
	for _, hdr := range hdrs {
		if err := twr.WriteHeader(hdr); err != nil {
			t.Error(err)
			return nil
		}
		if _, err := io.WriteString(twr, tarContent[hdr]); err != nil {
			t.Error(err)
			return nil
		}
	}
	if err := twr.Close(); err != nil {
		t.Error(err)
		return nil
	}
	return buf
}

// tarContent holds the content tarFile gives each header it builds.
var tarContent = map[*tar.Header]string{}

// tarFile returns the header of a regular file with content for tarOf.
func tarFile(name, content string) *tar.Header {
	hdr := &tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(content)),
	}
	tarContent[hdr] = content
	return hdr
}

// tarDir returns the header of a directory for tarOf.
func tarDir(name string) *tar.Header {
	return &tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}
}
