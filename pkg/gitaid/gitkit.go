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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

	// ErrNotClean is an error returned when working directory has untracked
	// files or not committed changes.
	ErrNotClean = errors.New("working directory not clean")

	// ErrGit is an error returned when git binary encounters unknown error.
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
// defined as one without commits.
func IsEmpty(ctx context.Context, repo string) (bool, error) {
	if _, err := ChangeLog(ctx, repo, ""); err != nil {
		if errors.Is(err, ErrEmptyRepo) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// ProjectName returns project name based on repository name at origin or
// name of the directory where ".git" directory is located.
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
			dir := repo
			if dir == "" {
				if dir, err = os.Getwd(); err != nil {
					return "", err
				}
			}
			return filepath.Base(dir), nil
		}
		return "", err
	}
	name := filepath.Base(origin)
	name = strings.TrimSuffix(name, ".git")
	return name, nil
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
		return "", err
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

// LatestHash returns the latest commit hash. If repository has no commits it
// will return ErrEmptyRepo error.
func LatestHash(ctx context.Context, repo string) (string, error) {
	args := []string{"log", "--pretty=format:%h", "-n", "1"}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	return sout, nil
}

// RevDate returns date of the given revision. Returns ErrUnkRev when
// repository is empty or revision doesn't exist. To distinguish between both
// cases use IsEmpty.
func RevDate(ctx context.Context, repo, rev string) (time.Time, error) {
	args := []string{"show", "--pretty=format:%ct", "--no-patch", rev}
	dt, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return time.Time{}, err
	}
	ts, err := strconv.ParseInt(dt, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}

