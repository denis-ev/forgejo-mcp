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

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// GetCommitStatusParams defines the parameters for the get_commit_status tool.
type GetCommitStatusParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// Ref is a branch, tag, or commit SHA.
	Ref string `json:"ref"`
}

// GetCommitStatusImpl implements the read-only get_commit_status tool.
type GetCommitStatusImpl struct {
	Client *tools.Client
}

// Definition describes the `get_commit_status` tool.
func (GetCommitStatusImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_commit_status",
		Title:       "Get Commit Status",
		Description: "Get the combined CI/commit status for a branch, tag, or commit SHA, including each individual status context (e.g. build, lint, tests).",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"ref":   {Type: "string", Description: "Branch name, tag, or commit SHA to read the status of"},
			},
			Required: []string{"owner", "repo", "ref"},
		},
	}
}

// Handler fetches the combined commit status via the SDK.
func (impl GetCommitStatusImpl) Handler() mcp.ToolHandlerFor[GetCommitStatusParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCommitStatusParams) (*mcp.CallToolResult, any, error) {
		p := args

		combined, _, err := impl.Client.GetCombinedStatus(p.Owner, p.Repo, p.Ref)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get commit status: %w", err)
		}

		var b strings.Builder
		state := string(combined.State)
		if state == "" {
			state = "(none)"
		}
		fmt.Fprintf(&b, "Combined status for %s/%s@%s: %s (%d contexts)\n",
			p.Owner, p.Repo, shortSHA(combined.SHA), state, combined.TotalCount)

		if len(combined.Statuses) == 0 {
			b.WriteString("\nNo individual status contexts reported.")
			return textResult(b.String()), nil, nil
		}
		b.WriteString("\n")
		for i, s := range combined.Statuses {
			line := fmt.Sprintf("%d. [%s] %s", i+1, s.State, s.Context)
			if s.Description != "" {
				line += " — " + s.Description
			}
			b.WriteString(line + "\n")
		}
		return textResult(strings.TrimRight(b.String(), "\n")), nil, nil
	}
}

// CreateCommitStatusParams defines the parameters for the create_commit_status tool.
type CreateCommitStatusParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// SHA is the commit SHA to attach the status to.
	SHA string `json:"sha"`
	// State is one of: pending, success, error, failure.
	State string `json:"state"`
	// Context is the unique label for this status (e.g. "ci/build").
	Context string `json:"context,omitempty"`
	// Description is a short human-readable summary.
	Description string `json:"description,omitempty"`
	// TargetURL links to the full status/build details.
	TargetURL string `json:"target_url,omitempty"`
}

// CreateCommitStatusImpl implements the create_commit_status tool.
type CreateCommitStatusImpl struct {
	Client *tools.Client
}

// Definition describes the `create_commit_status` tool.
func (CreateCommitStatusImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "create_commit_status",
		Title:       "Create Commit Status",
		Description: "Attach a CI/commit status (pending, success, error, or failure) to a commit SHA, with an optional context label, description, and target URL. Useful for external gating.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolFalse(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":       {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":        {Type: "string", Description: "Repository name"},
				"sha":         {Type: "string", Description: "Commit SHA to attach the status to"},
				"state":       {Type: "string", Description: "Status state", Enum: []any{"pending", "success", "error", "failure"}},
				"context":     {Type: "string", Description: "Unique context label for this status, e.g. 'ci/build' (optional)"},
				"description": {Type: "string", Description: "Short human-readable status description (optional)"},
				"target_url":  {Type: "string", Description: "URL linking to the full status/build details (optional)"},
			},
			Required: []string{"owner", "repo", "sha", "state"},
		},
	}
}

// Handler creates a commit status via the SDK.
func (impl CreateCommitStatusImpl) Handler() mcp.ToolHandlerFor[CreateCommitStatusParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CreateCommitStatusParams) (*mcp.CallToolResult, any, error) {
		p := args

		s, _, err := impl.Client.CreateStatus(p.Owner, p.Repo, p.SHA, forgejo.CreateStatusOption{
			State:       forgejo.StatusState(p.State),
			Context:     p.Context,
			Description: p.Description,
			TargetURL:   p.TargetURL,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create commit status: %w", err)
		}

		ctxLabel := p.Context
		if s != nil && s.Context != "" {
			ctxLabel = s.Context
		}
		return textResult(fmt.Sprintf("Created %s status on %s/%s@%s (context: %q).",
			p.State, p.Owner, p.Repo, shortSHA(p.SHA), ctxLabel)), nil, nil
	}
}
