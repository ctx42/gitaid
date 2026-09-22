// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/iokit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"
	"github.com/ctx42/testkit/pkg/randkit"
)

func Test_IsRepo(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
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
	t.Run("not git repo", func(t *testing.T) {
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

func Test_ProjectName(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		name, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, name)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		name, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", name)
	})

	t.Run("repo url", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", prjkit.GitOrigin)
		prj.Close()

		// --- When ---
		name, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", name)
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
		name, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "skw-proj", name)
	})

	t.Run("repository without origin", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		name, err := ProjectName(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", name)
	})

	t.Run("empty repo resolves current directory", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, oskit.MkdirAll(t, t.TempDir(), "project"))
		prj.Exe("git", "init")
		prj.Close()
		t.Chdir(prj.Root())

		// --- When ---
		name, err := ProjectName(ctx, "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "project", name)
	})
}

func Test_ProjectOrigin(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		src, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, src)
	})

	t.Run("repository without origin", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		src, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, src)
	})

	t.Run("success", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Exe("git", "remote", "add", "origin", prjkit.GitOrigin)
		prj.Close()

		// --- When ---
		src, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, prjkit.GitOrigin, src)
	})

	t.Run("config line exceeds scanner limit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		big := strings.Repeat("a", bufio.MaxScanTokenSize+1)
		prj.Exe("git", "config", "--local", "gitaid.big", big)
		prj.Close()

		// --- When ---
		src, err := ProjectOrigin(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, bufio.ErrTooLong, err)
		assert.Empty(t, src)
	})
}

func Test_FirstHash(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		hash, err := FirstHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, hash)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		hash, err := FirstHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, hash)
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
		hash, err := FirstHash(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, cm.Hash, hash)
	})
}

func Test_LatestHash(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		hash, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, hash)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		hash, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, hash)
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
		hash, err := LatestHash(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, cm.Hash, hash)
	})
}

func Test_RevDate(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		tim, err := RevDate(ctx, prj.Root(), "0000")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Zero(t, tim)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		tim, err := RevDate(ctx, prj.Root(), "0000")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.Zero(t, tim)
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

		// --- When ---
		tim1, err1 := RevDate(ctx, prj.Root(), cm1.Hash)
		tim2, err2 := RevDate(ctx, prj.Root(), cm2.Hash)

		// --- Then ---
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.True(t, tim1.Before(tim2))
		assert.Within(t, time.Now(), "3s", tim1)
		assert.Within(t, time.Now(), "3s", tim2)
	})

	t.Run("not existing revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		tim, err := RevDate(ctx, prj.Root(), "not_existing")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkRev, err)
		assert.Zero(t, tim)
	})

	t.Run("revision output is not a timestamp", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// A blob revision makes git print the file content instead of a
		// commit timestamp, so strconv.ParseInt fails.

		// --- When ---
		tim, err := RevDate(ctx, prj.Root(), "HEAD:file0.txt")

		// --- Then ---
		var e *strconv.NumError
		assert.ErrorAs(t, &e, err)
		assert.Zero(t, tim)
	})
}

func Test_ClosestTag(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, tag)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, tag)
	})

	t.Run("one commit no tags", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, tag)
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
		tag, err := ClosestTag(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
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
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
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
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
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
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
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
		tag, err := ClosestTag(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
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
		tag, err := ClosestTag(ctx, prj.Root(), "v0.2.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
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

func Test_Describe(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, tag)
	})

	t.Run("empty git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrEmptyRepo, err)
		assert.Empty(t, tag)
	})

	t.Run("one commit no tags", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		cm := prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assertHash(t, tag)
		assert.Equal(t, cm.Hash, tag)
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
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0", tag)
	})

	t.Run("one commit after tag", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 2")
		prj.Exe("git", "tag", "TAG")
		prj.CreateFileWith("file0 3", "file0.txt")
		prj.Exe("git", "commit", "-am", "test commit 3")
		prj.Close()

		// --- When ---
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		exp := fmt.Sprintf("TAG-1-g%s", prj.GitHash())
		assert.Equal(t, exp, tag)
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
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "v0.1.0-dev", tag)
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
		tag, err := Describe(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("v0.1.0-1-g%s-dev", cm.Hash), tag)
	})

	t.Run("match skips a tag that is not a version", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.GitCommit("nightly")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root(), WithMatch("v[0-9]*"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "v0.1.0-1-g", have)
	})

	t.Run("match skipping every tag falls back to hash", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0", "file0.txt")
		cm := prj.GitInitAddAll("v0.1.0")
		prj.Close()

		// --- When ---
		have, err := Describe(ctx, prj.Root(), WithMatch("rel-*"))

		// --- Then ---
		assert.NoError(t, err)
		assertHash(t, have)
		assert.Equal(t, cm.Hash, have)
	})
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
		assert.Error(t, err)
		assert.Equal(t, 0, have)
	})
}

