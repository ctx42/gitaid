// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"archive/tar"
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/testkit/pkg/randkit"
)

func Test_IsRepo(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := IsRepo(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		err := IsRepo(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.Close()

		// --- When ---
		err := IsRepo(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
	})
}

func Test_IsEmpty(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := IsEmpty(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.False(t, have)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := IsEmpty(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
	})

	t.Run("not empty", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := IsEmpty(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, have)
	})
}

func Test_Branch(t *testing.T) {
	t.Run("checked out branch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "branch", "-M", "feature/x")
		prj.Close()

		// --- When ---
		have, err := Branch(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "feature/x", have)
	})

	t.Run("error - detached head", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "checkout", "--detach")
		prj.Close()

		// --- When ---
		have, err := Branch(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrDetached, err)
		assert.Empty(t, have)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := Branch(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})
}

func Test_ProjectName(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("repo url", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", prjkit.GitOrigin)
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("repo url without .git", func(t *testing.T) {
		// --- Given ---
		origin := "example.com:vr/skw-proj"

		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", origin)
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "skw-proj", have)
	})

	t.Run("scp url without path", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", "git@server:skw-proj.git")
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "skw-proj", have)
	})

	t.Run("url with trailing slash", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		origin := "https://example.com/vr/skw-proj.git/"

		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", origin)
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "skw-proj", have)
	})

	t.Run("repository without origin", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("without origin from a subdirectory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()
		sub := oskit.MkdirAll(t, prj.Root(), "sub")

		// --- When ---
		have, err := ProjectName(ctx, sub)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("bare repository without origin", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		repo := Bare(t, oskit.MkdirAll(t, t.TempDir(), "project.git"))

		// --- When ---
		have, err := ProjectName(ctx, repo)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("empty repo resolves current directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()
		t.Chdir(prj.Root())

		// --- When ---
		have, err := ProjectName(ctx, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})
}

func Test_topLevelName(t *testing.T) {
	t.Run("work tree", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := topLevelName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("bare repository", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		repo := Bare(t, oskit.MkdirAll(t, t.TempDir(), "project.git"))

		// --- When ---
		have, err := topLevelName(ctx, repo)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", have)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := topLevelName(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - inside the git directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()
		gitDir := filepath.Join(prj.Root(), ".git")

		// --- When ---
		have, err := topLevelName(ctx, gitDir)

		// --- Then ---
		assert.ErrorContain(t, "must be run in a work tree", err)
		assert.Empty(t, have)
	})
}

func Test_ProjectOrigin(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("repository without origin", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", prjkit.GitOrigin)
		prj.Close()

		// --- When ---
		have, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prjkit.GitOrigin, have)
	})

	t.Run("error - config line exceeds scanner limit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		big := strings.Repeat("a", bufio.MaxScanTokenSize+1)
		prj.Exe("git", "config", "--local", "gitaid.big", big)
		prj.Close()

		// --- When ---
		have, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, bufio.ErrTooLong, err)
		assert.Empty(t, have)
	})
}

func Test_FirstHash(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := FirstHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := FirstHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, have)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.Close()

		// --- When ---
		have, err := FirstHash(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, cm.Hash, have)
	})
}

func Test_LatestHash(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, have)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, cm.Hash, have)
	})

	t.Run("short core abbrev setting", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "config", "core.abbrev", "5")
		prj.Close()

		// --- When ---
		have, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		want := prj.ExeStdout("git", "rev-parse", "--short=7", "HEAD")
		assert.Equal(t, strings.TrimSpace(want), have)
	})
}

func Test_RevDate(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), "0000")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Zero(t, have)
	})

	t.Run("error - empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), "0000")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.Zero(t, have)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm1 := prj.GitInitAddAll()
		time.Sleep(time.Second)
		prj.CreateFileWith("file0 2", "file0.txt")
		cm2 := prj.GitCommit("")
		prj.Close()

		first := must.Value(RevDate(ctx, prj.Root(), cm1.Hash))

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), cm2.Hash)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, first.Before(have))
		assert.Within(t, time.Now(), "3s", have)
	})

	t.Run("error - not existing revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), "not_existing")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.Zero(t, have)
	})

	t.Run("annotated tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll()
		prj.Exe("git", "tag", "-a", "-m", "release", "v0.1.0")
		prj.Close()

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, must.Value(RevDate(ctx, prj.Root(), cm.Hash)), have)
	})

	t.Run("error - revision is not a commit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), "HEAD^{tree}")

		// --- Then ---
		assert.ErrorContain(t, "expected commit type", err)
		assert.Zero(t, have)
	})

	t.Run("error - revision looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()
		out := filepath.Join(t.TempDir(), "out.txt")
		rev := "--output=" + out

		// --- When ---
		have, err := RevDate(ctx, prj.Root(), rev)

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.Zero(t, have)

		assert.NoFileExist(t, out)
	})
}

func Test_ClosestTag(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have)
	})

	t.Run("one commit no tags", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have)
	})

	t.Run("one commit and startRev used", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("HEAD tagged", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("one commit after tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "commit 2")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("dirty work dir", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("edit", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("one commit after tag and dirty work dir", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "commit", "-am", "commit 1")
		prj.CreateFileWith("edit", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("starting at", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "commit", "-am", "commit 1")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "commit 2")
		prj.Exe("git", "tag", "v0.2.0")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "v0.2.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("error - unknown start revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "v9.9.9")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.ErrorContain(t, "v9.9.9", err)
		assert.Empty(t, have)
	})

	t.Run("error - start revision in empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.Empty(t, have)
	})

	t.Run("error - start revision in not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - start revision looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		have, err := ClosestTag(ctx, prj.Root(), "--all")

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.Empty(t, have)
	})
}

