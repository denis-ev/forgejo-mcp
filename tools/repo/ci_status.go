// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
	"github.com/raohwork/forgejo-mcp/types"
)

// maxRunScanPages bounds how many pages of Actions runs are scanned while
// looking for runs attached to the resolved commit. Forgejo's runs endpoint
// cannot filter by commit SHA, so the runs have to be scanned client-side.
const maxRunScanPages = 3

// runScanPageSize is the page size used when scanning Actions runs.
const runScanPageSize = 50

// Normalized CI states. These are deliberately the same vocabulary used by
// commit statuses so that commit statuses and Actions jobs can be rolled up
// into a single answer.
const (
	ciStateSuccess = "success"
	ciStateFailure = "failure"
	ciStatePending = "pending"
	ciStateNeutral = "neutral"
	ciStateNone    = "none"
)

// GetCIStatusParams defines the parameters for the get_ci_status tool.
// Exactly one of Ref or PullNumber must be supplied.
type GetCIStatusParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// Ref is a branch, tag, or commit SHA. Ignored when PullNumber is set.
	Ref string `json:"ref,omitempty"`
	// PullNumber is a pull request index; its head commit is used.
	PullNumber int64 `json:"pull_number,omitempty"`
}

// GetCIStatusImpl implements the read-only get_ci_status tool. It merges the
// combined commit status and the Forgejo Actions jobs attached to the same
// commit into one rolled-up answer, so an agent can ask "is this green?" in a
// single call instead of correlating two separate APIs by hand.
type GetCIStatusImpl struct {
	Client *tools.Client
}

// ciCheck is one normalized check, sourced from either a commit status or an
// Actions job.
type ciCheck struct {
	// Source is "status" or "action".
	Source string
	// Name is the status context or the job name.
	Name string
	// RawState is the state as reported by Forgejo.
	RawState string
	// State is RawState normalized to the ciState* vocabulary.
	State string
	// Detail is an optional human-readable description.
	Detail string
	// URL links to the full build/status details, when available.
	URL string
	// RunID and JobID are set for Actions checks and are directly consumable
	// by get_action_job_logs.
	RunID int64
	JobID int64
}

// normalizeCIState maps both commit-status states and Actions job statuses
// onto a single vocabulary.
//
// Commit statuses use: pending, success, error, failure, warning.
// Actions jobs use: waiting, running, blocked, success, failure, cancelled,
// skipped, unknown.
func normalizeCIState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "success":
		return ciStateSuccess
	case "failure", "error", "cancelled", "canceled":
		return ciStateFailure
	case "pending", "running", "waiting", "blocked":
		return ciStatePending
	case "skipped", "warning":
		return ciStateNeutral
	case "":
		return ciStateNone
	default:
		return ciStateNeutral
	}
}

// rollupCIState reduces individual check states to one answer. Failure wins
// over pending, pending wins over success. Neutral states (skipped, warning)
// never mask a real result.
func rollupCIState(checks []ciCheck) string {
	sawSuccess := false
	sawPending := false
	for _, c := range checks {
		switch c.State {
		case ciStateFailure:
			return ciStateFailure
		case ciStatePending:
			sawPending = true
		case ciStateSuccess:
			sawSuccess = true
		}
	}
	switch {
	case sawPending:
		return ciStatePending
	case sawSuccess:
		return ciStateSuccess
	default:
		return ciStateNone
	}
}

// Definition describes the `get_ci_status` tool.
func (GetCIStatusImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:  "get_ci_status",
		Title: "Get CI Status",
		Description: "Get the rolled-up CI state for a pull request, branch, tag, or commit in one call. " +
			"Merges commit statuses and Forgejo Actions jobs into a single check list, and reports the run/job IDs " +
			"of failing Actions jobs so their logs can be fetched with get_action_job_logs.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"ref": {
					Type:        "string",
					Description: "Branch name, tag, or commit SHA to inspect. Ignored when pull_number is given.",
				},
				"pull_number": {
					Type:        "integer",
					Description: "Pull request number; its head commit is inspected. Takes precedence over ref.",
					Minimum:     tools.Float64Ptr(1),
				},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// resolveCIRef resolves the request to a single commit SHA.
func (impl GetCIStatusImpl) resolveCIRef(p GetCIStatusParams) (sha, origin string, err error) {
	switch {
	case p.PullNumber > 0:
		pr, _, err := impl.Client.GetPullRequest(p.Owner, p.Repo, p.PullNumber)
		if err != nil {
			return "", "", fmt.Errorf("failed to get pull request #%d: %w", p.PullNumber, err)
		}
		if pr == nil || pr.Head == nil || pr.Head.Sha == "" {
			return "", "", fmt.Errorf("pull request #%d has no resolvable head commit", p.PullNumber)
		}
		return pr.Head.Sha, fmt.Sprintf("PR #%d", p.PullNumber), nil
	case p.Ref != "":
		c, _, err := impl.Client.GetSingleCommit(p.Owner, p.Repo, p.Ref)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve ref %q: %w", p.Ref, err)
		}
		if c == nil || c.SHA == "" {
			return "", "", fmt.Errorf("ref %q did not resolve to a commit", p.Ref)
		}
		return c.SHA, p.Ref, nil
	default:
		return "", "", fmt.Errorf("either ref or pull_number must be provided")
	}
}

