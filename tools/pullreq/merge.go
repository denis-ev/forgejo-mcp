// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package pullreq

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// mergeStyles are the merge strategies merge_pull_request accepts. Which of
// them a repository allows is a repository setting (see edit_repository).
var mergeStyles = []any{"merge", "rebase", "rebase-merge", "squash", "fast-forward-only"}

// MergePullRequestParams defines the parameters for the merge_pull_request tool.
type MergePullRequestParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// Index is the pull request number.
	Index int `json:"index"`
	// Style is the merge method: merge, rebase, rebase-merge, squash or
	// fast-forward-only.
	Style string `json:"style,omitempty"`
	// Title overrides the merge commit title (optional).
	Title string `json:"title,omitempty"`
	// Message overrides the merge commit message body (optional).
	Message string `json:"message,omitempty"`
	// DeleteBranchAfterMerge deletes the head branch once merged (optional).
	DeleteBranchAfterMerge bool `json:"delete_branch_after_merge,omitempty"`
	// MergeWhenChecksSucceed schedules an auto-merge once required checks pass,
	// instead of merging immediately (optional).
	MergeWhenChecksSucceed bool `json:"merge_when_checks_succeed,omitempty"`
	// HeadCommitID makes Forgejo refuse the merge unless the PR head is still
	// this commit (optional).
	HeadCommitID string `json:"head_commit_id,omitempty"`
}

// MergePullRequestImpl implements the MCP tool for merging a pull request.
type MergePullRequestImpl struct {
	Client *tools.Client
}

// Definition describes the `merge_pull_request` tool.
func (MergePullRequestImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "merge_pull_request",
		Title:       "Merge Pull Request",
		Description: "Merge a pull request in a repository. Supports merge, rebase, rebase-merge, squash and fast-forward-only strategies, optional custom commit title/message, deleting the head branch after merge, a head-commit guard, and scheduling an auto-merge once required status checks succeed. When Forgejo refuses the merge (failing required checks, missing approvals, conflicts, a disallowed style) the result is an error carrying Forgejo's reason.",
		Annotations: &mcp.ToolAnnotations{
			// Merging is a state-changing, non-idempotent write.
			DestructiveHint: boolPtr(false),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {
					Type:        "string",
					Description: "Repository owner (username or organization name)",
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
				},
				"index": {
					Type:        "integer",
					Description: "Pull request index number",
				},
				"style": {
					Type:        "string",
					Description: "Merge strategy to use (optional, defaults to 'merge'); the repository must allow it",
					Enum:        mergeStyles,
				},
				"title": {
					Type:        "string",
					Description: "Custom merge commit title (optional)",
				},
				"message": {
					Type:        "string",
					Description: "Custom merge commit message body (optional)",
				},
				"delete_branch_after_merge": {
					Type:        "boolean",
					Description: "Delete the head branch after a successful merge (optional, defaults to false)",
				},
				"merge_when_checks_succeed": {
					Type:        "boolean",
					Description: "Schedule an automatic merge once required status checks succeed instead of merging immediately (optional, defaults to false). If the checks already pass, Forgejo merges right away",
				},
				"head_commit_id": {
					Type:        "string",
					Description: "Full SHA the PR head must still point at; Forgejo refuses the merge if new commits were pushed since (optional)",
				},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

// Handler merges through the merge endpoint and reports every outcome:
// merged, auto-merge scheduled, merged with the head branch kept, or refused
// with Forgejo's reason (as a tool error).
func (impl MergePullRequestImpl) Handler() mcp.ToolHandlerFor[MergePullRequestParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args MergePullRequestParams) (*mcp.CallToolResult, any, error) {
		p := args

		style := p.Style
		if style == "" {
			style = "merge"
		}

		res, err := impl.Client.MyMergePullRequest(p.Owner, p.Repo, int64(p.Index), tools.MergePullRequestBody{
			Do:                     style,
			MergeTitleField:        p.Title,
			MergeMessageField:      p.Message,
			DeleteBranchAfterMerge: p.DeleteBranchAfterMerge,
			HeadCommitID:           p.HeadCommitID,
			MergeWhenChecksSucceed: p.MergeWhenChecksSucceed,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to merge pull request: %w", err)
		}

		ref := fmt.Sprintf("%s/%s#%d", p.Owner, p.Repo, p.Index)
		switch res.Outcome {
		case tools.MergeOutcomeScheduled:
			return textResult(fmt.Sprintf("Auto-merge scheduled for pull request %s (style: %s); it will merge once required checks succeed. Use cancel_auto_merge to call it off.", ref, style)), nil, nil
		case tools.MergeOutcomeMerged:
			text := fmt.Sprintf("Pull request %s merged successfully (style: %s).", ref, style)
			if p.MergeWhenChecksSucceed {
				text = fmt.Sprintf("Pull request %s merged immediately (style: %s): the required checks had already succeeded, so no auto-merge was scheduled.", ref, style)
			}
			return textResult(text), nil, nil
		case tools.MergeOutcomeMergedBranchKept:
			return textResult(fmt.Sprintf("Pull request %s merged successfully (style: %s), but the head branch was not deleted: %s", ref, style, res.Reason)), nil, nil
		default:
			return errorResult(fmt.Sprintf("Pull request %s was NOT merged (HTTP %d): %s%s", ref, res.Status, res.Reason, mergeHint(res.Status, res.Reason))), nil, nil
		}
	}
}

// mergeHint adds a next step for the common refusals.
func mergeHint(status int, reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "status check"):
		return "\n\nCheck the PR's CI with get_ci_status, or pass merge_when_checks_succeed=true to merge once they pass."
	case strings.Contains(r, "approv"):
		return "\n\nThe protected base branch needs more approving reviews (create_pull_request_review with event APPROVED)."
	case strings.Contains(r, "not an allowed merge style"):
		return "\n\nPick a style the repository allows, or enable it with edit_repository."
	case strings.Contains(r, "try again later"):
		return "\n\nForgejo is still computing mergeability; retry shortly."
	case status == 409:
		return "\n\nThe PR conflicts with or is behind its base, or its head moved; update the branch and retry."
	}
	return ""
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// CancelAutoMergeParams defines the parameters for the cancel_auto_merge tool.
type CancelAutoMergeParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Index int    `json:"index"`
}