func Test_WithMatch(t *testing.T) {
	t.Run("sets the match glob", func(t *testing.T) {
		// --- Given ---
		var cfg describeCfg

		// --- When ---
		WithMatch("v[0-9]*")(&cfg)

		// --- Then ---
		assert.Equal(t, "v[0-9]*", cfg.match)
	})
}

func Test_isSemVer_tabular(t *testing.T) {
	tt := []struct {
		testN string

		tag  string
		want bool
	}{
		{"with v prefix", "v1.2.3", true},
		{"without v prefix", "1.2.3", true},
		{"zero version", "v0.0.0", true},
		{"pre-release", "v1.0.0-rc.1", true},
		{"build metadata", "v1.0.0+g9ab3d41", true},
		{"pre-release and build", "v1.0.0-rc.1+g9ab3d41", true},
		{"describe output", "v1.2.0-3-g9ab3d41", true},
		{"describe output dirty", "v1.2.0-3-g9ab3d41-dirty", true},
		{"moving pointer", "nightly", false},
		{"build stamp", "build-42", false},
		{"hierarchical name", "rel/v1.0.0", false},
		{"date stamp", "2026-01-15", false},
		{"missing patch", "v1.2", false},
		{"leading zero", "v1.02.3", false},
		{"empty", "", false},
		{"trailing dash", "v1.2.3-", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isSemVer(tc.tag)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_Describe(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, have)
	})

	t.Run("one commit no tags", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.0.0-1-g"+cm.Hash, have)
	})

	t.Run("no tags and dirty work dir", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll()
		prj.CreateFileWith("edit", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.0.0-1-g"+cm.Hash+"-dirty", have)
	})

	t.Run("HEAD tagged", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("one commit after tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.Exe("git", "tag", "v1.2.0")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 3")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("v1.2.0-1-g%s", prj.GitHash()), have)
	})

	t.Run("closest tag is not a version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("nightly")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		// "nightly" counts as no tag, and "v0.1.0" is not reached for.
		assert.Equal(t, "v0.0.0-2-g"+cm.Hash, have)
	})

	t.Run("tag with a pre-release is a version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.0.0-rc.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0-rc.1-1-g"+cm.Hash, have)
	})

	t.Run("dirty work dir", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("edit", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0-dirty", have)
	})

	t.Run("an untracked file makes the tree dirty", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("stray", "untracked.txt")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0-dirty", have)
	})

	t.Run("one commit after tag and dirty work dir", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitCommit("")
		prj.CreateFileWith("edit", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("v0.1.0-1-g%s-dirty", cm.Hash), have)
	})

	t.Run("match skips non-version tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("nightly")
		prj.Close()
		opt := WithMatch("v[0-9]*")

		// --- When ---
		have, err := Describe(ctx, prj.Root(), opt)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0-1-g"+cm.Hash, have)
	})

	t.Run("match skips every tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		cm := prj.GitInitAddAll("v0.1.0")
		prj.Close()
		opt := WithMatch("rel-*")

		// --- When ---
		have, err := Describe(ctx, prj.Root(), opt)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.0.0-1-g"+cm.Hash, have)
	})

	t.Run("nil option is ignored", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root(), nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", have)
	})

	t.Run("short core abbrev setting", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Exe("git", "config", "core.abbrev", "5")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		hash := prj.ExeStdout("git", "rev-parse", "--short=7", "HEAD")
		assert.Equal(t, "v0.1.0-1-g"+strings.TrimSpace(hash), have)
	})

	t.Run("error - bare repository", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		bare := t.TempDir()
		prj.Exe("git", "clone", "--bare", prj.Root(), bare)
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, bare)

		// --- Then ---
		assert.ErrorContain(t, "must be run in a work tree", err)
		assert.Empty(t, have)
	})
}

