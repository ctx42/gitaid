// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package gitaid wraps the git command line and turns its output into typed
// results and errors.
//
// It is not intended to be a complete git wrapper. It implements just enough to
// get information about commits and tags in a project's repository (and to
// drive the handful of mutations release tooling needs), rather than to cover
// git's full command surface.
package gitaid

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// Git related errors.
var (
	// ErrNotRepo is error returned when a directory is not a git repository.
	ErrNotRepo = errors.New("not a git repository")

	// ErrEmptyRepo is an error returned when repository is initialized but
	// has no commits.
	ErrEmptyRepo = errors.New("empty git repository")

	// ErrUnkRev is an error returned when revision is unknown.
	ErrUnkRev = errors.New("unknown revision")

	// ErrNoTags is an error returned when repository has no tags.
	ErrNoTags = errors.New("no tags found")

	// ErrNoRemote is an error returned when repository has no remote
	// configured.
	ErrNoRemote = errors.New("no remote")

	// ErrUnkTag is an error returned when searched tag does not exist in
	// the repository.
	ErrUnkTag = errors.New("tag not found")

	// ErrUnkFile is an error returned when file or directory does not exist
	// in the repository.
	ErrUnkFile = errors.New("file not found")

	// ErrNotFile is returned when a path names a directory, a symlink, or
	// anything else that is not a regular file.
	ErrNotFile = errors.New("not a regular file")

	// ErrNotClean is an error returned when working directory has untracked
	// files or not committed changes.
	ErrNotClean = errors.New("working directory not clean")

	// ErrDetached is an error returned when repository has no branch checked
	// out because its HEAD is detached.
	ErrDetached = errors.New("detached HEAD")

	// ErrBadArg is returned when a revision, tag, remote, or path argument
	// starts with "-", so git would read it as an option.
	ErrBadArg = errors.New("argument looks like an option")

	// ErrGit is an error returned when git binary encounters unknown error. It
	// carries git's message and wraps the underlying exec error.
	ErrGit = errors.New("git error")
)

// IsRepo returns nil error if directory is initialized git repository,
// otherwise it returns ErrNotRepo. The empty string used for dir means current
// working directory.
func IsRepo(ctx context.Context, dir string) error {
	args := []string{"rev-parse", "--git-dir"}
	if _, err := runGitCmd(ctx, dir, args...); err != nil {
		return err
	}
	return nil
}

// IsEmpty returns true if given repository is empty. The empty repository is
// defined as one whose HEAD points at no commit.
func IsEmpty(ctx context.Context, repo string) (bool, error) {
	args := []string{"rev-parse", "-q", "--verify", "HEAD^{commit}"}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		// A quiet "--verify" exits 1 without a message when HEAD resolves
		// to no commit; anything else is a real failure.
		ee, ok := errors.AsType[*exec.ExitError](err)
		if !ok || ee.ExitCode() != 1 {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// Branch returns the name of the branch checked out in the repository. It
// returns [ErrDetached] when HEAD is detached, because then there is no branch
// name to report. The empty string used for repo means current working
// directory.
func Branch(ctx context.Context, repo string) (string, error) {
	args := []string{"branch", "--show-current"}
	name, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", ErrDetached
	}
	return name, nil
}

// ProjectName returns project name based on repository name at origin or,
// without an origin, the name of the repository's top-level directory, which
// for a bare repository is its git directory without a ".git" suffix.
//
// Example:
//
//	ssh://git@example.com:vr/skw-proj.git
//
// In above example project name will be "skw-proj".
func ProjectName(ctx context.Context, repo string) (string, error) {
	args := []string{"remote", "get-url", "origin"}
	origin, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		if errors.Is(err, ErrNoRemote) {
			return topLevelName(ctx, repo)
		}
		return "", err
	}
	// The name follows the last "/" of a URL or path, or the ":" of an
	// scp-like "user@host:name.git" with no directory in front of the name.
	origin = strings.TrimRight(origin, "/")
	name := origin[strings.LastIndexAny(origin, "/:")+1:]
	name = strings.TrimSuffix(name, ".git")
	return name, nil
}

// topLevelName returns the name of the top-level directory of repo, or of the
// git directory without a ".git" suffix when repo is bare and so has no work
// tree.
func topLevelName(ctx context.Context, repo string) (string, error) {
	bare, err := runGitCmd(ctx, repo, "rev-parse", "--is-bare-repository")
	if err != nil {
		return "", err
	}
	args := []string{"rev-parse", "--show-toplevel"}
	if bare == "true" {
		args = []string{"rev-parse", "--path-format=absolute", "--git-dir"}
	}
	dir, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(filepath.Base(dir), ".git"), nil
}

