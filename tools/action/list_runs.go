// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package action

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
	"github.com/raohwork/forgejo-mcp/types"
)

// ListActionRunsParams defines the parameters for the list_action_runs tool.
type ListActionRunsParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// Page is the page number for pagination.
	Page int `json:"page,omitempty"`
	// Limit is the number of runs to return per page.
	Limit int `json:"limit,omitempty"`
	// Status filters runs by status (e.g. success, failure, running).
	Status string `json:"status,omitempty"`
}

// ListActionRunsImpl implements the read-only MCP tool for listing Forgejo
// Actions workflow runs. This uses the modern runs API (Forgejo v16+), which
// supersedes the legacy tasks endpoint and exposes the run IDs needed to
// drill into jobs and logs via get_action_run, list_action_run_jobs and
// get_action_job_logs.
type ListActionRunsImpl struct {
	Client *tools.Client
}

func (ListActionRunsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_action_runs",
		Title:       "List Action Runs",
		Description: "List Forgejo Actions workflow runs in a repository, with optional status filter and pagination. Use this to find a run_id for get_action_run or list_action_run_jobs.",
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
				"page": {
					Type:        "integer",
					Description: "Page number for pagination (optional, defaults to 1)",
					Minimum:     tools.Float64Ptr(1),
				},
				"limit": {
					Type:        "integer",
					Description: "Number of runs per page (optional, defaults to 20, max 50)",
					Minimum:     tools.Float64Ptr(1),
					Maximum:     tools.Float64Ptr(50),
				},
				"status": {
					Type:        "string",
					Description: "Filter by run status (optional): unknown, waiting, running, success, failure, cancelled, skipped, blocked",
					Enum: []any{
						"unknown", "waiting", "running", "success", "failure", "cancelled", "skipped", "blocked",
					},
				},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

func (impl ListActionRunsImpl) Handler() mcp.ToolHandlerFor[ListActionRunsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListActionRunsParams) (*mcp.CallToolResult, any, error) {
		p := args

		opt := tools.MyListActionRunsOptions{
			Page:   p.Page,
			Limit:  p.Limit,
			Status: p.Status,
		}

		resp, err := impl.Client.MyListActionRuns(p.Owner, p.Repo, opt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list action runs: %w", err)
		}

		var content string
		if resp.TotalCount == 0 || len(resp.Entries) == 0 {
			content = "No action runs found in this repository."
		} else {
			list := types.ActionRunList{MyActionRunListResponse: resp}
			content = fmt.Sprintf("Found %d action runs\n\n%s", resp.TotalCount, list.ToMarkdown())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: content},
			},
		}, nil, nil
	}
}
