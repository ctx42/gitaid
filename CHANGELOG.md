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