func Test_description_String_tabular(t *testing.T) {
	tt := []struct {
		testN string

		dsc  description
		want string
	}{
		{"on tag", description{"v1.2.0", 0, "9ab3d41", false}, "v1.2.0"},
		{
			"on tag dirty",
			description{"v1.2.0", 0, "9ab3d41", true},
			"v1.2.0-dirty",
		},
		{
			"past tag",
			description{"v1.2.0", 3, "9ab3d41", false},
			"v1.2.0-3-g9ab3d41",
		},
		{
			"past tag dirty",
			description{"v1.2.0", 3, "9ab3d41", true},
			"v1.2.0-3-g9ab3d41-dirty",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := tc.dsc.String()

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_splitDescribe_tabular(t *testing.T) {
	tt := []struct {
		testN string

		desc  string
		wTag  string
		wCnt  string
		wHash string
		wOK   bool
	}{
		{"on tag", "v1.2.0-0-g9ab3d41", "v1.2.0", "0", "9ab3d41", true},
		{"past tag", "v1.2.0-3-g9ab3d41", "v1.2.0", "3", "9ab3d41", true},
		{
			"tag holding a dash",
			"rel-1.0-2-g9ab3d41",
			"rel-1.0",
			"2",
			"9ab3d41",
			true,
		},
		{
			"tag holding -g",
			"v1.0.0-gamma-2-g9ab3d41",
			"v1.0.0-gamma",
			"2",
			"9ab3d41",
			true,
		},
		{"no hash marker", "v1.2.0", "", "", "", false},
		{"no count field", "v1.2.0-g9ab3d41", "", "", "", false},
		{"hash not hex", "v1.2.0-3-gzzzzzzz", "", "", "", false},
		{"count not a number", "v1.2.0-x-g9ab3d41", "", "", "", false},
		{"no tag part", "-3-g9ab3d41", "", "", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			hTag, hCnt, hHash, hOK := splitDescribe(tc.desc)

			// --- Then ---
			assert.Equal(t, tc.wOK, hOK)
			assert.Equal(t, tc.wTag, hTag)
			assert.Equal(t, tc.wCnt, hCnt)
			assert.Equal(t, tc.wHash, hHash)
		})
	}
}

func Test_describeNoTag(t *testing.T) {
	t.Run("counts every commit from the synthetic tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := describeNoTag(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := description{tag: "v0.0.0", count: 2, hash: cm.Hash}
		assert.Equal(t, want, have)
	})

	t.Run("dirty work dir", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll()
		prj.CreateFileWith("edit", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := describeNoTag(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		want := description{tag: "v0.0.0", count: 1, hash: cm.Hash, dirty: true}
		assert.Equal(t, want, have)
	})

	t.Run("error - empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := describeNoTag(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := describeNoTag(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - bare repository", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		bare := t.TempDir()
		prj.Exe("git", "clone", "--bare", prj.Root(), bare)
		prj.Close()

		// --- When ---
		have, err := describeNoTag(ctx, bare)

		// --- Then ---
		assert.ErrorContain(t, "must be run in a work tree", err)
		assert.Empty(t, have)
	})
}

func Test_Derive(t *testing.T) {
	t.Run("on the tag and clean is the release", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll("v0.4.0")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.0", have.Rev)
		assert.Equal(t, "v0.4.0", have.Tag)
		assert.Equal(t, cm.Hash, have.Hash)
		assert.Equal(t, 0, have.Count)
		assert.False(t, have.Dirty)
		assert.True(t, have.Release)
	})

	t.Run("on the tag and dirty is a patch pre-release", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.1-dev.0.dirty+g"+cm.Hash, have.Rev)
		assert.True(t, have.Dirty)
		assert.False(t, have.Release)
	})

	t.Run("an untracked file is not a release", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("stray", "untracked.txt")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.1-dev.0.dirty+g"+cm.Hash, have.Rev)
		assert.False(t, have.Release)
	})

	t.Run("commits past the tag count", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.CreateFileWith("file0 3", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), BumpPatch)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.1-dev.2+g"+cm.Hash, have.Rev)
		assert.Equal(t, 2, have.Count)
	})

	t.Run("a minor bump zeroes the patch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.3")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), BumpMinor)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.5.0-dev.1+g"+cm.Hash, have.Rev)
	})

	t.Run("a major bump on 0.x advances the minor", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), BumpMajor)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.5.0-dev.1+g"+cm.Hash, have.Rev)
	})

	t.Run("a major bump on 1.x advances the major", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), BumpMajor)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v2.0.0-dev.1+g"+cm.Hash, have.Rev)
	})

	t.Run("a patch bump of a pre-release tag extends it", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.0.0-rc.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), BumpPatch)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0-rc.1.dev.1+g"+cm.Hash, have.Rev)
		rev, tag := semver.MustParse(have.Rev), semver.MustParse(have.Tag)
		assert.True(t, rev.GreaterThan(tag))
	})

	t.Run("a dirty pre-release tag extends it", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll("v1.0.0-rc.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0-rc.1.dev.0.dirty+g"+cm.Hash, have.Rev)
	})

	t.Run("a minor bump of a pre-release tag advances it", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.0.0-rc.1")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), BumpMinor)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.1.0-dev.1+g"+cm.Hash, have.Rev)
	})

	t.Run("no version tag builds up from the base", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.0.1-dev.2+g"+cm.Hash, have.Rev)
		assert.Equal(t, noTagBase, have.Tag)
		assert.False(t, have.Release)
	})

	t.Run("a tag that is not a version is skipped", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		cm := prj.GitCommit("nightly")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.4.1-dev.1+g"+cm.Hash, have.Rev)
		assert.Equal(t, "v0.4.0", have.Tag)
	})

	t.Run("tag ending in the dirty marker is a release", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll("v1.0.0-dirty")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0-dirty", have.Rev)
		assert.Equal(t, "v1.0.0-dirty", have.Tag)
		assert.Equal(t, cm.Hash, have.Hash)
		assert.Equal(t, 0, have.Count)
		assert.False(t, have.Dirty)
		assert.True(t, have.Release)
	})

	t.Run("tag shaped like describe output is a release", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.2.0-3-gabcdef1")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v1.2.0-3-gabcdef1", have.Rev)
		assert.Equal(t, "v1.2.0-3-gabcdef1", have.Tag)
		assert.Equal(t, 0, have.Count)
		assert.True(t, have.Release)
	})

	t.Run("error - unknown bump on a release", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "sideways")

		// --- Then ---
		assert.ErrorIs(t, ErrBadBump, err)
		assert.Equal(t, Version{}, have)
	})

	t.Run("error - unknown bump outside a repository", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "sideways")

		// --- Then ---
		assert.ErrorIs(t, ErrBadBump, err)
		assert.Equal(t, Version{}, have)
	})

	t.Run("error - unknown bump", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "sideways")

		// --- Then ---
		assert.ErrorIs(t, ErrBadBump, err)
		assert.Equal(t, Version{}, have)
	})

	t.Run("error - not a git repository", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Equal(t, Version{}, have)
	})

	t.Run("error - bare repository", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.4.0")
		bare := t.TempDir()
		prj.Exe("git", "clone", "--bare", prj.Root(), bare)
		prj.Close()

		// --- When ---
		have, err := Derive(ctx, bare, "")

		// --- Then ---
		assert.ErrorContain(t, "must be run in a work tree", err)
		assert.Equal(t, Version{}, have)
	})
}

