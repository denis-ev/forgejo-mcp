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
)

// ListActionRunJobsParams defines the parameters for the
// list_action_run_jobs tool.
type ListActionRunJobsParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// RunID is the numeric ID of the workflow run.
	RunID int64 `json:"run_id"`
}

// ListActionRunJobsImpl implements the read-only MCP tool for listing the
// jobs that belong to a single Forgejo Actions workflow run. The returned
// job_id is required by get_action_job_logs to fetch the raw execution log,
// since run IDs, job IDs and task IDs are all distinct numbering schemes in
// Forgejo's Actions implementation.
type ListActionRunJobsImpl struct {
	Client *tools.Client
}

func (ListActionRunJobsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_action_run_jobs",
		Title:       "List Action Run Jobs",
		Description: "List the jobs belonging to a specific Forgejo Actions workflow run. Use the returned job_id with get_action_job_logs to fetch execution logs.",
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
				"run_id": {
					Type:        "integer",
					Description: "Numeric ID of the workflow run (from list_action_runs)",
				},
			},
			Required: []string{"owner", "repo", "run_id"},
		},
	}
}

func (impl ListActionRunJobsImpl) Handler() mcp.ToolHandlerFor[ListActionRunJobsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListActionRunJobsParams) (*mcp.CallToolResult, any, error) {
		p := args

		jobs, err := impl.Client.MyListActionRunJobs(p.Owner, p.Repo, p.RunID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list action run jobs: %w", err)
		}

		var content string
		if len(jobs) == 0 {
			content = "No jobs found for this run."
		} else {
			content = fmt.Sprintf("Found %d jobs\n\n%s", len(jobs), jobs.ToMarkdown())
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: content},
			},
		}, nil, nil
	}
}
