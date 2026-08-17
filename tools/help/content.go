// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package help

// This file holds the curated half of the help output: the domain groupings
// and the narrative sections that cannot be derived from tool schemas.
//
// The tool inventory itself is generated (see help.go), so adding a tool never
// requires touching this file. Adding a whole new tool *package* does: give it
// a domain entry here, otherwise its tools are listed under "Other".

// domain describes one grouping of tools in the help output.
type domain struct {
	// key is the implementation package name, matching
	// tools.RegisteredTool.Domain.
	key string
	// topic is the value a caller passes as help's topic parameter.
	topic string
	// title is the human-readable heading.
	title string
	// summary is a one-paragraph description of what the domain covers.
	summary string
	// notes holds domain-specific gotchas, shown only on the drill-down.
	notes string
}

// domainOrder lists the domains in the order they appear in the overview,
// most commonly used first.
var domainOrder = []domain{
	{
		key:     "issue",
		topic:   "issues",
		title:   "Issues",
		summary: "Create, read, edit and comment on issues; manage issue labels, attachments and dependencies.",
		notes: "Issues are addressed by `index`, the per-repository issue number shown in the UI — not a database ID.\n\n" +
			"Label operations on issues take label **IDs**, not names. Resolve names with `list_repo_labels` first. " +
			"By contrast `list_repo_issues` filters by label **names**. See the ID conventions section of the overview.\n\n" +
			"Pull requests are issues in Forgejo's data model, so `create_issue_comment` also comments on a pull request " +
			"when given the PR's `index`.\n",
	},
	{
		key:     "pullreq",
		topic:   "pull_requests",
		title:   "Pull requests",
		summary: "List, inspect, create, review and merge pull requests, including file lists and raw diffs.",
		notes: "Pull requests use `index` (the per-repository number) like issues, except `get_ci_status`, which names the " +
			"same value `pull_number` because it also accepts a plain `ref`.\n\n" +
			"To leave review feedback use `create_pull_request_review`; `reply_to_review_comment` answers an existing review " +
			"comment by its `comment_id`. A general, non-review comment is `create_issue_comment` with the PR index.\n",
	},
	{
		key:     "repo",
		topic:   "repository",
		title:   "Repository",
		summary: "Search and inspect repositories, browse file contents, commits, branches and tags, and read or write commit statuses.",
		notes: "`get_ci_status` lives here rather than under actions because it answers the cross-cutting question " +
			"\"is this ref green?\" by merging commit statuses with Actions jobs.\n\n" +
			"Repository writing is limited: branches and tags can be created, but file contents cannot yet be written " +
			"(see the known gaps section of the overview).\n",
	},
	{
		key:     "label",
		topic:   "labels",
		title:   "Labels",
		summary: "Repository-level label management: list, create, edit and delete label definitions.",
		notes: "Labels are addressed by numeric `id`. `list_repo_labels` is the only way to map a label name to its ID, and " +
			"is a prerequisite for `add_issue_labels` and `replace_issue_labels`.\n",
	},
	{
		key:     "milestone",
		topic:   "milestones",
		title:   "Milestones",
		summary: "Repository milestone management: list, create, edit and delete milestones.",
		notes:   "Milestones are addressed by numeric `id`, not by title.\n",
	},
	{
		key:     "release",
		topic:   "releases",
		title:   "Releases",
		summary: "Manage releases and their attachments.",
		notes: "Releases are addressed by numeric `id`; attachments by `attachment_id`. Uploading a new attachment is not " +
			"supported yet — existing attachments can be listed, renamed and deleted.\n",
	},
	{
		key:     "action",
		topic:   "actions",
		title:   "Actions (CI/CD)",
		summary: "Inspect Forgejo Actions: workflow runs, the jobs inside a run, and raw job logs.",
		notes: "The drill-down path is `list_action_runs` → `list_action_run_jobs` (needs `run_id`) → " +
			"`get_action_job_logs` (needs `job_id`). Both IDs are database IDs returned by the previous call; " +
			"neither is a run \"number\".\n\n" +
			"`get_action_job_logs` requires a Forgejo version exposing the job logs endpoint (verified on v16.0.1+); " +
			"older servers return 404.\n\n" +
			"`list_action_tasks` is the older task-level view; prefer the runs API for anything new.\n",
	},
	{
		key:     "wiki",
		topic:   "wiki",
		title:   "Wiki",
		summary: "Read, create, edit, delete and list wiki pages.",
		notes:   "Wiki pages are addressed by page name; the server falls back to the slug when a title lookup misses.\n",
	},
	{
		key:     "help",
		topic:   "help",
		title:   "Help",
		summary: "This tool. Describes the server without touching the Forgejo API.",
	},
}