// ClosestTag returns the closest tag reachable from the start revision. The
// empty startRev means HEAD. If the returned tag is the same as the startRev,
// it means this is the only revision in the repository.
func ClosestTag(ctx context.Context, repo, startRev string) (string, error) {
	args := []string{"describe", "--tags", "--abbrev=0"}
	if startRev != "" {
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

// Describe returns a human-readable name for the current state of the
// repository. The empty string used for repo means the current working
// directory.
//
// When HEAD sits exactly on a considered tag the result is that tag alone.
// Otherwise it is the closest considered tag, the number of commits made since
// it, and the short HEAD hash, in the "<tag>-<count>-g<hash>" form. A dirty
// working tree appends "-dev". Both annotated and lightweight tags count, and
// the tag is rendered verbatim, so a leading "v" is kept.
//
// Every tag is considered unless [WithMatch] narrows them to a glob. Because
// the output shape varies, and because a tag name may itself contain "-" and
// "/", a caller that parses the result tests for the "-<count>-g" infix and
// splits from the right.
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
//	Describe(ctx, repo) // "v0.1.0-dev"
//
//	// The lightweight tag "nightly" is closer to HEAD than "v0.1.0".
//	Describe(ctx, repo) // "nightly-1-g7b27033"
//
//	// The same state, but the glob skips "nightly"; the count still
//	// spans the commit "nightly" points at.
//	Describe(ctx, repo, WithMatch("v[0-9]*")) // "v0.1.0-2-g7b27033"
//
//	// The closest matching tag has a hierarchical name.
//	Describe(ctx, repo, WithMatch("rel/*")) // "rel/v1.0.0-1-g96b2af8"
//
//	// No tag is reachable at all - the short hash stands alone.
//	Describe(ctx, repo) // "e11e688"
//
//	// A tag exists, but the glob matches none - the same fallback.
//	Describe(ctx, repo, WithMatch("rel-*")) // "e11e688"
//
// It falls back to the bare short hash when no tag is reachable - because the
// repository has none, or because none matches the glob - so it never reports
// [ErrNoTags].
//
// It returns [ErrEmptyRepo] when the repository has no commits, and
// [ErrNotRepo] when repo is not a git repository.
func Describe(
	ctx context.Context,
	repo string,
	opts ...DescribeOpt,
) (string, error) {

	var cfg describeCfg
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	args := []string{"describe", "--tags", "--always", "--dirty=-dev"}
	if cfg.match != "" {
		args = append(args, "--match", cfg.match)
	}
	rev, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return "", err
	}
	return rev, nil
}

// CountCommits returns the number of commits reachable from rev. The empty
// string used for rev means HEAD.
func CountCommits(ctx context.Context, repo, rev string) (int, error) {
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

// ChangeLog generates changelog between given base revision and the HEAD.
// The changelog messages are constructed from the first line of the commit
// message.
func ChangeLog(ctx context.Context, repo, rev string) ([]string, error) {
	if rev != "" {
		rev = fmt.Sprintf("%s...", rev)
	}
	rev += "HEAD"

	args := []string{"log", "--reverse", "--pretty=format:>>%B", rev}
	sout, err := runGitCmd(ctx, repo, args...)
	if err != nil {
		return nil, err
	}

	var entries []string
	scn := bufio.NewScanner(strings.NewReader(sout))
	scn.Split(bufio.ScanLines)
	for scn.Scan() {
		entry := scn.Text()
		entry = strings.TrimSpace(entry)
		if entry == "" || !strings.HasPrefix(entry, ">>") {
			continue
		}
		entry = entry[2:]
		entries = append(entries, entry)
	}
	if err = scn.Err(); err != nil {
		return nil, err
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

// WorkTreeStatus returns working tree status: `clean` or `dirty`.
func WorkTreeStatus(ctx context.Context, repo string) (string, error) {
	clean, err := IsClean(ctx, repo)
	if err != nil {
		return "", err
	}
	if clean {
		return "clean", nil
	}
	return "dirty", nil
}

// Add adds files to the index.
func Add(ctx context.Context, repo string, pth ...string) error {
	args := append([]string{"add"}, pth...)
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
	args := []string{"tag", "-a", "-m", msg, tag}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// Push will push the current branch and tags to the origin. The empty string
// used for repo directory means current working directory.
func Push(ctx context.Context, repo string) error {
	ctx, cxl := context.WithTimeout(ctx, 15*time.Second)
	defer cxl()
	args := []string{"push", "--follow-tags"}
	if _, err := runGitCmd(ctx, repo, args...); err != nil {
		return err
	}
	return nil
}

// GetFile gets a file from given repository, branch or tag, source path and
// stores it in dst. When deadline on the context is not set it will be set to
// 10s. The empty string used for repo directory means current working
// directory.
func GetFile(ctx context.Context, repo, branch, src, dst string) error {
	// Set deadline if not already set.
	var waitDelay time.Duration
	if tim, ok := ctx.Deadline(); ok {
		waitDelay = time.Until(tim)
	} else {
		waitDelay = 10 * time.Second
		var cxl func()
		ctx, cxl = context.WithTimeout(ctx, 10*time.Second)
		defer cxl()
	}

	remote := fmt.Sprintf("--remote=%s", repo)
	argsDwl := []string{"archive", remote, "--format=tar", branch, src}
	eoutDwl := &bytes.Buffer{}
	cmdDwl := exec.CommandContext(ctx, "git", argsDwl...)
	cmdDwl.Stderr = eoutDwl
	cmdDwl.Dir = filepath.Dir(dst)
	cmdDwl.WaitDelay = waitDelay

	soutTar, eoutTar := &bytes.Buffer{}, &bytes.Buffer{}
	cmdTar := exec.CommandContext(ctx, "tar", "-xO")
	cmdTar.Stdout = soutTar
	cmdTar.Stderr = eoutTar
	cmdTar.Dir = filepath.Dir(dst)
	cmdTar.WaitDelay = waitDelay

	r, w := io.Pipe()
	cmdDwl.Stdout = w
	cmdTar.Stdin = r

	if err := cmdDwl.Start(); err != nil {
		return err
	}
	if err := cmdTar.Start(); err != nil {
		// Unblock and reap the archive process before returning.
		_ = w.Close()
		_ = cmdDwl.Wait()
		return err
	}
	if err := cmdDwl.Wait(); err != nil {
		// Unblock the tar reader (no more input) and reap it, otherwise it
		// blocks reading the pipe until the context kills it.
		_ = w.Close()
		_ = cmdTar.Wait()
		if exitStatus(err) == -1 {
			return context.DeadlineExceeded
		}
		return gitErrorOr(eoutDwl.String(), err)
	}
	if err := w.Close(); err != nil {
		_ = cmdTar.Wait()
		return err
	}
	if err := cmdTar.Wait(); err != nil {
		return err
	}

	//nolint:gosec // dst is the caller-controlled destination path.
	fil, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	if _, err = io.Copy(fil, soutTar); err != nil {
		return err
	}
	return fil.Close()
}

// hashRx represents commit hash.
var hashRx = regexp.MustCompile("^[0-9a-f]{7,}$")

// IsHash returns true if s is git hash.
func IsHash(s string) bool { return hashRx.MatchString(s) }

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
		return "", gitErrorOr(firstLine(eout), err)
	}
	return strings.TrimSpace(sout.String()), nil
}

// firstLine returns the first trimmed line from a reader.
func firstLine(r io.Reader) string {
	scn := bufio.NewScanner(r)
	scn.Split(bufio.ScanLines)
	for scn.Scan() {
		return strings.TrimSpace(scn.Text())
	}
	return "" // A scan error yields the empty string, same as no input.
}

// gitErrorOr takes error message printed by the git command and returns a
// matching sentinel error or an error with the message text. If the message
// text is empty err will be returned.
//
//nolint:cyclop
func gitErrorOr(msg string, err error) error {
	switch {
	case msg == "":
		if err == nil {
			return errors.New("empty git error message and nil error parameter")
		}
		return err

	case strings.Contains(msg, "ambiguous argument ") &&
		strings.Contains(msg, "...HEAD"):
		return ErrUnkTag

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
		return errors.New(msg)
	}
}