// ProjectOrigin returns the repository origin URL. If repository has no origin
// it returns empty string and nil error.
func ProjectOrigin(ctx context.Context, repo string) (string, error) {
	args := []string{"config", "--local", "-l"}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	scn := bufio.NewScanner(strings.NewReader(sout))
	for scn.Scan() {
		after, found := strings.CutPrefix(scn.Text(), "remote.origin.url=")
		if found {
			return after, nil
		}
	}
	if err = scn.Err(); err != nil {
		return "", fmt.Errorf("read git config: %w", err)
	}
	return "", nil
}

// FirstHash returns the abbreviated hash of the first commit in the repo. Git
// abbreviates to at least seven characters, extending it only as far as needed
// to stay unambiguous. If the repository has no commits, it will return
// ErrEmptyRepo error.
func FirstHash(ctx context.Context, repo string) (string, error) {
	args := []string{
		"rev-list", "--max-parents=0", "--abbrev-commit", "--abbrev=7", "HEAD",
	}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	return firstLine(strings.NewReader(sout)), nil
}

// LatestHash returns the abbreviated hash of the latest commit, at least seven
// characters long whatever core.abbrev says. If repository has no commits it
// will return ErrEmptyRepo error.
func LatestHash(ctx context.Context, repo string) (string, error) {
	args := []string{"log", "--abbrev=7", "--pretty=format:%h", "-n", "1"}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	return sout, nil
}

// RevDate returns the commit date of the given revision. A tag, annotated or
// not, resolves to the commit it points at. Returns ErrUnkRev when repository
// is empty or revision doesn't exist. To distinguish between both cases use
// IsEmpty.
func RevDate(ctx context.Context, repo, rev string) (time.Time, error) {
	if err := noOption(rev); err != nil {
		return time.Time{}, err
	}
	// Peeling to the commit keeps "git show" from printing an annotated
	// tag's header in front of the timestamp.
	args := []string{"show", "--no-patch", "--format=%ct", rev + "^{commit}"}
	dt, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return time.Time{}, err
	}
	ts, err := strconv.ParseInt(dt, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("commit date of %s: %w", rev, err)
	}
	return time.Unix(ts, 0), nil
}

// ClosestTag returns the closest tag reachable from the start revision. The
// empty startRev means HEAD. If the returned tag is the same as the startRev,
// it means this is the only revision in the repository. It returns [ErrUnkRev]
// when startRev does not name a commit.
func ClosestTag(ctx context.Context, repo, startRev string) (string, error) {
	if err := noOption(startRev); err != nil {
		return "", err
	}
	args := []string{"describe", "--tags", "--abbrev=0"}
	if startRev != "" {
		// Describing "startRev~" fails both for an unknown startRev and for
		// a root commit, so the revision is verified up front and a failure
		// below can only mean the latter.
		vfy := []string{"rev-parse", "-q", "--verify", startRev + "^{commit}"}
		if _, err := runGitCmd(ctx, repo, vfy...); err != nil {
			// A quiet "--verify" exits 1 without a message when the
			// revision is unknown; anything else is a real failure.
			ee, ok := errors.AsType[*exec.ExitError](err)
			if !ok || ee.ExitCode() != 1 {
				return "", err
			}
			return "", fmt.Errorf("%w: %s", ErrUnkRev, startRev)
		}
		args = append(args, startRev+"~")
	}
	rev, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoTags):
			return "", nil

		case startRev != "" && errors.Is(err, ErrUnkRev):
			return startRev, nil

		default:
			return "", err
		}
	}
	return rev, nil
}

// DescribeOpt is an option changing the behavior of [Describe].
type DescribeOpt func(*describeCfg)

// describeCfg is the configuration the [DescribeOpt] options build.
type describeCfg struct {
	// The glob restricting which tags are considered.
	match string
}

// WithMatch restricts [Describe] to the tags matching the glob, so that a tag
// like "nightly" or "build-42" does not shadow a release tag. The glob is
// matched against the tag name; the commit count still spans the commits the
// skipped tags point at. Without this option, or with an empty glob, every
// tag is considered.
func WithMatch(glob string) DescribeOpt {
	return func(cfg *describeCfg) { cfg.match = glob }
}

