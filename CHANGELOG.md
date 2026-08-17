# Changelog

All notable changes to this fork (`denis-ev/forgejo-mcp`) are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Fork note:** Upstream (`raohwork/forgejo-mcp`) stopped at `v0.0.7` and has
> shown no maintainer activity for an extended period. This fork continues the
> line from `v0.1.0` onward, targeting current Forgejo (verified against
> **v16.0.1**). See the "About This Fork" section in the README for details.

## [Unreleased]

### Added

- **`help`** tool — zero-context onboarding for agents. Called with no
  arguments it returns one orientation document: the tool inventory grouped by
  domain, ID conventions (`index` vs `id` vs `comment_id`, and the label
  name-versus-ID gotcha), common workflows written as ordered tool sequences,
  pagination rules, version-gated tools, and known gaps. Called with a
  `topic` it drills into one domain or one tool, listing every parameter with
  its type, requiredness and constraints. The tool is read-only and idempotent
  and never contacts the Forgejo server.
  ([#11](https://github.com/denis-ev/forgejo-mcp/issues/11))
- The tool inventory is **generated from the live tool registry**, so it cannot
  drift as tools are added: `tools.Register` now indexes each tool definition
  alongside the domain inferred from its implementation package.

## [0.7.0] - 2026-08-13

### Added

- **`get_ci_status`** tool — answer "is this green?" in a single call for a
  pull request, branch, tag, or commit. Merges commit statuses and Forgejo
  Actions jobs into one normalized check list, rolls them up to a single
  state, and reports the `run_id`/`job_id` of failing Actions jobs so their
  logs can be fetched directly with `get_action_job_logs`. Previously this
  required 5–6 correlated calls across two unrelated APIs.
  ([#8](https://github.com/denis-ev/forgejo-mcp/issues/8))
- Repository-returning tools (`search_repositories`, `list_my_repositories`,
  `list_user_repositories`, `list_org_repositories`, `get_repository`) now
  include **clone URLs** (HTTPS and SSH) and the **default branch**. These are
  echoed verbatim from the API rather than reconstructed from the server base
  URL, so they remain correct on instances with a separate SSH host or a
  non-default SSH port.
  ([#8](https://github.com/denis-ev/forgejo-mcp/issues/8))

### Security

- Clone URLs are emitted without credentials. Tokens are never interpolated
  into rendered URLs, which would otherwise leak them into agent transcripts,
  shell history, and CI logs. A regression test enforces this.

## [0.6.1] - 2026-08-13

### Fixed

- **`list_action_tasks` and `list_action_runs`** now send `page=1` whenever a
  `limit` is given without an explicit page. Forgejo only applies `limit` when
  `page` is also present — `?limit=5` alone is served as an unpaginated
  listing and returns the entire history — so the 0.6.0 fix for
  [#4](https://github.com/denis-ev/forgejo-mcp/issues/4) still returned every
  task for the common `{limit: N}` call. Verified against Forgejo 15.0.3:
  `?limit=5` returned 691 tasks, `?limit=5&page=1` returned 5. The same
  latent bug was present in `list_action_runs` and is fixed alongside it.

## [0.6.0] - 2026-08-13

### Added

- Validation errors for unknown input fields now name the **accepted** fields
  and, where possible, suggest a specific replacement, instead of only naming
  the rejected one. Calling `merge_pull_request` with the REST API's `do`
  field previously failed with `unmarshaling: json: unknown field "do"`; it
  now reports `unknown field "do" for merge_pull_request — did you mean
  "style"?; expected one of: index, owner, repo, style, title`. Suggestions
  come from a table of known REST-API aliases plus a nearest-match fallback,
  and are only emitted when the suggested field actually exists on that tool.
  Applied uniformly to every tool via server middleware rather than per-tool
  code. ([#5](https://github.com/denis-ev/forgejo-mcp/issues/5))

### Fixed

- **`list_action_tasks`** now forwards `page` and `limit` to the Forgejo API.
  The tool advertised pagination in its schema but dropped both parameters when
  building the request, so every call returned the repository's full task list
  regardless of the requested limit. On repositories with hundreds of tasks the
  unbounded response could blow past client-side output limits and fail the
  call outright. `MyListActionTasks` now takes a `MyListActionTasksOptions`
  struct; omitting the fields preserves the previous behaviour of letting the
  server pick its default page size. The tool output also reports
  `Showing N of M action tasks` so a truncated page is no longer mistaken for
  the complete set. ([#4](https://github.com/denis-ev/forgejo-mcp/issues/4))

## [0.5.0] - 2026-07-22

### Added

- **`get_commit_status`** tool — read the combined CI/commit status for a
  branch, tag, or commit SHA, including each individual status context
  (build, lint, tests, etc.).
- **`create_commit_status`** tool — attach a `pending`/`success`/`error`/
  `failure` status to a commit SHA with an optional context label,
  description, and target URL, for external gating.

This release brings the fork roughly to parity with the equivalent GitHub
tooling for everyday repository/PR/CI workflows.

## [0.4.0] - 2026-07-22

### Added

- **`list_branches`** tool — list a repository's branches with protection
  status and each branch's tip commit, with pagination.
- **`create_branch`** tool — create a new branch, optionally from a specified
  source branch.
- **`list_tags`** tool — list a repository's git tags with the tagged commit
  and any message, with pagination.
- **`create_tag`** tool — create a new git tag, optionally targeting a specific
  commit/branch and including an annotation message.

## [0.3.0] - 2026-07-22

### Added

- **`get_pull_request_files`** tool — list the files changed by a pull request,
  with per-file status and addition/deletion counts, with pagination.
- **`get_pull_request_diff`** tool — fetch a pull request's raw unified diff
  (truncated for very large diffs, optional binary inclusion). Together these
  let a client actually review a PR's contents, not just its metadata.

## [0.2.0] - 2026-07-22

### Added

- **`get_file_contents`** tool — read a file's decoded contents, or list a
  directory's entries, at an optional ref (branch, tag, or commit SHA). Large
  files are truncated and binary files are detected and skipped rather than
  dumped. This closes the fork's biggest gap: previously the server could
  manage repository *metadata* but could not read a single line of source.
- **`list_commits`** tool — list commits with optional branch/SHA start point,
  path filter, and pagination.
- **`get_commit`** tool — view a single commit's metadata and stats, optionally
  including its raw unified diff (truncated for very large diffs).

## [0.1.0] - 2026-07-22

First independent fork release. Everything already merged from community PRs and
the earlier Actions/CI work is considered the `0.1.0` baseline; this release adds
pull request merging on top and introduces the fork's own release automation.

### Added

- **`merge_pull_request`** tool — merge a pull request using any of the
  `merge`, `rebase`, `rebase-merge`, or `squash` strategies, with optional
  custom merge-commit title/message, head-branch deletion after merge, and
  scheduling an auto-merge once required status checks succeed.
- **`is_pull_request_merged`** tool — read-only check of whether a pull request
  has already been merged.
- `CHANGELOG.md` (this file) and a `denis-ev`-owned semantic-versioning line.
- `.github/workflows/release.yml` — on every `v*` tag, runs the test suite,
  cross-compiles binaries for linux/darwin/windows (amd64 + arm64) with
  checksums, and publishes a GitHub Release whose notes are drawn from this
  changelog. (Multi-arch container images continue to publish to GHCR via the
  existing `docker-publish.yml`.)

### Baselined (already present before 0.1.0 tagging)

- Actions Runs API tools: `list_action_runs`, `get_action_run`,
  `list_action_run_jobs`, `get_action_job_logs`.
- Pull request review tools: `create_pull_request_review`,
  `list_pull_request_reviews`, `list_pull_request_review_comments`,
  `reply_to_review_comment`.
- `list_user_repositories`; wiki page title/slug 404 fallback; label IDs in
  markdown output; multi-user HTTP token-prefix fix; wiki pagination fix.
- CI (`ci.yml`) and GHCR publishing (`docker-publish.yml`).
