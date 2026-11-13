# OBS-001: rejected authentication diagnostic capture (Linux)

Date: 2026-11-13. Run by Codex for Ricardo Lüders. Candidate: the commit containing this record, based on #18 integration `9ba18b799ed40cb806f19720e40c3947f1ebb9ec` (merged PR #368). Issue: #170; related authentication: #92.

## Scope and evidence

**Integration-tested:** extends the existing real-Git SDK/authentication/HTTP fixture with distinct attacker-supplied bearer, launch-code, CSRF and cookie markers. Invalid bearer/session cookie and invalid launch-code requests return 401; a logout with invalid CSRF returns 403. Response bodies exclude the supplied markers, and the legitimate session remains live after rejected logout.

The existing clean-shutdown stdout/stderr capture now excludes all four rejected markers as well as the percent-encoded launch code. The launch marker contains slash/plus characters to exercise query encoding. Every request remains part of the exact HTTP-status multiset and fresh trusted-correlation assertions added in PR #368. This checks rejected credential values, in addition to the successful session credentials already covered. Assertions do not print credentials or captured channels.

No runtime, contract, dependency, schema or architecture change. Storage is fake; HTTP, SDK authentication, Git and child-process output capture are real local components. Node injects browser Cookie/Origin headers.

## Commands and outcomes

- `make sdk-ts`: PASS, exit 0; 48 tests, zero failures/skips; generated-schema staleness and TypeScript checks passed.
- `make check`: PASS, exit 0; 346 matching Go/TypeScript fixture outcomes, 48 SDK tests, Platform tests, formatting, documentation, host/macOS/Windows vet and PostgreSQL-tagged vet. Local checks are not hosted CI.
- `trail check --base 9ba18b799ed40cb806f19720e40c3947f1ebb9ec`: PASS, exit 0, no conflicts.
- Final `make docs-check` and `git diff --check`: PASS, exit 0.

Logs: `/tmp/amaru-170-auth-sdk.log`, `/tmp/amaru-170-auth-check.log`. These are local files, not hosted artifacts.

## Refresh, environment and limits

Refreshed all open M3 issues' native blockers and delivery sections, #18 native children, open PRs and latest merged PRs on 2026-11-13. PR #368 is merged at the candidate base; preceding #367/#366/#365 add rejected attachment, SDK Project and PostgreSQL HTTP evidence. No competing open PR was present. #170's blockers #83/#84 and inherited #15 are closed and available on this base. Remaining #18 children #92/#170/#191/#219 target `feat/18-wp-m3-06-public-api-events`; #191's #120 and #219's #135 are closed. This bounded security-evidence follow-up needs no new architecture decision.

Other lanes retain their designated bases: #188/#197 target `feat/16-wp-m3-04-policy-grants-and`; #154/#124/#125 target `feat/19-wp-m3-07-worktrees-configuration-scopes`. #197 waits on #124; #125 waits on #197. Provider implementation children target `feat/17-wp-m3-05-providers-and-context` and remain behind open migration approval gates #80–#82; #189 also waits on #113 and #190 on #127/#113. Parent #16–#19 prerequisites #15 are closed; exit #20 still waits on all four parents. No issue/board state or approval decision changed.

Environment: Linux x86_64, kernel `7.2.11-100.fc43.x86_64`, host Go 1.26.0 (Platform selects 1.27.0), Node v25.7.0, pnpm 11.5.0, Git 2.56.0. Physical/VM classification unverified.

**Not validated:** product startup/CLI, PostgreSQL runtime or SDK composition, native browser cookies, real Providers, Execution/Event public transport, native macOS/Windows, Node 22 and unknown/encoded secret variants beyond the exercised launch-code encoding. Existing unresolved Worker timeout evidence in the preceding PR #368 record remains unresolved; this passing run does not establish its absence. #170/#92/#191/#18/M3 stay open. No merge or issue closure is part of this change. Existing shared checkout work and prior worktrees/branches were preserved.
