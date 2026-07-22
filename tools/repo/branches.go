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

// ListBranchesParams defines the parameters for the list_branches tool.
type ListBranchesParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Page  int    `json:"page,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// ListBranchesImpl implements the read-only list_branches tool.
type ListBranchesImpl struct {
	Client *tools.Client
}

// Definition describes the `list_branches` tool.
func (ListBranchesImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_branches",
		Title:       "List Branches",
		Description: "List a repository's branches, including protection status and the tip commit of each, with pagination.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"page":  {Type: "integer", Description: "Page number for pagination (optional, defaults to 1)", Minimum: ptrFloat(1)},
				"limit": {Type: "integer", Description: "Number of branches per page (optional, defaults to 20, max 50)", Minimum: ptrFloat(1), Maximum: ptrFloat(50)},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// Handler lists branches via the SDK.
func (impl ListBranchesImpl) Handler() mcp.ToolHandlerFor[ListBranchesParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListBranchesParams) (*mcp.CallToolResult, any, error) {
		p := args
		opt := forgejo.ListRepoBranchesOptions{}
		if p.Page > 0 {
			opt.ListOptions.Page = p.Page
		}
		if p.Limit > 0 {
			opt.ListOptions.PageSize = p.Limit
		}

		branches, _, err := impl.Client.ListRepoBranches(p.Owner, p.Repo, opt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list branches: %w", err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d branches\n\n", len(branches))
		for i, br := range branches {
			tip := ""
			if br.Commit != nil {
				tip = shortSHA(br.Commit.ID)
			}
			protected := ""
			if br.Protected {
				protected = " [protected]"
			}
			fmt.Fprintf(&b, "%d. %s (tip: `%s`)%s\n", i+1, br.Name, tip, protected)
		}
		return textResult(b.String()), nil, nil
	}
}

// CreateBranchParams defines the parameters for the create_branch tool.
type CreateBranchParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// Branch is the name of the new branch to create.
	Branch string `json:"branch"`
	// OldBranch is the source branch to create from (optional, defaults to the
	// repository default branch).
	OldBranch string `json:"old_branch,omitempty"`
}

// CreateBranchImpl implements the create_branch tool.
type CreateBranchImpl struct {
	Client *tools.Client
}

// Definition describes the `create_branch` tool.
func (CreateBranchImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "create_branch",
		Title:       "Create Branch",
		Description: "Create a new branch in a repository, optionally from a specified source branch (defaults to the repository's default branch).",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolFalse(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":      {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":       {Type: "string", Description: "Repository name"},
				"branch":     {Type: "string", Description: "Name of the new branch to create"},
				"old_branch": {Type: "string", Description: "Source branch to create from (optional, defaults to the repository default branch)"},
			},
			Required: []string{"owner", "repo", "branch"},
		},
	}
}

// Handler creates a branch via the SDK.
func (impl CreateBranchImpl) Handler() mcp.ToolHandlerFor[CreateBranchParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CreateBranchParams) (*mcp.CallToolResult, any, error) {
		p := args
		br, _, err := impl.Client.CreateBranch(p.Owner, p.Repo, forgejo.CreateBranchOption{
			BranchName:    p.Branch,
			OldBranchName: p.OldBranch,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create branch: %w", err)
		}

		tip := ""
		if br != nil && br.Commit != nil {
			tip = shortSHA(br.Commit.ID)
		}
		name := p.Branch
		if br != nil && br.Name != "" {
			name = br.Name
		}
		return textResult(fmt.Sprintf("Created branch %q in %s/%s (tip: `%s`).", name, p.Owner, p.Repo, tip)), nil, nil
	}
}

func boolFalse() *bool { b := false; return &b }