// The words a version built here carries. They mean the same thing in every
// module that reads one, so none of them is ever spelled two ways.
const (
	// StateClean is the state of a working tree with nothing outstanding.
	StateClean = "clean"

	// StateDirty is the state of a working tree with outstanding changes,
	// untracked files included. See [IsClean] for what counts.
	StateDirty = "dirty"

	// LabelDev is the first pre-release identifier [Derive] gives a version
	// that is not a release. It marks a development build, which is a
	// different thing from [StateDirty]: a build may be either, both or
	// neither.
	LabelDev = "dev"
)

// MatchSemVer is the tag glob that keeps a name which is not a version - a
// date stamp, a moving pointer - from being taken for a release. It is the
// match [Derive] uses unless [WithMatch] overrides it, and the one a project
// tagging anything besides releases wants for [Describe].
const MatchSemVer = "v[0-9]*.[0-9]*.[0-9]*"

// Bump levels [Derive] advances the described tag by.
const (
	// BumpPatch advances the patch version. It is the level [Derive] uses
	// for an empty bump, because guessing low leaves a gap nobody fills
	// where guessing high publishes a version outranking a real release.
	BumpPatch = "patch"

	// BumpMinor advances the minor version and zeroes the patch.
	BumpMinor = "minor"

	// BumpMajor advances the major version and zeroes the minor and patch.
	BumpMajor = "major"
)

// ErrBadBump is returned when a bump level names none of [BumpPatch],
// [BumpMinor], or [BumpMajor].
var ErrBadBump = errors.New("unknown version bump")

// dirtyMark is what [Describe] appends when the working tree is not clean.
// It is git's own default for "describe --dirty".
const dirtyMark = "-" + StateDirty

// noTagBase is the tag [Describe] describes against when no considered tag is
// a version. Git would answer with the bare short hash, or with a tag that is
// not a version at all; the lowest possible release keeps the result SemVer
// and keeps it sorting below every real tag.
const noTagBase = "v0.0.0"

// isSemVer returns true if s is a SemVer 2.0 version, with or without the
// leading "v" git tags conventionally carry.
//
// It parses with [semver.StrictNewVersion] rather than [semver.NewVersion],
// because the latter is deliberately lenient: it reads "v1.2" as 1.2.0 and
// the date stamp "2026-01-15" as 2026.0.0-01-15, and a tag that is not a
// version has to be rejected here, not coerced into one.
func isSemVer(s string) bool {
	_, err := semver.StrictNewVersion(strings.TrimPrefix(s, "v"))
	return err == nil
}

// Describe returns a human-readable name for the current state of the
// repository, always as a valid SemVer 2.0 version. The empty string used for
// repo means the current working directory.
//
// When HEAD sits exactly on a considered tag the result is that tag alone.
// Otherwise it is the closest considered tag, the number of commits made since
// it, and the short HEAD hash, in the "<tag>-<count>-g<hash>" form. A dirty
// working tree appends "-dirty". Both annotated and lightweight tags count,
// and the tag is rendered verbatim, so a leading "v" is kept.
//
// A tree is dirty when "git status --porcelain" reports anything at all,
// untracked files included - see [IsClean]. An untracked file may be source
// the build needs just as easily as a stray log, and nothing here can tell
// the two apart; keeping build artefacts in ignored directories is the
// project's job.
//
// The count, the hash and the "dev" marker land in a single alphanumeric
// pre-release identifier, so the result is a version the whole way down. Note
// that SemVer ranks a pre-release below its normal version, so a description
// of a commit past a tag sorts below that tag; a caller that needs a develop
// build to outrank the release it descends from bumps the tag first and
// describes against the successor.
//
// The closest considered tag has to be a version itself. When it is not - a
// tag like "nightly", "build-42" or "rel/2026-01" - it is treated as no tag
// at all rather than rendered verbatim. Note that this does not reach past it
// to an older version tag; git picks the closest tag and the check only
// accepts or rejects that one. Use [WithMatch] to make git skip the names
// that are not versions in the first place.
//
// When no considered tag is usable - the repository has none, the closest one
// is not a version, or none matches the glob - the same form is built against
// the synthetic tag "v0.0.0", counting every commit reachable from HEAD. So
// the result is never a bare hash, and [ErrNoTags] is never reported.
//
// Every tag is considered unless [WithMatch] narrows them to a glob. Because
// the output shape varies, and because a tag name may itself contain "-", a
// caller that parses the result strips the optional "-dirty" suffix first, then
// tests for the "-<count>-g" infix and splits from the right.
//
// Examples:
//
//	// HEAD sits on tag "v0.1.0" - the tag stands alone.
//	Describe(ctx, repo) // "v0.1.0"
//
//	// Three commits were made since tag "v1.2.0".
//	Describe(ctx, repo) // "v1.2.0-3-g9ab3d41"
//
//	// The working tree has uncommitted changes.
//	Describe(ctx, repo) // "v0.1.0-dirty"
//
//	// Both: one commit since the tag, and a dirty working tree.
//	Describe(ctx, repo) // "v0.1.0-1-g9ab3d41-dirty"
//
//	// The lightweight tag "nightly" is closer to HEAD than "v0.1.0". It
//	// is not a version, so it counts as no tag and "v0.1.0" is not
//	// reached for - every commit is counted from "v0.0.0" instead.
//	Describe(ctx, repo) // "v0.0.0-4-g7b27033"
//
//	// The same state, but the glob makes git skip "nightly" outright; the
//	// count still spans the commit "nightly" points at.
//	Describe(ctx, repo, WithMatch("v[0-9]*")) // "v0.1.0-2-g7b27033"
//
//	// No tag is reachable at all - four commits describe against "v0.0.0".
//	Describe(ctx, repo) // "v0.0.0-4-ge11e688"
//
//	// A version tag exists, but the glob matches none - the same fallback.
//	Describe(ctx, repo, WithMatch("v9.*")) // "v0.0.0-4-ge11e688"
//
// It returns [ErrEmptyRepo] when the repository has no commits, and
// [ErrNotRepo] when repo is not a git repository.
func Describe(
	ctx context.Context,
	repo string,
	opts ...DescribeOpt,
) (string, error) {

	dsc, err := describe(ctx, repo, opts)
	if err != nil {
		return "", err
	}
	return dsc.String(), nil
}