func Test_bumped(t *testing.T) {
	t.Run("error - unknown bump", func(t *testing.T) {
		// --- Given ---
		base := semver.MustParse("v1.2.3")

		// --- When ---
		have, err := bumped(base, "sideways")

		// --- Then ---
		assert.ErrorIs(t, ErrBadBump, err)
		assert.Equal(t, semver.Version{}, have)
	})
}

func Test_bumped_tabular(t *testing.T) {
	tt := []struct {
		testN string

		base string
		bump string
		want string
	}{
		{"major", "v1.2.3", BumpMajor, "v2.0.0"},
		{"minor", "v1.2.3", BumpMinor, "v1.3.0"},
		{"patch", "v1.2.3", BumpPatch, "v1.2.4"},
		{"empty is patch", "v1.2.3", "", "v1.2.4"},
		{"major below 1 is minor", "v0.4.3", BumpMajor, "v0.5.0"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			base := semver.MustParse(tc.base)

			// --- When ---
			have, err := bumped(base, tc.bump)

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have.Original())
		})
	}
}

func Test_checkBump(t *testing.T) {
	t.Run("error - unknown bump", func(t *testing.T) {
		// --- When ---
		err := checkBump("sideways")

		// --- Then ---
		assert.ErrorIs(t, ErrBadBump, err)
		assert.ErrorContain(t, "sideways", err)
	})
}

func Test_checkBump_tabular(t *testing.T) {
	tt := []struct {
		testN string

		bump string
	}{
		{"empty", ""},
		{"patch", BumpPatch},
		{"minor", BumpMinor},
		{"major", BumpMajor},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			err := checkBump(tc.bump)

			// --- Then ---
			assert.NoError(t, err)
		})
	}
}

func Test_CountCommits(t *testing.T) {
	t.Run("counts every commit reachable from HEAD", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := CountCommits(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 2, have)
	})

	t.Run("counts a range", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("")
		prj.Close()

		// --- When ---
		have, err := CountCommits(ctx, prj.Root(), "v0.1.0..HEAD")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 1, have)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := CountCommits(ctx, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Equal(t, 0, have)
	})

	t.Run("error - revision looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		have, err := CountCommits(ctx, prj.Root(), "--all")

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.Equal(t, 0, have)
	})
}

func Test_Messages(t *testing.T) {
	t.Run("keeps the body as well as the subject", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		msg := "feat: add a thing\n\nBREAKING CHANGE: it changed"

		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", msg)
		prj.Close()

		// --- When ---
		have, err := Messages(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 2, have)
		assert.Equal(t, msg, have[1])
	})

	t.Run("oldest first within a range", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "fix: second")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.GitCommit("", "feat: third")
		prj.Close()

		// --- When ---
		have, err := Messages(ctx, prj.Root(), "v0.1.0..HEAD")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"fix: second", "feat: third"}, have)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := Messages(ctx, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Nil(t, have)
	})

	t.Run("error - range looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()
		out := filepath.Join(t.TempDir(), "out.txt")
		rng := "--output=" + out

		// --- When ---
		have, err := Messages(ctx, prj.Root(), rng)

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.Nil(t, have)

		assert.NoFileExist(t, out)
	})
}

func Test_ChangeLog(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "0.0.0")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - unknown base revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "0.0.0")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTag, err)
		assert.Empty(t, have)
	})

	t.Run("base revision equal to HEAD", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have)
	})

	t.Run("one commit ahead of base revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"test commit 2"}, have)
	})

	t.Run("multiple commits ahead of base revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 3")
		prj.CreateFileWith("file0 4", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 4")
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"test commit 2",
			"test commit 3",
			"test commit 4",
		}
		assert.Equal(t, want, have)
	})

	t.Run("changelog since repo start", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 3")
		prj.CreateFileWith("file0 4", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 4")
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"Initial commit.",
			"test commit 2",
			"test commit 3",
			"test commit 4",
		}
		assert.Equal(t, want, have)
	})

	t.Run("multi paragraph commit messages", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Exe("git", "tag", "v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		msg := "CM2 Paragraph 1 Sentence 1. Paragraph 1 Sentence 2.\n" +
			"Paragraph 2 Sentence 1. Paragraph 2 Sentence 2.\n"
		prj.Exe("git", "commit", "-am", msg)
		prj.CreateFileWith("file0 3", "file0.txt")
		msg = "CM3 Paragraph 1 Sentence 1. Paragraph 1 Sentence 2.\n" +
			"Paragraph 2 Sentence 1. Paragraph 2 Sentence 2.\n"
		prj.Exe("git", "commit", "-am", msg)
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		want := []string{
			"CM2 Paragraph 1 Sentence 1. Paragraph 1 Sentence 2.",
			"CM3 Paragraph 1 Sentence 1. Paragraph 1 Sentence 2.",
		}
		assert.Equal(t, want, have)
	})

	t.Run("base revision on a diverged branch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v1.0.0")
		prj.Exe("git", "checkout", "-b", "maint")
		prj.CreateFileWith("hotfix", "file1.txt")
		prj.GitCommit("v1.0.1", "fix: hotfix")
		prj.Exe("git", "checkout", "-")
		prj.CreateFileWith("feature", "file2.txt")
		prj.GitCommit("", "feat: feature")
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v1.0.1")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"feat: feature"}, have)
	})

	t.Run("body line starting with the entry marker", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("", "fix: subject\n\n>> quoted reply\n  >>indented\n")
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"fix: subject"}, have)
	})

	t.Run("commit message line longer than scanner limit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		big := strings.Repeat("a", bufio.MaxScanTokenSize+1)
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", big)
		prj.Close()

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{big}, have)
	})

	t.Run("error - revision looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()
		out := filepath.Join(t.TempDir(), "out.txt")
		rev := "--output=" + out

		// --- When ---
		have, err := ChangeLog(ctx, prj.Root(), rev)

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.Nil(t, have)

		assert.NoFileExist(t, out+"...HEAD")
	})
}