// domainByKey indexes domainOrder by implementation package name.
var domainByKey = func() map[string]domain {
	out := make(map[string]domain, len(domainOrder))
	for _, d := range domainOrder {
		out[d.key] = d
	}
	return out
}()

// domainByTopic indexes domainOrder by the topic string callers pass in. It
// accepts a few obvious aliases so a reasonable guess ("pr", "issue", "ci")
// resolves instead of failing.
var domainByTopic = func() map[string]domain {
	out := make(map[string]domain, len(domainOrder)*2)
	for _, d := range domainOrder {
		out[d.topic] = d
		out[d.key] = d
	}

	aliases := map[string]string{
		"issue":        "issues",
		"pull_request": "pull_requests",
		"pullrequest":  "pull_requests",
		"pullrequests": "pull_requests",
		"pr":           "pull_requests",
		"prs":          "pull_requests",
		"review":       "pull_requests",
		"reviews":      "pull_requests",
		"repo":         "repository",
		"repositories": "repository",
		"label":        "labels",
		"milestone":    "milestones",
		"release":      "releases",
		"action":       "actions",
		"ci":           "actions",
		"cd":           "actions",
		"workflow":     "actions",
		"workflows":    "actions",
		"comment":      "issues",
		"comments":     "issues",
		"attachment":   "issues",
		"attachments":  "issues",
		"dependency":   "issues",
		"dependencies": "issues",
		"branch":       "repository",
		"branches":     "repository",
		"tag":          "repository",
		"tags":         "repository",
		"commit":       "repository",
		"commits":      "repository",
		"file":         "repository",
		"files":        "repository",
		"status":       "repository",
	}
	for alias, target := range aliases {
		if d, ok := out[target]; ok {
			out[alias] = d
		}
	}

	return out
}()

// lookupDomain resolves a normalized topic to a domain.
func lookupDomain(topic string) (domain, bool) {
	d, ok := domainByTopic[topic]
	return d, ok
}

// domainTopics lists the canonical topic strings, in overview order.
func domainTopics() []string {
	out := make([]string, 0, len(domainOrder))
	for _, d := range domainOrder {
		out = append(out, d.topic)
	}
	return out
}

// idConventions explains how this server names identifiers. Getting this wrong
// is the single most common source of failed calls, so it leads the curated
// sections.
const idConventions = "## ID conventions\n\n" +
	"- `index` — the per-repository **issue or pull request number**, the one shown in the web UI. Never a database ID.\n" +
	"- `pull_number` — same value as `index`, used only by `get_ci_status`, which also accepts a `ref`.\n" +
	"- `id` — a **database ID**, used for labels, milestones and releases. Obtain it from the matching `list_*` tool.\n" +
	"- `comment_id`, `release_id`, `attachment_id`, `run_id`, `job_id` — database IDs, likewise returned by a list call.\n" +
	"- `ref` / `sha` — a branch name, tag or commit SHA.\n\n" +
	"### The label gotcha\n\n" +
	"Label tools mix names and IDs, deliberately following the underlying Forgejo API:\n\n" +
	"- `add_issue_labels` and `replace_issue_labels` take label **IDs** (`labels`: integer array)\n" +
	"- `remove_issue_label` takes a single label **ID** (`label`)\n" +
	"- `list_repo_issues` filters by label **names** (`labels`: comma-separated string)\n\n" +
	"So labelling an issue by name is always two calls: `list_repo_labels` to map names to IDs, then `add_issue_labels`.\n\n"