// description is the state of a repository that [Describe] renders and
// [Derive] builds a [Version] from. Both read the fields, never the rendered
// string, because a tag may itself end in "-dirty" or look like a
// "<tag>-<count>-g<hash>" description.
type description struct {
	tag   string // The considered tag, or noTagBase when none is usable.
	count int    // Commits made since tag.
	hash  string // Short hash HEAD points at.
	dirty bool   // Whether the working tree has outstanding changes.
}

// String renders the description in the [Describe] form.
func (dsc description) String() string {
	rev := dsc.tag
	if dsc.count > 0 {
		rev = fmt.Sprintf("%s-%d-g%s", dsc.tag, dsc.count, dsc.hash)
	}
	if dsc.dirty {
		rev += dirtyMark
	}
	return rev
}

// describe describes HEAD of repo against the closest considered tag; see
// [Describe] for the rules.
func describe(
	ctx context.Context,
	repo string,
	opts []DescribeOpt,
) (description, error) {

	var cfg describeCfg
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	// --long renders "<tag>-<count>-g<hash>" even when the count is zero, so
	// one parse covers both the on-tag and the past-the-tag case, and a tag
	// holding "-" stays unambiguous. The short forms are rebuilt below.
	//
	// --dirty is not passed: it diffs the index against HEAD and cannot see
	// untracked files, which count here. [IsClean] is the one definition, so
	// the marker is appended below instead.
	//
	// --abbrev=7 overrides a shorter core.abbrev, which would yield a hash
	// [IsHash] rejects and so a parse that falls back to no tag at all.
	args := []string{"describe", "--long", "--tags", "--abbrev=7"}
	if cfg.match != "" {
		args = append(args, "--match", cfg.match)
	}
	rev, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		if !errors.Is(err, ErrNoTags) {
			return description{}, err
		}
		return describeNoTag(ctx, repo)
	}

	tag, cnt, hash, ok := splitDescribe(rev)
	if !ok || !isSemVer(tag) {
		return describeNoTag(ctx, repo)
	}
	count, err := strconv.Atoi(cnt)
	if err != nil {
		return describeNoTag(ctx, repo)
	}

	clean, err := IsClean(ctx, repo)
	if err != nil {
		return description{}, err
	}
	return description{tag: tag, count: count, hash: hash, dirty: !clean}, nil
}

