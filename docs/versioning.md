# Version strings

What [Describe] returns for a given state of a repository, why the strings
are shaped the way they are, and what they do and do not promise when
compared.

This is the reference for anything that consumes `Describe` output. The
implementation lives in `pkg/gitaid/gitkit.go`.

[Describe]: https://pkg.go.dev/github.com/ctx42/gitaid/pkg/gitaid#Describe

## The rule

`Describe` wraps `git describe`, so a result is the closest tag, how far HEAD
has moved past it, and which commit that is. Every result is a valid SemVer
2.0 version.

```
<tag>[-<count>-g<hash>][-dev]
```

The tag stands alone only when HEAD sits exactly on it and nothing is
modified. Starting from `v0.4.0`, on commit `7f93fb4`:

| Git state                     | `Describe` result        |
| ----------------------------- | ------------------------ |
| On the tag, clean             | `v0.4.0`                 |
| On the tag, dirty             | `v0.4.0-dev`             |
| 3 commits past the tag, clean | `v0.4.0-3-g7f93fb4`      |
| 3 commits past the tag, dirty | `v0.4.0-3-g7f93fb4-dev`  |
| No usable tag, clean          | `v0.0.0-12-g7f93fb4`     |
| No usable tag, dirty          | `v0.0.0-12-g7f93fb4-dev` |

`Describe` never returns a bare hash and never reports `ErrNoTags`. It
returns `ErrEmptyRepo` for a repository without commits and `ErrNotRepo` for
a directory that is not one.

## Why the result is always a version

SemVer §9 allows a hyphen inside a pre-release identifier - the grammar reads
`<non-digit> ::= <letter> | "-"`, and identifiers "MUST comprise only ASCII
alphanumerics and hyphens `[0-9A-Za-z-]`". Only the *first* hyphen after the
version core is structural; every later one is an ordinary character.

So `v0.4.0-3-g7f93fb4-dev` parses as the core `0.4.0` plus one alphanumeric
pre-release identifier, `3-g7f93fb4-dev`. The count, the hash and the `dev`
marker never need a separator of their own.

When the tag already carries a pre-release the same parts extend its last
identifier instead: `v1.0.0-rc.1-1-g7f93fb4` is `rc` and `1-1-g7f93fb4`.

The leading `v` is not part of SemVer 2.0 - the grammar published with the
spec rejects `v1.2.3`. It is the Go and git convention, and a tag is rendered
verbatim, so a result carries whatever the tag had. Implementations differ on
it: `golang.org/x/mod/semver` requires it and rejects a bare `1.2.3`,
`node-semver` accepts either and strips it, and a parser built straight from
the spec grammar needs it removed first. `Describe` accepts a tag with or
without it.

## The tag has to be a version too

A tag is rendered verbatim, so a tag that is not a version would produce a
string that is not one either. Such a tag is therefore treated as *no tag*,
and the fallback below applies.

| Tag           | A version? | One commit later         |
| ------------- | ---------- | ------------------------ |
| `v0.4.0`      | yes        | `v0.4.0-1-g7f93fb4`      |
| `0.4.0`       | yes        | `0.4.0-1-g7f93fb4`       |
| `v1.0.0-rc.1` | yes        | `v1.0.0-rc.1-1-g7f93fb4` |
| `v1.2`        | no         | `v0.0.0-12-g7f93fb4`     |
| `nightly`     | no         | `v0.0.0-12-g7f93fb4`     |
| `build-42`    | no         | `v0.0.0-12-g7f93fb4`     |
| `rel/v1.0.0`  | no         | `v0.0.0-12-g7f93fb4`     |
| `2026-01-15`  | no         | `v0.0.0-12-g7f93fb4`     |

This check only accepts or rejects the tag git picked. It does **not** reach
past a rejected tag to an older version tag: with `nightly` on HEAD~1 and
`v0.1.0` four commits back, the result is the `v0.0.0` fallback, not
`v0.1.0-4-g…`. Git chooses the closest tag before `Describe` sees anything.

`WithMatch` is how a caller gets the older tag instead - the glob is passed
to `git describe --match`, so git skips the unwanted names itself:

```go
gitaid.Describe(ctx, repo, gitaid.WithMatch("v[0-9]*.[0-9]*.[0-9]*"))
```