func Test_Init(t *testing.T) {
	t.Run("init", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Init(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		assert.DirExist(t, filepath.Join(prj.Root(), ".git"))
		assert.Contain(t, "No commits yet", prj.ExeStdout("git", "status"))
	})

	t.Run("error - not existing directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()
		dir := filepath.Join(prj.Root(), "not_existing")

		// --- When ---
		err := Init(ctx, dir)

		// --- Then ---
		assert.ErrorContain(t, "no such file or directory", err)
	})
}

func Test_AddRemote(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := AddRemote(ctx, prj.Root(), prjkit.GitOrigin)

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		err := AddRemote(ctx, prj.Root(), prjkit.GitOrigin)

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := AddRemote(ctx, prj.Root(), prjkit.GitOrigin)

		// --- Then ---
		assert.NoError(t, err)

		want := "" +
			"origin\tgit@example.com:comp/project.git (fetch)\n" +
			"origin\tgit@example.com:comp/project.git (push)\n"
		remotes := prj.ExeStdout("git", "remote", "-v")
		assert.Equal(t, want, remotes)
	})

	t.Run("error - remote looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		err := AddRemote(ctx, prj.Root(), "--mirror=push")

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
	})
}

func Test_IsClean(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.False(t, have)
	})

	t.Run("initialized", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
	})

	t.Run("not added file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, have)
	})

	t.Run("added not committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "add", "-A")
		prj.Close()

		// --- When ---
		have, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, have)
	})

	t.Run("added and committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
	})
}

func Test_WorkTreeStatus(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		have, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("initialized", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		have, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "clean", have)
	})

	t.Run("not added file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()

		// --- When ---
		have, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dirty", have)
	})

	t.Run("added not committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "add", "-A")
		prj.Close()

		// --- When ---
		have, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dirty", have)
	})

	t.Run("added and committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "clean", have)
	})
}

func Test_Add(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Add(ctx, prj.Root(), "file0.txt")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.CreateFileWith("file1 1", "file1.txt")
		prj.Close()

		// --- When ---
		err := Add(ctx, prj.Root(), "file0.txt", "file1.txt")

		// --- Then ---
		assert.NoError(t, err)

		out := prj.ExeStdout("git", "status", "-s")
		assert.Equal(t, "A  file0.txt\nA  file1.txt\n", out)
	})

	t.Run("path looking like an option is a path", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.CreateFileWith("dash 1", "-A")
		prj.Close()

		// --- When ---
		err := Add(ctx, prj.Root(), "-A")

		// --- Then ---
		assert.NoError(t, err)

		out := prj.ExeStdout("git", "status", "-s")
		assert.Equal(t, "A  -A\n?? file0.txt\n", out)
	})
}

func Test_AddAll(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := AddAll(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.CreateFileWith("file1 1", "file1.txt")
		prj.Close()

		// --- When ---
		err := AddAll(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)

		out := prj.ExeStdout("git", "status", "-s")
		assert.Equal(t, "A  file0.txt\nA  file1.txt\n", out)
	})
}

func Test_Commit(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Commit(ctx, prj.Root(), "message")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("error - empty commit message", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "config", "user.email", "test@example.com")
		prj.Exe("git", "config", "user.name", "Test User")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "add", "file0.txt")
		prj.Close()

		// --- When ---
		err := Commit(ctx, prj.Root(), "")

		// --- Then ---
		wMsg := "Aborting commit due to empty commit message"
		assert.ErrorContain(t, wMsg, err)
		assert.ErrorIs(t, ErrGit, err)
	})

	t.Run("commit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "config", "user.email", "test@example.com")
		prj.Exe("git", "config", "user.name", "Test User")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.CreateFileWith("file1 1", "file1.txt")
		prj.Exe("git", "add", "file0.txt", "file1.txt")
		prj.Close()

		// --- When ---
		err := Commit(ctx, prj.Root(), "message")

		// --- Then ---
		assert.NoError(t, err)

		out := prj.ExeStdout("git", "log", "--oneline", "--name-status")
		assert.Contain(t, "message\n", out)
		assert.Contain(t, "A\tfile0.txt\n", out)
		assert.Contain(t, "A\tfile1.txt\n", out)
	})
}