// CancelAutoMergeImpl implements the cancel_auto_merge tool.
type CancelAutoMergeImpl struct {
	Client *tools.Client
}

// Definition describes the `cancel_auto_merge` tool.
func (CancelAutoMergeImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "cancel_auto_merge",
		Title:       "Cancel Auto-Merge",
		Description: "Cancel the auto-merge scheduled on a pull request by merge_pull_request with merge_when_checks_succeed, so it no longer merges when its checks pass.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolPtr(false),
			IdempotentHint:  true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"index": {Type: "integer", Description: "Pull request index number"},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

// Handler cancels the scheduled auto-merge.
func (impl CancelAutoMergeImpl) Handler() mcp.ToolHandlerFor[CancelAutoMergeParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CancelAutoMergeParams) (*mcp.CallToolResult, any, error) {
		p := args
		if err := impl.Client.MyCancelAutoMerge(p.Owner, p.Repo, int64(p.Index)); err != nil {
			return nil, nil, fmt.Errorf("failed to cancel auto-merge: %w", err)
		}
		return textResult(fmt.Sprintf("Cancelled the scheduled auto-merge of pull request %s/%s#%d.", p.Owner, p.Repo, p.Index)), nil, nil
	}
}

// IsPullRequestMergedParams defines the parameters for the is_pull_request_merged tool.
type IsPullRequestMergedParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// Index is the pull request number.
	Index int `json:"index"`
}

// IsPullRequestMergedImpl implements the read-only MCP tool for checking merge state.
type IsPullRequestMergedImpl struct {
	Client *tools.Client
}

// Definition describes the `is_pull_request_merged` tool.
func (IsPullRequestMergedImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "is_pull_request_merged",
		Title:       "Is Pull Request Merged",
		Description: "Check whether a pull request has already been merged.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {
					Type:        "string",
					Description: "Repository owner (username or organization name)",
				},
				"repo": {
					Type:        "string",
					Description: "Repository name",
				},
				"index": {
					Type:        "integer",
					Description: "Pull request index number",
				},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

// Handler implements the merge-state check using the Forgejo SDK.
func (impl IsPullRequestMergedImpl) Handler() mcp.ToolHandlerFor[IsPullRequestMergedParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args IsPullRequestMergedParams) (*mcp.CallToolResult, any, error) {
		p := args

		merged, _, err := impl.Client.IsPullRequestMerged(p.Owner, p.Repo, int64(p.Index))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to check pull request merge state: %w", err)
		}

		var text string
		if merged {
			text = fmt.Sprintf("Pull request %s/%s#%d has been merged.", p.Owner, p.Repo, p.Index)
		} else {
			text = fmt.Sprintf("Pull request %s/%s#%d has not been merged.", p.Owner, p.Repo, p.Index)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: text},
			},
		}, nil, nil
	}
}

// boolPtr returns a pointer to the given bool, for optional ToolAnnotations fields.
func boolPtr(b bool) *bool { return &b }