// workflows lists the multi-tool sequences that solve the common tasks. These
// exist because the individual tool descriptions cannot express ordering.
const workflows = "## Common workflows\n\n" +
	"### Triage a CI failure\n\n" +
	"1. `get_ci_status` with `pull_number` (or `ref`) — one rolled-up answer; failing Actions jobs come back with their `run_id` and `job_id`\n" +
	"2. `get_action_job_logs` with the failing `job_id` — raw log including the stack trace\n" +
	"3. `create_issue_comment` with the PR `index` — report the finding\n\n" +
	"If `get_ci_status` is unavailable, the long way is `list_action_runs` → `list_action_run_jobs` → `get_action_job_logs`.\n\n" +
	"### Review a pull request\n\n" +
	"1. `list_pull_requests` — find the `index`\n" +
	"2. `get_pull_request` — metadata, and `get_pull_request_files` or `get_pull_request_diff` for the changes\n" +
	"3. `create_pull_request_review` — submit the review (approve, request changes, or comment)\n" +
	"4. `reply_to_review_comment` — answer an existing review comment by `comment_id`\n\n" +
	"### Triage an issue\n\n" +
	"1. `list_repo_issues` — filter by state, label names, assignee or milestone\n" +
	"2. `list_repo_labels` — map the label names you want to their IDs\n" +
	"3. `add_issue_labels` — apply them by ID\n" +
	"4. `edit_issue` — set milestone, assignees or state\n\n" +
	"### Land a change\n\n" +
	"1. `create_branch` — branch off the base\n" +
	"2. Push commits with git (this server cannot write file contents yet)\n" +
	"3. `create_pull_request` — open the PR\n" +
	"4. `get_ci_status` — wait for green\n" +
	"5. `merge_pull_request` — the merge method parameter is `style`, not `do` or `merge_method`\n\n"

// paginationNotes documents the pagination convention shared by every list
// tool.
const paginationNotes = "## Pagination\n\n" +
	"List tools take `page` (1-based, defaults to 1) and `limit` (defaults to 20, capped at 50). There is no cursor and no " +
	"next-page token: request page 2 when a full page comes back. The REST names `per_page` and `page_size` are rejected — " +
	"the parameter is `limit`.\n\n" +
	"An unknown parameter name is rejected before the call reaches the handler; the error names the accepted fields and, " +
	"where possible, suggests the right one.\n\n"

// versionGates records the tools that depend on a recent Forgejo release, so
// an agent can tell a capability gap from a bug.
const versionGates = "## Version-gated tools\n\n" +
	"This server is developed against Forgejo v16.0.1. Against older servers:\n\n" +
	"- `get_action_job_logs` — needs the Actions job logs endpoint (verified on v16.0.1+); older servers return **404**\n" +
	"- `list_action_runs`, `get_action_run`, `list_action_run_jobs` — need the modern Actions runs API (Forgejo v16+)\n" +
	"- `get_ci_status` — degrades gracefully: without the runs API it still reports commit statuses\n\n" +
	"A 404 from these tools means the server is too old, not that the run or job is missing.\n\n"

// knownGaps names the operations this server does not implement, so an agent
// stops looking instead of guessing at tool names.
const knownGaps = "## Known gaps\n\n" +
	"Not implemented — do not go looking for a tool:\n\n" +
	"- Writing repository file contents (create/update/delete files)\n" +
	"- Editing an existing pull request (title, body, base branch)\n" +
	"- Uploading new issue or release attachments (listing, renaming and deleting work)\n" +
	"- Deleting branches, and branch protection\n" +
	"- Dispatching, re-running or cancelling Actions runs\n" +
	"- Repository creation, forking and settings; webhooks, collaborators and topics\n\n"

// toolNotes holds per-tool warnings that belong in help rather than in the
// tool description itself, either because they reference another tool or
// because they are only relevant once a caller is already using the tool.
var toolNotes = map[string]string{
	"add_issue_labels": "`labels` is an array of label **IDs**, not names. Call `list_repo_labels` first to translate.",
	"replace_issue_labels": "`labels` is an array of label **IDs**, not names, and replaces the full set. " +
		"Pass an empty array to clear all labels.",
	"remove_issue_label": "`label` is the label **ID**, not its name.",
	"list_repo_issues": "Filters by label **names** here, unlike the issue label tools which use IDs. " +
		"Returns pull requests as well, since Forgejo models them as issues.",
	"merge_pull_request": "The merge method parameter is `style`. The REST spellings `do` and `merge_method` are rejected.",
	"get_action_job_logs": "Requires Forgejo v16.0.1 or newer; older servers return 404. " +
		"`job_id` comes from `list_action_run_jobs` or from a failing check reported by `get_ci_status`.",
	"list_action_run_jobs": "`run_id` is the database ID from `list_action_runs`, not a run number.",
	"get_ci_status": "Prefer this over merging `get_commit_status` and the Actions jobs by hand. " +
		"Supply either `pull_number` or `ref`; `pull_number` wins when both are given.",
	"create_issue_comment": "Also comments on a pull request: pass the PR's `index`. " +
		"For review feedback use `create_pull_request_review` instead.",
}