// splitDescribe takes the output of "git describe --long" and splits it into
// the tag, the commit count and the short hash. It reports false when desc
// does not have that shape.
func splitDescribe(desc string) (tag, cnt, hash string, ok bool) {
	// The tag may itself hold "-g", so the split works from the right.
	i := strings.LastIndex(desc, "-g")
	if i < 0 {
		return "", "", "", false
	}
	hash, desc = desc[i+2:], desc[:i]
	if !IsHash(hash) {
		return "", "", "", false
	}
	if i = strings.LastIndex(desc, "-"); i < 0 {
		return "", "", "", false
	}
	cnt, tag = desc[i+1:], desc[:i]
	if cnt == "" || strings.TrimLeft(cnt, "0123456789") != "" || tag == "" {
		return "", "", "", false
	}
	return tag, cnt, hash, true
}

// describeNoTag describes HEAD of a repository where no considered tag is a
// version against noTagBase.
func describeNoTag(ctx context.Context, repo string) (description, error) {
	// Git answers "No names found, cannot describe anything" both for a
	// repository without commits and for one without tags, so "--always" is
	// not passed and the two are told apart here instead.
	empty, err := IsEmpty(ctx, repo)
	if err != nil {
		return description{}, err
	}
	if empty {
		return description{}, ErrEmptyRepo
	}

	cnt, err := CountCommits(ctx, repo, "HEAD")
	if err != nil {
		return description{}, err
	}
	hash, err := LatestHash(ctx, repo)
	if err != nil {
		return description{}, err
	}
	clean, err := IsClean(ctx, repo)
	if err != nil {
		return description{}, err
	}
	dsc := description{tag: noTagBase, count: cnt, hash: hash, dirty: !clean}
	return dsc, nil
}

// Version is the state of a repository rendered as a SemVer 2.0 version that
// orders correctly against the releases it descends from. [Derive] builds it.
type Version struct {
	// The version itself: the tag verbatim when Release is true, and the
	// assembled development version otherwise.
	//
	// Example: v0.4.1-dev.3.dirty+g7f93fb4
	Rev string

	// Tag the repository was described against, verbatim.
	//
	// Example: v0.4.0
	Tag string

	// Short commit hash HEAD points at.
	//
	// Example: 7f93fb4
	Hash string

	// Commits made since Tag.
	Count int

	// Dirty reports whether the working tree had outstanding changes. See
	// [IsClean] for what counts.
	Dirty bool

	// Release reports whether HEAD is a clean checkout sitting exactly on
	// Tag - the one state whose version is the tag itself.
	Release bool
}

// Derive returns the [Version] of the repository at repo. The empty string
// used for repo means the current working directory.
//
// A release - a clean tree sitting exactly on a considered tag - is that tag
// and nothing else. Every other state is a pre-release of the release the
// bump names, so that a development build sorts above the release it
// descends from and below the one it heads towards:
//
//	<bumped tag>-dev.<count>[.dirty]+g<hash>
//
// This is the form [Describe] cannot produce. Its own output packs the count
// and the hash into one pre-release identifier, which SemVer then ranks below
// the tag and compares as text; see docs/versioning.md. Here the count is an
// identifier of its own so it compares numerically, the hash moves behind "+"
// where ordering ignores it, and the tag is bumped first so the result
// outranks the release it was built on.
//
// The bump is one of [BumpPatch], [BumpMinor] or [BumpMajor]; an empty string
// means [BumpPatch]. Which one a commit range deserves is a policy this
// package does not decide - read it off the commits with [Messages], or take
// it from configuration. A patch bump of a pre-release tag extends the tag's
// own pre-release instead, so a build one commit past v1.0.0-rc.1 is
// v1.0.0-rc.1.dev.1: above the tag, below v1.0.0-rc.2 and v1.0.0.
//
// Tags are restricted to [MatchSemVer] unless [WithMatch] says otherwise. It
// returns [ErrBadBump] for an unknown bump, and what [Describe] returns for a
// repository it cannot describe.
func Derive(
	ctx context.Context,
	repo, bump string,
	opts ...DescribeOpt,
) (Version, error) {

	if err := checkBump(bump); err != nil {
		return Version{}, err
	}
	opts = append([]DescribeOpt{WithMatch(MatchSemVer)}, opts...)
	dsc, err := describe(ctx, repo, opts)
	if err != nil {
		return Version{}, err
	}
	ver := Version{
		Tag:   dsc.tag,
		Hash:  dsc.hash,
		Count: dsc.count,
		Dirty: dsc.dirty,
	}

	base, err := semver.NewVersion(ver.Tag)
	if err != nil {
		return Version{}, fmt.Errorf("%s: %w", ver.Tag, err)
	}
	if ver.Count == 0 && !ver.Dirty {
		ver.Rev, ver.Release = base.Original(), true
		return ver, nil
	}

	pre := fmt.Sprintf("%s.%d", LabelDev, ver.Count)
	if ver.Dirty {
		pre += "." + StateDirty
	}
	var next semver.Version
	if base.Prerelease() != "" && (bump == "" || bump == BumpPatch) {
		// A patch bump would land on the release the tag heads towards,
		// and "dev" ranks below most pre-release labels, so the build
		// would sort below the tag it descends from.
		next, pre = *base, base.Prerelease()+"."+pre
	} else if next, err = bumped(base, bump); err != nil {
		return Version{}, err
	}
	if next, err = next.SetPrerelease(pre); err != nil {
		return Version{}, fmt.Errorf("pre-release %s: %w", pre, err)
	}
	if next, err = next.SetMetadata("g" + ver.Hash); err != nil {
		return Version{}, fmt.Errorf("build metadata g%s: %w", ver.Hash, err)
	}
	ver.Rev = next.Original()
	return ver, nil
}