func Test_Messages(t *testing.T) {
	t.Run("keeps the body as well as the subject", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		msg := "feat: add a thing\n\nBREAKING CHANGE: it changed"
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
		assert.Error(t, err)
		assert.Nil(t, have)
	})
}

func Test_ChangeLog(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		cl, err := ChangeLog(ctx, prj.Root(), "0.0.0")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, cl)
	})

	t.Run("unknown base revision", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		cl, err := ChangeLog(ctx, prj.Root(), "0.0.0")

		// --- Then ---
		assert.ErrorIs(t, ErrUnkTag, err)
		assert.Empty(t, cl)
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
		cl, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, cl)
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
		cl, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			"test commit 2",
		}
		assert.Equal(t, exp, cl)
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
		cl, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			"test commit 2",
			"test commit 3",
			"test commit 4",
		}
		assert.Equal(t, exp, cl)
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
		cl, err := ChangeLog(ctx, prj.Root(), "")

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			"Initial commit.",
			"test commit 2",
			"test commit 3",
			"test commit 4",
		}
		assert.Equal(t, exp, cl)
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
		cl, err := ChangeLog(ctx, prj.Root(), "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)

		exp := []string{
			"CM2 Paragraph 1 Sentence 1. Paragraph 1 Sentence 2.",
			"CM3 Paragraph 1 Sentence 1. Paragraph 1 Sentence 2.",
		}
		assert.Equal(t, exp, cl)
	})

	t.Run("commit message exceeds scanner limit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		big := strings.Repeat("a", bufio.MaxScanTokenSize+1)
		prj.CreateFileWith("file0 2", "file0.txt")
		prj.Exe("git", "commit", "-am", big)
		prj.Close()

		// --- When ---
		cl, err := ChangeLog(ctx, prj.Root(), "")

		// --- Then ---
		assert.ErrorIs(t, bufio.ErrTooLong, err)
		assert.Empty(t, cl)
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

		have := prj.ExeStdout("git", "status")
		assert.Contain(t, "On branch master", have)
		assert.Contain(t, "No commits yet", have)
	})

	t.Run("error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Init(ctx, filepath.Join(prj.Root(), "not_existing"))

		// --- Then ---
		assert.ErrorContain(t, "no such file or directory", err)
	})
}

func Test_AddRemote(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
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
		have := prj.ExeStdout("git", "remote", "-v")
		assert.Equal(t, want, have)
	})
}

func Test_IsClean(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		clean, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.False(t, clean)
	})

	t.Run("initialized", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		clean, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, clean)
	})

	t.Run("not added file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()

		// --- When ---
		clean, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, clean)
	})

	t.Run("added not committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "add", "-A")
		prj.Close()

		assert.NoError(t, Init(ctx, prj.Root()))

		// --- When ---
		clean, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, clean)
	})

	t.Run("added and committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		assert.NoError(t, Init(ctx, prj.Root()))

		// --- When ---
		clean, err := IsClean(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, clean)
	})
}

func Test_WorkTreeStatus(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		state, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
		assert.Empty(t, state)
	})

	t.Run("initialized", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.Close()

		// --- When ---
		clean, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "clean", clean)
	})

	t.Run("not added file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Close()

		// --- When ---
		clean, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dirty", clean)
	})

	t.Run("added not committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "add", "-A")
		prj.Close()

		assert.NoError(t, Init(ctx, prj.Root()))

		// --- When ---
		clean, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "dirty", clean)
	})

	t.Run("added and committed file", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.GitInitAddAll()
		prj.Close()

		assert.NoError(t, Init(ctx, prj.Root()))

		// --- When ---
		clean, err := WorkTreeStatus(ctx, prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "clean", clean)
	})
}

func Test_Add(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
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
}