func Test_Tag(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Tag(ctx, prj.Root(), "v0.0.0", "message")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("empty tag message", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := Tag(ctx, prj.Root(), "v0.0.0", "")

		// --- Then ---
		assert.NoError(t, err)

		out := prj.ExeStdout("git", "tag", "-n")
		assert.Equal(t, "v0.0.0          \n", out)
	})

	t.Run("tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		err := Tag(ctx, prj.Root(), "v0.0.0", "tag message")

		// --- Then ---
		assert.NoError(t, err)

		out := prj.ExeStdout("git", "tag", "-n")
		assert.Equal(t, "v0.0.0          tag message\n", out)
	})

	t.Run("error - tag looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		err := Tag(ctx, prj.Root(), "--force", "message")

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
	})
}

func Test_Push(t *testing.T) {
	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Push(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		bare := Bare(t)
		branch := randkit.Str()
		tag := "tag-" + branch

		ctx := t.Context()
		prj0 := prjkit.New(t, t.TempDir())
		prj0.Exe("git", "clone", bare, ".")
		prj0.Exe("git", "checkout", "-b", branch)
		prj0.CreateFileWith(branch, "file0.txt")
		prj0.GitCommit("", "test commit 1")
		prj0.Exe("git", "tag", "-a", "-m", "tag-msg-"+tag, tag)
		prj0.Close()

		// --- When ---
		err := Push(ctx, prj0.Root())

		// --- Then ---
		assert.NoError(t, err)

		prj1 := prjkit.New(t, t.TempDir())
		prj1.Exe("git", "clone", "--branch", branch, bare, ".")
		prj1.Close()

		assert.Equal(t, branch, prj1.ReadFileStr("file0.txt"))
		want := fmt.Sprintf("tag-%s  tag-msg-tag-%s\n", branch, branch)
		assert.Equal(t, want, prj1.ExeStdout("git", "tag", "-n99"))
	})

	t.Run("error - detached head", func(t *testing.T) {
		// --- Given ---
		bare := Bare(t)

		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "clone", bare, ".")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitCommit("", "test commit 1")
		prj.Exe("git", "checkout", "--detach")
		prj.Close()

		// --- When ---
		err := Push(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrDetached, err)
	})

	t.Run("does not push lightweight tags", func(t *testing.T) {
		// --- Given ---
		bare := Bare(t)
		branch := randkit.Str()
		tag := "tag-" + branch

		ctx := t.Context()
		prj0 := prjkit.New(t, t.TempDir())
		prj0.Exe("git", "clone", bare, ".")
		prj0.Exe("git", "checkout", "-b", branch)
		prj0.CreateFileWith(branch, "file0.txt")
		prj0.GitCommit("", "test commit 1")
		prj0.Exe("git", "tag", tag)
		prj0.Close()

		// --- When ---
		err := Push(ctx, prj0.Root())

		// --- Then ---
		assert.NoError(t, err)

		prj1 := prjkit.New(t, t.TempDir())
		prj1.Exe("git", "clone", "--branch", branch, bare, ".")
		prj1.Close()

		assert.Equal(t, branch, prj1.ReadFileStr("file0.txt"))
		assert.Empty(t, prj1.ExeStdout("git", "tag", "-n99"))
	})
}

