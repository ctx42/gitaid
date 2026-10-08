## v0.8.0 (Thu, 08 Oct 2026 12:10:35 UTC)
- feat(gitaid): add InitBranch, CommitEmpty and CreateBranch.

## v0.7.0 (Fri, 02 Oct 2026 17:48:25 UTC)
- fix(gitaid)!: reject arguments git would parse as options.
- fix(gitaid): read the commit date of an annotated tag in RevDate.
- fix(gitaid): return ErrUnkRev from ClosestTag for an unknown start.
- fix(gitaid): keep ChangeLog from splitting on ">>" body lines.
- fix(gitaid): abbreviate hashes to seven characters in Describe.
- fix(gitaid): report a canceled context from GetFile as canceled.
- fix(gitaid): derive the version from describe fields, not its text.
- fix(gitaid): take the project name after ":" in scp-like origins.
- fix(gitaid)!: name a project without origin after its top-level dir.
- test(gitaid): stop Push tests from pushing tags themselves.
- fix(gitaid): add context to errors returned without any.
- fix(gitaid): wrap ErrGit and the exec error for unknown git failures.
- fix(gitaid): map git's fatal line, not whatever stderr starts with.
- perf(gitaid): check for commits in IsEmpty without reading history.
- fix(gitaid): reject an unknown bump in Derive before running git.
- fix(gitaid)!: keep Derive builds above the pre-release tag they follow.
- fix(gitaid): stop GetFile hanging when tar exits before the archive.
- fix(gitaid)!: read the GetFile archive in Go and resolve repo from cwd.
- fix(gitaid): report a detached HEAD from Push and keep longer deadlines.
- fix(gitaid)!: list only HEAD's own commits in ChangeLog.
- test(gitaid): time out GetFile against a local silent remote.
- style(gitaid): align tests with the project test conventions.
- style(gitaid): tidy test setup and table layout.
- refactor(gitaid): move firstLine and its test to helpers files.
- docs(gitaid): add Derive and Messages examples, fix Describe's.
- docs(gitaid): tidy godoc wording and link exported names.
- docs: bring AGENTS.md in line with the current package.
- chore: drop stale testkit v0.14.0 sums from go.sum.
- fix(gitaid): run git in the C locale.
- test(gitaid): assert the tar header error in extractFile.
- fix(gitaid): return ErrNoRemote from Push without an origin.
- fix(gitaid): ignore log.showSignature in log and show output.
- fix(gitaid): ignore inherited repository location variables.
- fix(gitaid): return the oldest root commit from FirstHash.
- test(gitaid): check the file ChangeLog's option probe would write.
- refactor(gitaid): move withTimeout to helpers.go.
- test(gitaid): name expected values want.
- style(gitaid): group LatestHash's result assertions.
- test(gitaid): carry tar entry content with its header.
- test(gitaid): cover IsHash length and character boundaries.
- style(gitaid): separate checkBump switch cases.
- docs(gitaid): fix ProjectName and gitErrorOr godoc wording.
- docs(gitaid): describe where ClosestTag starts searching.
- refactor(gitaid): drop the "exit status 1" case from gitErrorOr.
- test(gitaid): name os.WriteFile in the GetFile destination comment.
- refactor(gitaid): drop the default split func from firstLine.
- docs(gitaid): use a valid scp-style URL in the ProjectName example.
- docs: list every returned sentinel in the README error table.
- docs: sync README examples with example_test.go.
- docs: note that gitCommand drops inherited repository variables.
- docs: install the gitaid package, not the bare module.
- docs: switch README example markers to gmmce.
- docs: require git 2.31 or newer in the README.
- docs: add the CI workflow badge to the README.
- docs: limit the README's context claim to functions that run git.
- docs: list IsHash and the locale-proof errors in README features.
- refactor(gitaid)!: remove the unused ErrNotClean sentinel.
- test(gitaid): name the gitErrorOr table's case field testN.

## v0.6.2 (Thu, 24 Sep 2026 11:24:11 UTC)
- ci: add GitHub Actions workflow running race tests.
- fix(gitaid): push the current branch to origin regardless of config.

## v0.6.1 (Thu, 24 Sep 2026 10:01:14 UTC)
- test(gitaid): bump testkit and stop Test_Init assuming master.

## v0.6.0 (Thu, 24 Sep 2026 08:56:17 UTC)
- feat(gitaid): add Branch to report the checked-out branch.
- test(gitaid): cover remaining reachable error paths.

## v0.5.0 (Tue, 22 Sep 2026 13:20:29 UTC)
- feat(gitaid)!: add Derive for ordered SemVer versions.

## v0.4.0 (Tue, 22 Sep 2026 11:14:45 UTC)
- feat(gitaid): add WithMatch option, CountCommits and Messages.
- fix(gitaid): ignore a nil Describe option.
- docs: document the version strings Describe returns.
- feat(gitaid)!: make Describe always return SemVer.
- test(gitaid): cover describeNoTag and runGitCmd directly.

## v0.3.0 (Wed, 08 Jul 2026 15:07:59 UTC)
- feat: initial public release of gitaid.
- docs: update README to reflect public release status.

