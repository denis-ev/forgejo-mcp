// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package pullreq

import (
	"context"
	"fmt"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// MergePullRequestParams defines the parameters for the merge_pull_request tool.
type MergePullRequestParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// Index is the pull request number.
	Index int `json:"index"`
	// Style is the merge method: merge, rebase, rebase-merge, or squash.
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
		Description: "Merge a pull request in a repository. Supports merge, rebase, rebase-merge, and squash strategies, optional custom commit title/message, deleting the head branch after merge, and scheduling an auto-merge once required status checks succeed.",
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
					Description: "Merge strategy to use (optional, defaults to 'merge')",
					Enum:        []any{"merge", "rebase", "rebase-merge", "squash"},
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
					Description: "Schedule an automatic merge once required status checks succeed instead of merging immediately (optional, defaults to false)",
				},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

// Handler implements the merge logic using the Forgejo SDK's MergePullRequest.
func (impl MergePullRequestImpl) Handler() mcp.ToolHandlerFor[MergePullRequestParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args MergePullRequestParams) (*mcp.CallToolResult, any, error) {
		p := args

		style := forgejo.MergeStyle(p.Style)
		if style == "" {
			style = forgejo.MergeStyleMerge
		}

		opt := forgejo.MergePullRequestOption{
			Style:                  style,
			Title:                  p.Title,
			Message:                p.Message,
			DeleteBranchAfterMerge: p.DeleteBranchAfterMerge,
			MergeWhenChecksSucceed: p.MergeWhenChecksSucceed,
		}

		merged, _, err := impl.Client.MergePullRequest(p.Owner, p.Repo, int64(p.Index), opt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to merge pull request: %w", err)
		}

		var text string
		switch {
		case merged && p.MergeWhenChecksSucceed:
			text = fmt.Sprintf("Auto-merge scheduled for pull request %s/%s#%d (style: %s); it will merge once required checks succeed.", p.Owner, p.Repo, p.Index, style)
		case merged:
			text = fmt.Sprintf("Pull request %s/%s#%d merged successfully (style: %s).", p.Owner, p.Repo, p.Index, style)
		default:
			text = fmt.Sprintf("Pull request %s/%s#%d was not merged. It may not be in a mergeable state (conflicts, failing required checks, or missing approvals).", p.Owner, p.Repo, p.Index)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: text},
			},
		}, nil, nil
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