func Test_GetFile(t *testing.T) {
	bare := Bare(t)
	// Generate random branch name check it out, add file, commit and push.
	branch := randkit.Str()

	prj := prjkit.New(t, t.TempDir())
	prj.Exe("git", "clone", bare, ".")
	prj.Exe("git", "checkout", "-b", branch)
	prj.CreateFileWith(branch, "file0.txt")
	prj.CreateFileWith("a", "dir", "a.txt")
	prj.CreateFileWith("b", "dir", "b.txt")
	prj.GitCommit("", "test commit 1")
	prj.Exe("git", "push", "origin", "HEAD")
	prj.Close()

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "file0.txt", dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, branch, oskit.ReadFileStr(t, dst))
	})

	t.Run("error - remote never answers", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithTimeout(t.Context(), 200*time.Millisecond)
		t.Cleanup(cxl)

		// A local listener that accepts and stays silent stands in for an
		// unreachable remote without depending on the network.
		lsn := must.Value(net.Listen("tcp", "127.0.0.1:0"))
		t.Cleanup(func() { _ = lsn.Close() })
		go func() {
			var cons []net.Conn
			for {
				con, err := lsn.Accept()
				if err != nil {
					for _, con := range cons {
						_ = con.Close()
					}
					return
				}
				cons = append(cons, con)
			}
		}()
		badRepo := "git://" + lsn.Addr().String() + "/repo.git"
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, badRepo, branch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("error - invalid branch", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		badBranch := randkit.Str()
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, badBranch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("error - invalid repo file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "bad.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkFile, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("error - invalid destination", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "not_existing", "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "file0.txt", dst)

		// --- Then ---
		var e *fs.PathError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, dst, e.Path)
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("error - destination is a directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := t.TempDir() // Existing directory makes os.Create fail.

		// --- When ---
		err := GetFile(ctx, bare, branch, "file0.txt", dst)

		// --- Then ---
		var e *fs.PathError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, dst, e.Path)
	})

	t.Run("error - very short deadline", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		ctxTO, cxlTO := context.WithTimeout(ctx, time.Millisecond)
		defer cxlTO()

		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctxTO, bare, branch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("error - context canceled", func(t *testing.T) {
		// --- Given ---
		ctx, cxl := context.WithCancel(t.Context())
		t.Cleanup(cxl)

		// A git that never finishes keeps the archive step running until
		// the context is canceled.
		binDir := t.TempDir()
		script := "#!/bin/sh\nexec sleep 30\n"
		gitBin := oskit.Create(t, script, binDir, "git")
		must.Nil(os.Chmod(gitBin, 0o755))
		sep := string(os.PathListSeparator)
		t.Setenv("PATH", binDir+sep+os.Getenv("PATH"))
		time.AfterFunc(100*time.Millisecond, cxl)

		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("empty repo means the working directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		t.Chdir(prj.Root())
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, "", branch, "file0.txt", dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, branch, oskit.ReadFileStr(t, dst))
	})

	t.Run("relative repo is from the working directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		t.Chdir(filepath.Dir(bare))
		repo := filepath.Base(bare)
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, repo, branch, "file0.txt", dst)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, branch, oskit.ReadFileStr(t, dst))
	})

	t.Run("error - source is a directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "dir", dst)

		// --- Then ---
		assert.ErrorIs(t, ErrNotFile, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("error - writing destination", func(t *testing.T) {
		// --- Given ---
		if _, err := os.Stat("/dev/full"); err != nil {
			t.Skip("/dev/full not available")
		}

		ctx := t.Context()

		// Every write to /dev/full fails with ENOSPC.
		dst := "/dev/full"

		// --- When ---
		err := GetFile(ctx, bare, branch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, syscall.ENOSPC, err)
	})

	t.Run("error - branch looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		marker := filepath.Join(t.TempDir(), "marker")
		badBranch := "--exec=touch " + marker
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, badBranch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.NoFileExist(t, dst)

		assert.NoFileExist(t, marker)
	})

	t.Run("error - source looks like an option", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "--list", dst)

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.NoFileExist(t, dst)
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

func Test_extractFile(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		// --- Given ---
		r := tarOf(t, tarDir("dir/"), tarFile("dir/a.txt", "a"))

		// --- When ---
		have, err := extractFile(r, "./dir/a.txt")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "a", string(have))
	})

	t.Run("error - directory with files", func(t *testing.T) {
		// --- Given ---
		r := tarOf(t, tarDir("dir/"), tarFile("dir/a.txt", "a"))

		// --- When ---
		have, err := extractFile(r, "dir")

		// --- Then ---
		assert.ErrorIs(t, ErrNotFile, err)
		assert.Nil(t, have)
	})

	t.Run("error - symlink", func(t *testing.T) {
		// --- Given ---
		lnk := &tar.Header{
			Name:     "a.txt",
			Typeflag: tar.TypeSymlink,
			Linkname: "b",
		}
		r := tarOf(t, lnk)

		// --- When ---
		have, err := extractFile(r, "a.txt")

		// --- Then ---
		assert.ErrorIs(t, ErrNotFile, err)
		assert.Nil(t, have)
	})

	t.Run("error - no such entry", func(t *testing.T) {
		// --- Given ---
		r := tarOf(t, tarDir("dir/"))

		// --- When ---
		have, err := extractFile(r, "a.txt")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkFile, err)
		assert.Nil(t, have)
	})

	t.Run("error - not a tar stream", func(t *testing.T) {
		// --- Given ---
		r := strings.NewReader(strings.Repeat("x", 1024))

		// --- When ---
		have, err := extractFile(r, "a.txt")

		// --- Then ---
		assert.ErrorContain(t, "read archive: ", err)
		assert.Nil(t, have)
	})
}

func Test_IsHash_tabular(t *testing.T) {
	tt := []struct {
		in   string
		want bool
	}{
		{"0123456789abcdef", true},
		{"xx", false},
	}

	for _, tc := range tt {
		t.Run(tc.in, func(t *testing.T) {
			// --- When ---
			have := IsHash(tc.in)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_noOption(t *testing.T) {
	t.Run("no option", func(t *testing.T) {
		// --- When ---
		err := noOption("", "v1.0.0..HEAD", "dir/file-1.txt")

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - option among args", func(t *testing.T) {
		// --- When ---
		err := noOption("v1.0.0", "--output=x", "-y")

		// --- Then ---
		assert.ErrorIs(t, ErrBadArg, err)
		assert.ErrorContain(t, `"--output=x"`, err)
	})
}

func Test_gitCommand(t *testing.T) {
	t.Run("pins the locale", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		t.Setenv("LC_ALL", "de_DE.UTF-8")

		// --- When ---
		have := gitCommand(ctx, "status", "--porcelain")

		// --- Then ---
		assert.Equal(t, []string{"git", "status", "--porcelain"}, have.Args)
		assert.Equal(t, "LC_ALL=C", have.Env[len(have.Env)-1])
	})
}

func Test_runGitCmd(t *testing.T) {
	t.Run("standard output trimmed", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()
		repo := prj.Root()

		// --- When ---
		have, err := runGitCmd(ctx, repo, "rev-parse", "--git-dir")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, ".git", have)
	})

	t.Run("error - not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()
		repo := prj.Root()

		// --- When ---
		have, err := runGitCmd(ctx, repo, "rev-parse", "--git-dir")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - message after other stderr lines", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()
		repo := prj.Root()

		// Trace output lands on stderr in front of the fatal line.
		t.Setenv("GIT_TRACE", "1")

		// --- When ---
		have, err := runGitCmd(ctx, repo, "rev-parse", "--git-dir")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})

	t.Run("error - not git repo in a translated locale", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()
		repo := prj.Root()

		// A German catalog for both forms of git's "not a git repository"
		// message stands in for an installed translation.
		msgs := map[string]string{
			"" +
				"not a git repository (or any parent up to mount point %s)\n" +
				"Stopping at filesystem boundary " +
				"(GIT_DISCOVERY_ACROSS_FILESYSTEM not set).": "" +
				"Kein Git-Repository (bis %s)",
			"not a git repository (or any of the parent directories): %s": "" +
				"Kein Git-Repository: %s",
		}
		dir := t.TempDir()
		mod := oskit.MkdirAll(t, dir, "de", "LC_MESSAGES")
		oskit.Create(t, string(moFile(msgs)), mod, "git.mo")
		t.Setenv("GIT_TEXTDOMAINDIR", dir)
		t.Setenv("LC_ALL", "en_US.UTF-8")
		t.Setenv("LANGUAGE", "de")

		cmd := exec.Command("git", "rev-parse", "--git-dir")
		cmd.Dir = repo
		out, _ := cmd.CombinedOutput()
		if !strings.Contains(string(out), "Kein Git-Repository") {
			t.Skipf("git does not load the test translation: %s", out)
		}

		// --- When ---
		have, err := runGitCmd(ctx, repo, "rev-parse", "--git-dir")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, have)
	})
}