// collectActionChecks finds Actions runs attached to sha and expands them into
// per-job checks. A failure to read Actions (for example on an instance with
// Actions disabled) is not fatal: commit statuses alone still give a useful
// answer, so the error is reported as a note instead.
func (impl GetCIStatusImpl) collectActionChecks(owner, repoName, sha string) (checks []ciCheck, note string) {
	var runs []*types.MyActionRun
	for page := 1; page <= maxRunScanPages; page++ {
		resp, err := impl.Client.MyListActionRuns(owner, repoName, tools.MyListActionRunsOptions{
			Page:  page,
			Limit: runScanPageSize,
		})
		if err != nil {
			if len(runs) == 0 {
				return nil, fmt.Sprintf("Actions runs could not be read (%v); showing commit statuses only.", err)
			}
			break
		}
		if resp == nil || len(resp.Entries) == 0 {
			break
		}
		for _, r := range resp.Entries {
			if r != nil && r.CommitSHA == sha {
				runs = append(runs, r)
			}
		}
		if len(resp.Entries) < runScanPageSize {
			break
		}
	}

	for _, r := range runs {
		jobs, err := impl.Client.MyListActionRunJobs(owner, repoName, r.ID)
		if err != nil {
			// Fall back to run-level state when jobs cannot be listed.
			checks = append(checks, ciCheck{
				Source:   "action",
				Name:     r.Title,
				RawState: r.Status,
				State:    normalizeCIState(r.Status),
				Detail:   fmt.Sprintf("run-level state; jobs unavailable (%v)", err),
				URL:      r.HTMLURL,
				RunID:    r.ID,
			})
			continue
		}
		if len(jobs) == 0 {
			checks = append(checks, ciCheck{
				Source:   "action",
				Name:     r.Title,
				RawState: r.Status,
				State:    normalizeCIState(r.Status),
				Detail:   "run reported no jobs",
				URL:      r.HTMLURL,
				RunID:    r.ID,
			})
			continue
		}
		for _, j := range jobs {
			if j == nil {
				continue
			}
			name := j.Name
			if r.WorkflowID != "" {
				name = fmt.Sprintf("%s / %s", r.WorkflowID, j.Name)
			}
			checks = append(checks, ciCheck{
				Source:   "action",
				Name:     name,
				RawState: j.Status,
				State:    normalizeCIState(j.Status),
				URL:      r.HTMLURL,
				RunID:    r.ID,
				JobID:    j.ID,
			})
		}
	}
	return checks, ""
}

// Handler implements the rolled-up CI status lookup.
func (impl GetCIStatusImpl) Handler() mcp.ToolHandlerFor[GetCIStatusParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCIStatusParams) (*mcp.CallToolResult, any, error) {
		p := args

		sha, origin, err := impl.resolveCIRef(p)
		if err != nil {
			return nil, nil, err
		}

		var checks []ciCheck

		combined, _, err := impl.Client.GetCombinedStatus(p.Owner, p.Repo, sha)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get commit status: %w", err)
		}
		if combined != nil {
			for _, s := range combined.Statuses {
				if s == nil {
					continue
				}
				checks = append(checks, ciCheck{
					Source:   "status",
					Name:     s.Context,
					RawState: string(s.State),
					State:    normalizeCIState(string(s.State)),
					Detail:   s.Description,
					URL:      s.TargetURL,
				})
			}
		}

		actionChecks, note := impl.collectActionChecks(p.Owner, p.Repo, sha)
		checks = append(checks, actionChecks...)

		state := rollupCIState(checks)

		var b strings.Builder
		fmt.Fprintf(&b, "CI status for %s/%s @ %s (%s): %s\n", p.Owner, p.Repo, shortSHA(sha), origin, strings.ToUpper(state))
		fmt.Fprintf(&b, "%d check(s) total\n", len(checks))
		if note != "" {
			fmt.Fprintf(&b, "\nNote: %s\n", note)
		}

		if len(checks) == 0 {
			b.WriteString("\nNo commit statuses and no Actions jobs are attached to this commit.")
			return textResult(b.String()), nil, nil
		}

		b.WriteString("\n")
		for i, c := range checks {
			line := fmt.Sprintf("%d. [%s] %s", i+1, c.RawState, c.Name)
			if c.Detail != "" {
				line += " — " + c.Detail
			}
			b.WriteString(line + "\n")
			if c.JobID > 0 {
				fmt.Fprintf(&b, "   run_id: %d | job_id: %d\n", c.RunID, c.JobID)
			} else if c.RunID > 0 {
				fmt.Fprintf(&b, "   run_id: %d\n", c.RunID)
			}
			if c.URL != "" {
				fmt.Fprintf(&b, "   %s\n", c.URL)
			}
		}

		// Surface the drill-down path for failures explicitly: this is the
		// whole point of merging the two sources.
		var failedJobs []string
		for _, c := range checks {
			if c.State == ciStateFailure && c.JobID > 0 {
				failedJobs = append(failedJobs, fmt.Sprintf("job_id %d (%s)", c.JobID, c.Name))
			}
		}
		if len(failedJobs) > 0 {
			fmt.Fprintf(&b, "\nFailing Actions jobs — fetch logs with get_action_job_logs: %s",
				strings.Join(failedJobs, ", "))
		}

		return textResult(strings.TrimRight(b.String(), "\n")), nil, nil
	}
}