// bumped returns the release base advances to by one bump level. On a 0.x
// version a major bump advances the minor instead: SemVer leaves 0.y.z
// explicitly unstable, and a project is not declaring 1.0.0 by writing one
// breaking change.
func bumped(base *semver.Version, bump string) (semver.Version, error) {
	if err := checkBump(bump); err != nil {
		return semver.Version{}, err
	}
	if base.Major() == 0 && bump == BumpMajor {
		bump = BumpMinor
	}
	switch bump {
	case BumpMajor:
		return base.IncMajor(), nil

	case BumpMinor:
		return base.IncMinor(), nil

	default:
		return base.IncPatch(), nil
	}
}

// checkBump returns [ErrBadBump] unless bump is one of [BumpPatch],
// [BumpMinor], [BumpMajor], or the empty string.
func checkBump(bump string) error {
	switch bump {
	case "", BumpPatch, BumpMinor, BumpMajor:
		return nil
	default:
		return fmt.Errorf("%s: %w", bump, ErrBadBump)
	}
}

// CountCommits returns the number of commits reachable from rev. The empty
// string used for rev means HEAD.
func CountCommits(ctx context.Context, repo, rev string) (int, error) {
	if err := noOption(rev); err != nil {
		return 0, err
	}
	if rev == "" {
		rev = "HEAD"
	}
	sout, err := runGitCmd(ctx, repo, "rev-list", "--count", rev)
	if err != nil {
		return 0, err
	}
	cnt, err := strconv.Atoi(sout)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", sout, err)
	}
	return cnt, nil
}

// Messages returns the full commit messages - the subject and the body of
// each - for the commits in rng, oldest first. The rng is any revision range
// git log accepts, for example "v1.2.3..HEAD"; the empty string means every
// commit reachable from HEAD.
//
// Where [ChangeLog] keeps only the subject line, this keeps the body too, so a
// caller can read a footer such as "BREAKING CHANGE:".
//
// A commit with an empty message contributes no entry, so the result may be
// shorter than the range and its indices do not track the commits.
func Messages(ctx context.Context, repo, rng string) ([]string, error) {
	if err := noOption(rng); err != nil {
		return nil, err
	}
	if rng == "" {
		rng = "HEAD"
	}
	args := []string{"log", "--reverse", "--pretty=format:%x00%B", rng}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return nil, err
	}

	var msgs []string
	for msg := range strings.SplitSeq(sout, "\x00") {
		if msg = strings.TrimSpace(msg); msg != "" {
			msgs = append(msgs, msg)
		}
	}
	return msgs, nil
}

// ChangeLog generates changelog of the commits HEAD has and the given base
// revision does not, so a base on a diverged branch adds none of its own
// commits. The empty base means every commit reachable from HEAD.
// The changelog messages are constructed from the first line of the commit
// message. A commit with an empty message contributes no entry. It returns
// [ErrUnkTag] when the base revision does not exist.
func ChangeLog(ctx context.Context, repo, rev string) ([]string, error) {
	if err := noOption(rev); err != nil {
		return nil, err
	}
	base := rev
	if base != "" {
		rev = base + ".."
	}
	rev += "HEAD"

	// A NUL byte, which a commit message cannot hold, separates the
	// messages, so no body line can pass for the start of the next one.
	args := []string{"log", "--reverse", "--pretty=format:%x00%B", rev}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		if base != "" && errors.Is(err, ErrUnkRev) {
			return nil, fmt.Errorf("%w: %s", ErrUnkTag, base)
		}
		return nil, err
	}

	var entries []string
	for msg := range strings.SplitSeq(sout, "\x00") {
		if msg = strings.TrimSpace(msg); msg == "" {
			continue
		}
		subject, _, _ := strings.Cut(msg, "\n")
		entries = append(entries, strings.TrimSpace(subject))
	}
	return entries, nil
}

