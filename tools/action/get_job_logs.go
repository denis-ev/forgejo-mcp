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

// GetActionJobLogsParams defines the parameters for the get_action_job_logs
// tool.
type GetActionJobLogsParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// JobID is the numeric ID of the job (from list_action_run_jobs).
	JobID int64 `json:"job_id"`
}

// GetActionJobLogsImpl implements the read-only MCP tool for fetching the
// raw plaintext execution log of a single Forgejo Actions job. This exposes
// a capability added in newer Forgejo releases (verified on v16.0.1): full
// step-by-step logs, including failure stack traces, are retrievable over
// the REST API. On older Forgejo/Gitea versions this endpoint does not
// exist and the call will fail with a 404.
type GetActionJobLogsImpl struct {
	Client *tools.Client
}

func (GetActionJobLogsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_action_job_logs",
		Title:       "Get Action Job Logs",
		Description: "Fetch the raw plaintext execution log for a Forgejo Actions job, including failure output. Requires a Forgejo version that supports the job logs endpoint (verified on v16.0.1+); older servers will return a 404.",
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
				"job_id": {
					Type:        "integer",
					Description: "Numeric ID of the job (from list_action_run_jobs)",
				},
			},
			Required: []string{"owner", "repo", "job_id"},
		},
	}
}

func (impl GetActionJobLogsImpl) Handler() mcp.ToolHandlerFor[GetActionJobLogsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetActionJobLogsParams) (*mcp.CallToolResult, any, error) {
		p := args

		text, err := impl.Client.MyGetActionJobLogs(p.Owner, p.Repo, p.JobID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get action job logs: %w", err)
		}

		logs := types.ActionJobLogs{JobID: p.JobID, Text: text}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: logs.ToMarkdown()},
			},
		}, nil, nil
	}
}