func Test_AddAll(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
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
	t.Run("not git repo", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Close()

		// --- When ---
		err := Commit(ctx, prj.Root(), "message")

		// --- Then ---
		assert.ErrorIs(t, ErrNotRepo, err)
	})

	t.Run("empty commit message", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
		prj.CreateFileWith("file0 1", "file0.txt")
		prj.Exe("git", "add", "file0.txt")
		prj.Close()

		// --- When ---
		err := Commit(ctx, prj.Root(), "")

		// --- Then ---
		wMsg := "Aborting commit due to empty commit message"
		assert.ErrorContain(t, wMsg, err)
	})

	t.Run("commit", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		prj := prjkit.New(t, t.TempDir())
		prj.Exe("git", "init")
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
	t.Run("not git repo", func(t *testing.T) {
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
}

func Test_Push(t *testing.T) {
	t.Run("not git repo", func(t *testing.T) {
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
		prj0.Exe("git", "add", "-A")
		prj0.Exe("git", "commit", "-m", "test commit 1")
		prj0.Exe("git", "tag", "-a", "-m", "tag-msg-"+tag, tag)
		prj0.Exe("git", "push", "--tags")
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

	t.Run("does push unannotated tags", func(t *testing.T) {
		// --- Given ---
		bare := Bare(t)
		branch := randkit.Str()
		tag := "tag-" + branch

		ctx := t.Context()
		prj0 := prjkit.New(t, t.TempDir())
		prj0.Exe("git", "clone", bare, ".")
		prj0.Exe("git", "checkout", "-b", branch)
		prj0.CreateFileWith(branch, "file0.txt")
		prj0.Exe("git", "add", "-A")
		prj0.Exe("git", "commit", "-m", "test commit 1")
		prj0.Exe("git", "tag", tag)
		prj0.Exe("git", "push", "--tags")
		prj0.Close()

		// --- When ---
		err := Push(ctx, prj0.Root())

		// --- Then ---
		assert.NoError(t, err)

		prj1 := prjkit.New(t, t.TempDir())
		prj1.Exe("git", "clone", "--branch", branch, bare, ".")
		prj1.Close()

		assert.Equal(t, branch, prj1.ReadFileStr("file0.txt"))
		want := fmt.Sprintf("tag-%s  test commit 1\n", branch)
		assert.Equal(t, want, prj1.ExeStdout("git", "tag", "-n99"))
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
	prj.Exe("git", "add", "-A")
	prj.Exe("git", "commit", "-m", "test commit 1")
	prj.Exe("git", "push")
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

	t.Run("invalid repo error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		badRepo := "git@example.com:project/repo.git"
		dst := filepath.Join(t.TempDir(), "from-remote.txt")
		ctx, cxl := context.WithTimeout(ctx, 200*time.Millisecond)
		t.Cleanup(func() { cxl() })

		// --- When ---
		err := GetFile(ctx, badRepo, branch, "file0.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("invalid branch error", func(t *testing.T) {
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

	t.Run("invalid repo file error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "bad.txt", dst)

		// --- Then ---
		assert.ErrorIs(t, ErrUnkFile, err)
		assert.NoFileExist(t, dst)
	})

	t.Run("invalid destination error", func(t *testing.T) {
		// --- Given ---
		ctx := t.Context()
		dst := filepath.Join(t.TempDir(), "not_existing", "from-remote.txt")

		// --- When ---
		err := GetFile(ctx, bare, branch, "bad.txt", dst)

		// --- Then ---
		var e *fs.PathError
		assert.ErrorAs(t, &e, err)
		assert.Equal(t, filepath.Dir(dst), e.Path)
		assert.True(t, os.IsNotExist(err))
		assert.NoFileExist(t, dst)
	})

	t.Run("destination is a directory error", func(t *testing.T) {
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

	t.Run("very short deadline", func(t *testing.T) {
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
			assert.Equal(t, tc.want, IsHash(tc.in))
		})
	}
}

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

func Test_gitErrorOr(t *testing.T) {
	t.Run("message not covered by cases", func(t *testing.T) {
		// --- When ---
		err := gitErrorOr("message not covered by cases", nil)

		// --- Then ---
		assert.ErrorContain(t, "message not covered by cases", err)
	})

	t.Run("empty message", func(t *testing.T) {
		// --- When ---
		err := gitErrorOr("", ErrTest)

		// --- Then ---
		assert.ErrorIs(t, ErrTest, err)
	})

	t.Run("empty message nil err", func(t *testing.T) {
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
			"ambiguous rev range unknown tag",
			"ambiguous argument ...HEAD",
			ErrTest,
			ErrUnkTag,
		},
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