// Init initializes git repository in given directory. The empty string used
// for dir means current working directory.
func Init(ctx context.Context, dir string) error {
	args := []string{"init"}
	if _, err := runGitCmd(ctx, dir, args...); err != nil {
		return err
	}
	return nil
}

// AddRemote adds remote named origin to git repository.
func AddRemote(ctx context.Context, dir, remote string) error {
	if err := noOption(remote); err != nil {
		return err
	}
	args := []string{"remote", "add", "origin", remote}
	if _, err := runGitCmd(ctx, dir, args...); err != nil {
		return err
	}
	return nil
}

// IsClean returns true if given repository is clean and has no untracked files.
func IsClean(ctx context.Context, repo string) (bool, error) {
	args := []string{"status", "--porcelain"}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return false, err
	}
	return sout == "", nil
}

// WorkTreeStatus returns the working tree status: [StateClean] or
// [StateDirty].
func WorkTreeStatus(ctx context.Context, repo string) (string, error) {
	clean, err := IsClean(ctx, repo)
	if err != nil {
		return "", err
	}
	if clean {
		return StateClean, nil
	}
	return StateDirty, nil
}

// Add adds files to the index. Every pth is a path, even one starting with
// "-".
func Add(ctx context.Context, repo string, pth ...string) error {
	args := append([]string{"add", "--"}, pth...)
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// AddAll adds all files to the index. The empty string used for repo directory
// means current working directory.
func AddAll(ctx context.Context, repo string) error {
	args := []string{"add", "-A"}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// Commit commits with a message. The empty string used for repo directory
// means current working directory.
func Commit(ctx context.Context, repo, msg string) error {
	args := []string{"commit", "-m", msg}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// Tag tags the current revision with tag and message. The empty string used
// for working directory means current working directory.
func Tag(ctx context.Context, repo, tag, msg string) error {
	if err := noOption(tag); err != nil {
		return err
	}
	args := []string{"tag", "-a", "-m", msg, tag}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// Push will push the current branch and tags to the origin. When deadline on
// the context is not set it will be set to 15s. It returns [ErrDetached] when
// HEAD is detached, because then there is no branch to push. The empty string
// used for repo directory means current working directory.
func Push(ctx context.Context, repo string) error {
	ctx, cxl := withTimeout(ctx, 15*time.Second)
	defer cxl()
	if _, err := Branch(ctx, repo); err != nil {
		return err
	}
	args := []string{"push", "--follow-tags", "origin", "HEAD"}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// GetFile gets a file from given repository, branch or tag, source path and
// stores it in dst. When deadline on the context is not set it will be set to
// 10s. The empty string used for repo directory means current working
// directory, and a relative repo path is resolved against it too. It returns
// [ErrUnkFile] when src does not exist and [ErrNotFile] when src names a
// directory or anything else that is not a regular file.
func GetFile(ctx context.Context, repo, branch, src, dst string) error {
	if err := noOption(branch, src); err != nil {
		return err
	}
	ctx, cxl := withTimeout(ctx, 10*time.Second)
	defer cxl()
	tim, _ := ctx.Deadline()
	waitDelay := time.Until(tim)
	if repo == "" {
		repo = "."
	}

	args := []string{"archive", "--remote=" + repo, "--format=tar", branch, src}
	eout := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stderr = eout
	cmd.WaitDelay = waitDelay
	sout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git archive: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("git archive: %w", err)
	}
	data, errTar := extractFile(sout, src)
	// Drain what was not read, so the archive never blocks writing.
	_, _ = io.Copy(io.Discard, sout)
	if err = cmd.Wait(); err != nil {
		// A killed archive reports a signal, not why it was killed; the
		// context knows whether it was canceled or ran out of time.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if eout.Len() == 0 {
			return fmt.Errorf("git archive: %w", err)
		}
		return gitErrorOr(eout.String(), err)
	}
	if errTar != nil {
		return errTar
	}

	//nolint:gosec // dst is the caller-controlled destination path.
	if err = os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	return nil
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

// extractFile reads the tar stream r to its end and returns the content of
// the regular file called name. It returns [ErrNotFile] when name is not a
// regular file or other files sit below it, as they do for a directory, and
// [ErrUnkFile] when the stream holds no entry called name.
func extractFile(r io.Reader, name string) ([]byte, error) {
	name = path.Clean(name)
	var data []byte
	var found, other bool
	trd := tar.NewReader(r)
	for {
		hdr, err := trd.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		switch typ := hdr.Typeflag; {
		case typ == tar.TypeDir || typ == tar.TypeXGlobalHeader:
			continue

		case typ == tar.TypeReg && hdr.Name == name:
			if data, err = io.ReadAll(trd); err != nil {
				return nil, fmt.Errorf("read archive: %w", err)
			}
			found = true

		default:
			other = true
		}
	}
	if other {
		return nil, fmt.Errorf("%w: %s", ErrNotFile, name)
	}
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrUnkFile, name)
	}
	return data, nil
}

// hashRx represents commit hash.
var hashRx = regexp.MustCompile("^[0-9a-f]{7,}$")

// IsHash returns true if s is git hash.
func IsHash(s string) bool { return hashRx.MatchString(s) }

// noOption returns [ErrBadArg] for the first of args that starts with "-".
// Git parses such a value as an option, not as the revision, tag, remote, or
// path it is passed as; "--output=<file>" or "--exec=<cmd>" then writes a file
// or runs a command. The empty string passes.
func noOption(args ...string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("%w: %q", ErrBadArg, arg)
		}
	}
	return nil
}