A project that tags anything other than releases wants this option. Without
it one stray `nightly` collapses the result to `v0.0.0`.

## The no-tag fallback

With no usable tag - none exists, the closest is not a version, or none
matches the glob - the same form is built against the synthetic tag
`v0.0.0`, counting every commit reachable from HEAD.

`v0.0.0` is chosen because a pre-release of it sorts below every real
release, so a build from an untagged tree can never appear to supersede a
tagged one. The count is the full history rather than a distance, which is
the only thing there is to count.

## What counts as dirty

A tracked file that differs from HEAD, staged or not. Untracked files are
ignored on purpose: an editor swap file or a stray build artefact must not
change the version.

That is `git describe --dirty`'s own rule. The fallback path does not go
through `git describe`, so it checks
`git status --porcelain --untracked-files=no` instead - the two paths have to
agree on what a dirty tree is.

## What the ordering does and does not promise

These strings are identifiers. They are safe to compare for equality, to
stamp into a binary, and to read. They are **not** a release ordering, and
three things go wrong if they are used as one. Verified with both
`golang.org/x/mod/semver` and `node-semver`:

```
v0.0.0-12-g7f93fb4  <  v0.4.0-10-g7f93fb4  <  v0.4.0-3-g7f93fb4
                    <  v0.4.0-3-g7f93fb4-dev  <  v0.4.0-dev  <  v0.4.0
```

**A description of a commit past a tag sorts below that tag.** SemVer §9
ranks any pre-release below its normal version, so `v0.4.0-3-g7f93fb4` <
`v0.4.0`: a develop build claims to be older than the release it was built
on top of. The dirty on-tag form `v0.4.0-dev` has the same problem.

The one exception is a tag that is itself a pre-release: §11 ranks a numeric
identifier below an alphanumeric one, so `v1.0.0-rc.1` <
`v1.0.0-rc.1-1-g7f93fb4`, which is the wanted direction by accident.

**The commit count is compared as text, not as a number.** It shares an
identifier with the hash, which makes that identifier alphanumeric, and §11
compares alphanumeric identifiers by ASCII. So `v0.4.0-10-g…` <
`v0.4.0-3-g…` - ten commits past the tag sort below three. Every repository
with more than nine commits since its last tag has this.

**The hash affects the ordering.** It sits in the same identifier, so two
builds at the same distance on different branches order by whatever their
hashes happen to spell.

Only the dirty marker behaves: `-dev` extends the identifier with a common
prefix, so a dirty tree sorts above the clean commit it was built from.

## Why this is not fixed here

All three follow from packing the count and the hash into one pre-release
identifier, which is what `git describe` does. `Describe` is a wrapper around
that command, and the output shape is the reason it is recognisable.

Separating the parts with `.` would fix the count - `dev.10` outranks `dev.3`
because §11 compares numeric identifiers numerically - but not the hash,
which would still be a pre-release identifier and still order arbitrarily.
The hash has to move behind `+` to be ignored (§10), and the version has to
name the *next* release rather than the last one before a develop build can
outrank the release it descends from. That last step is a policy decision -
patch, minor or major - that cannot be read off the repository alone.

So a correctly ordered version is a different string, built from the same
facts:

```
v0.4.1-dev.3.dirty+g7f93fb4
```

`Describe`, `CountCommits` and `Messages` supply the tag, the distance and
the commit subjects that scheme needs; choosing the successor and assembling
the string is the consumer's job, not this library's.

## Parsing a result

Prefer not to. When it is unavoidable, work from the right, because a tag may
itself contain `-`, and may even contain `-g`:

1. Strip a trailing `-dev`; its presence is the dirty flag.
2. Split at the last `-g`. The remainder is the short hash - check it is
   hex, because a tag like `v1.0.0-gamma` contains `-g` too.
3. Split what is left at the last `-`. The remainder is the count - check it
   is all digits.
4. Everything before that is the tag, verbatim.
5. If there is no `-g`, or either check fails, HEAD sits on the tag: the
   whole string is the tag and there is no count or hash.

A count of `0` never appears in a result; that case is rendered as the bare
tag, or as `<tag>-dev`.
