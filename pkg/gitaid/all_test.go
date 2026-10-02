// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/exekit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/pathkit"
	"github.com/ctx42/testkit/pkg/prjkit"
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

// tarEntry is one entry of a tar stream tarOf builds.
type tarEntry struct {
	hdr     *tar.Header // The entry header.
	content string      // The content of a regular file.
}

// tarOf returns a tar stream holding ents in order.
func tarOf(t tester.T, ents ...tarEntry) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	twr := tar.NewWriter(buf)
	for _, ent := range ents {
		if err := twr.WriteHeader(ent.hdr); err != nil {
			t.Error(err)
			return nil
		}
		if _, err := io.WriteString(twr, ent.content); err != nil {
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

// tarFile returns the entry of a regular file with content for tarOf.
func tarFile(name, content string) tarEntry {
	hdr := &tar.Header{
		Name:     name,
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(content)),
	}
	return tarEntry{hdr: hdr, content: content}
}

// tarDir returns the entry of a directory for tarOf.
func tarDir(name string) tarEntry {
	hdr := &tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0o755}
	return tarEntry{hdr: hdr}
}

// signCommits makes git sign every later commit in the repository of prj with
// a new SSH key. It skips the test when ssh-keygen is not installed.
func signCommits(t tester.T, prj *prjkit.Project) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
		return
	}
	key := filepath.Join(t.TempDir(), "key")
	prj.Exe("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	prj.Exe("git", "config", "gpg.format", "ssh")
	prj.Exe("git", "config", "user.signingkey", key)
	prj.Exe("git", "config", "commit.gpgSign", "true")
}

// showSignatures turns log.showSignature on for every git run after it, as a
// user's configuration would, so "git log" and "git show" print a signature
// check in front of each commit.
func showSignatures(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "log.showSignature")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
}

// moFile returns a GNU gettext catalog translating each key of msgs to its
// value, the format git loads from GIT_TEXTDOMAINDIR.
func moFile(msgs map[string]string) []byte {
	msgs = maps.Clone(msgs)
	msgs[""] = "Content-Type: text/plain; charset=UTF-8\n"
	ids := slices.Sorted(maps.Keys(msgs))

	// The header and both string tables come first, then the strings.
	cnt := uint32(len(ids))
	strs := 28 + 16*cnt
	var ori, trn, data bytes.Buffer
	put := func(tbl *bytes.Buffer, s string) {
		ent := []uint32{uint32(len(s)), strs + uint32(data.Len())}
		_ = binary.Write(tbl, binary.LittleEndian, ent)
		data.WriteString(s + "\x00")
	}
	for _, id := range ids {
		put(&ori, id)
	}
	for _, id := range ids {
		put(&trn, msgs[id])
	}

	buf := &bytes.Buffer{}
	hdr := []uint32{0x950412de, 0, cnt, 28, 28 + 8*cnt, 0, strs}
	_ = binary.Write(buf, binary.LittleEndian, hdr)
	buf.Write(ori.Bytes())
	buf.Write(trn.Bytes())
	buf.Write(data.Bytes())
	return buf.Bytes()
}
