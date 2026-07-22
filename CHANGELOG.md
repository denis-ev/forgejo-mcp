# Changelog

All notable changes to this fork (`denis-ev/forgejo-mcp`) are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Fork note:** Upstream (`raohwork/forgejo-mcp`) stopped at `v0.0.7` and has
> shown no maintainer activity for an extended period. This fork continues the
> line from `v0.1.0` onward, targeting current Forgejo (verified against
> **v16.0.1**). See the "About This Fork" section in the README for details.

## [Unreleased]

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