// runGitCmd runs git command in given repo with arguments. Returns messages
// written by the git to standard output as strings and error if any. The empty
// string used for repo directory means current working directory.
func runGitCmd(
	ctx context.Context,
	repo string,
	args ...string,
) (string, error) {

	sout, eout := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout, cmd.Stderr = sout, eout
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return "", gitErrorOr(gitMessage(eout.String()), err)
	}
	return strings.TrimSpace(sout.String()), nil
}

// gitMessage picks the line of git's stderr that explains a failure: the
// first one starting with "fatal:" or "error:", or the first line when none
// does. Warnings, hints, and trace output may precede that line.
func gitMessage(stderr string) string {
	for line := range strings.Lines(stderr) {
		line = strings.TrimSpace(line)
		fatal := strings.HasPrefix(line, "fatal:")
		if fatal || strings.HasPrefix(line, "error:") {
			return line
		}
	}
	return firstLine(strings.NewReader(stderr))
}

// gitErrorOr takes error message printed by the git command and returns a
// matching sentinel error or, for a message no case matches, [ErrGit] with the
// message text and err wrapped. If the message text is empty err will be
// returned.
//
//nolint:cyclop
func gitErrorOr(msg string, err error) error {
	switch {
	case msg == "":
		if err == nil {
			return errors.New("empty git error message and nil error parameter")
		}
		return err

	case strings.Contains(msg, "ambiguous argument 'HEAD'"):
		return ErrEmptyRepo

	case strings.Contains(msg, "ambiguous argument ") &&
		strings.Contains(msg, "unknown revision or path"):
		return ErrUnkRev

	case strings.Contains(msg, "Not a valid object name HEAD"):
		return ErrEmptyRepo

	case strings.Contains(msg, "Not a valid object name "):
		return ErrUnkRev

	case strings.Contains(msg, "no such ref: "):
		return ErrUnkRev

	case strings.Contains(msg, "not a git repository"):
		return ErrNotRepo

	case strings.Contains(msg, "bad revision 'HEAD'"):
		return ErrEmptyRepo

	case strings.Contains(msg, "does not have any commits yet"):
		return ErrEmptyRepo

	case strings.Contains(msg, "cannot describe anything"):
		return ErrNoTags

	case strings.Contains(msg, "No tags can describe"):
		return ErrNoTags

	case strings.Contains(msg, "or path not in the working tree"):
		return ErrEmptyRepo

	case strings.Contains(msg, "No configured push destination"):
		return ErrNoRemote

	case strings.Contains(msg, "No such remote 'origin'"):
		return ErrNoRemote

	case strings.Contains(msg, "key does not contain a section: remote"):
		return ErrNoRemote

	case strings.Contains(msg, "did not match any files"):
		return ErrUnkFile

	case strings.Contains(msg, "can only be used inside a git repository"):
		return ErrNotRepo

	case msg == "exit status 1":
		return ErrGit

	default:
		if err == nil {
			return fmt.Errorf("%w: %s", ErrGit, msg)
		}
		return fmt.Errorf("%w: %s: %w", ErrGit, msg, err)
	}
}