func Test_gitMessage_tabular(t *testing.T) {
	tt := []struct {
		testN string

		stderr string
		want   string
	}{
		{"empty", "", ""},
		{"single line", "fatal: not a repo\n", "fatal: not a repo"},
		{"no prefix", "Aborting commit\nhint: x\n", "Aborting commit"},
		{"fatal after trace", "trace: git\nfatal: bad\n", "fatal: bad"},
		{"error before fatal", "error: first\nfatal: second\n", "error: first"},
		{"indented", "warning: w\n  fatal: bad  \n", "fatal: bad"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := gitMessage(tc.stderr)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_gitErrorOr(t *testing.T) {
	t.Run("error - unmapped message", func(t *testing.T) {
		// --- When ---
		err := gitErrorOr("message not covered by cases", nil)

		// --- Then ---
		assert.ErrorIs(t, ErrGit, err)
		assert.ErrorContain(t, "message not covered by cases", err)
	})

	t.Run("error - unmapped message keeps the cause", func(t *testing.T) {
		// --- When ---
		err := gitErrorOr("message not covered by cases", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, ErrGit, err)
		assert.ErrorIs(t, ErrTest, err)
		assert.ErrorContain(t, "message not covered by cases", err)
	})

	t.Run("error - empty message", func(t *testing.T) {
		// --- When ---
		err := gitErrorOr("", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, ErrTest, err)
	})

	t.Run("error - empty message nil err", func(t *testing.T) {
		// --- When ---
		err := gitErrorOr("", nil)

		// --- Then ---
		wMsg := "empty git error message and nil error parameter"
		assert.ErrorContain(t, wMsg, err)
	})
}

func Test_gitErrorOr_tabular(t *testing.T) {
	tt := []struct {
		name string

		msg string
		or  error
		err error
	}{
		{
			"invalid object HEAD empty repo",
			"Not a valid object name HEAD",
			ErrTest,
			ErrEmptyRepo,
		},
		{
			"invalid object name unknown rev",
			"Not a valid object name ",
			ErrTest,
			ErrUnkRev,
		},
		{
			"not a git repository",
			"not a git repository",
			ErrTest,
			ErrNotRepo,
		},
		{
			"bad revision HEAD empty repo",
			"bad revision 'HEAD'",
			ErrTest,
			ErrEmptyRepo,
		},
		{
			"no commits yet empty repo",
			"does not have any commits yet",
			ErrTest,
			ErrEmptyRepo,
		},
		{
			"cannot describe no tags",
			"cannot describe anything",
			ErrTest,
			ErrNoTags,
		},
		{
			"no tags can describe",
			"No tags can describe",
			ErrTest,
			ErrNoTags,
		},
		{
			"path not in working tree empty repo",
			"or path not in the working tree",
			ErrTest,
			ErrEmptyRepo,
		},
		{
			"no push destination no remote",
			"No configured push destination",
			ErrTest,
			ErrNoRemote,
		},
		{
			"no such remote origin",
			"No such remote 'origin'",
			ErrTest,
			ErrNoRemote,
		},
		{
			"bare exit status 1",
			"exit status 1",
			ErrTest,
			ErrGit,
		},
		{
			"missing remote section no remote",
			"key does not contain a section: remote",
			ErrTest,
			ErrNoRemote,
		},
		{
			"ambiguous HEAD empty repo",
			"ambiguous argument 'HEAD'",
			ErrTest,
			ErrEmptyRepo,
		},
		{
			"ambiguous arg unknown revision",
			"ambiguous argument '0000': unknown revision or path",
			ErrTest,
			ErrUnkRev,
		},
		{
			"no such ref unknown rev",
			"remote: fatal: no such ref: ddiMIeGaMj",
			ErrTest,
			ErrUnkRev,
		},
		{
			"did not match any files",
			"did not match any files",
			ErrTest,
			ErrUnkFile,
		},
		{
			"outside git repository",
			"can only be used inside a git repository",
			ErrTest,
			ErrNotRepo,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// --- When ---
			err := gitErrorOr(tc.msg, tc.or)

			// --- Then ---
			assert.ErrorIs(t, tc.err, err)
		})
	}
}
