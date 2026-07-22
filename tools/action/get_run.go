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

// GetActionRunParams defines the parameters for the get_action_run tool.
type GetActionRunParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// RunID is the numeric ID of the workflow run.
	RunID int64 `json:"run_id"`
}

// GetActionRunImpl implements the read-only MCP tool for fetching a single
// Forgejo Actions workflow run by ID.
type GetActionRunImpl struct {
	Client *tools.Client
}

func (GetActionRunImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_action_run",
		Title:       "Get Action Run",
		Description: "Get detailed information about a single Forgejo Actions workflow run, including status, timing and commit ref.",
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

func (impl GetActionRunImpl) Handler() mcp.ToolHandlerFor[GetActionRunParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetActionRunParams) (*mcp.CallToolResult, any, error) {
		p := args

		run, err := impl.Client.MyGetActionRun(p.Owner, p.Repo, p.RunID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get action run: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: run.ToMarkdown()},
			},
		}, nil, nil
	}
}
