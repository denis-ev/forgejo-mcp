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

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// maxPullRequestDiffChars bounds how much of a PR diff is returned inline.
const maxPullRequestDiffChars = 20000

// GetPullRequestFilesParams defines the parameters for the get_pull_request_files tool.
type GetPullRequestFilesParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Index int    `json:"index"`
	Page  int    `json:"page,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// GetPullRequestFilesImpl implements the read-only get_pull_request_files tool.
type GetPullRequestFilesImpl struct {
	Client *tools.Client
}

// Definition describes the `get_pull_request_files` tool.
func (GetPullRequestFilesImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_pull_request_files",
		Title:       "Get Pull Request Files",
		Description: "List the files changed by a pull request, with per-file status and addition/deletion counts, with pagination.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"index": {Type: "integer", Description: "Pull request index number"},
				"page":  {Type: "integer", Description: "Page number for pagination (optional, defaults to 1)", Minimum: pfloat(1)},
				"limit": {Type: "integer", Description: "Number of files per page (optional, defaults to 20, max 50)", Minimum: pfloat(1), Maximum: pfloat(50)},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

// Handler lists a PR's changed files via the SDK.
func (impl GetPullRequestFilesImpl) Handler() mcp.ToolHandlerFor[GetPullRequestFilesParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetPullRequestFilesParams) (*mcp.CallToolResult, any, error) {
		p := args
		opt := forgejo.ListPullRequestFilesOptions{}
		if p.Page > 0 {
			opt.ListOptions.Page = p.Page
		}
		if p.Limit > 0 {
			opt.ListOptions.PageSize = p.Limit
		}

		files, _, err := impl.Client.ListPullRequestFiles(p.Owner, p.Repo, int64(p.Index), opt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list pull request files: %w", err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d changed files in %s/%s#%d\n\n", len(files), p.Owner, p.Repo, p.Index)
		for i, f := range files {
			name := f.Filename
			if f.PreviousFilename != "" && f.PreviousFilename != f.Filename {
				name = fmt.Sprintf("%s (renamed from %s)", f.Filename, f.PreviousFilename)
			}
			fmt.Fprintf(&b, "%d. [%s] %s (+%d -%d)\n", i+1, f.Status, name, f.Additions, f.Deletions)
		}
		return prTextResult(b.String()), nil, nil
	}
}

// GetPullRequestDiffParams defines the parameters for the get_pull_request_diff tool.
type GetPullRequestDiffParams struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Index  int    `json:"index"`
	Binary bool   `json:"binary,omitempty"`
}

// GetPullRequestDiffImpl implements the read-only get_pull_request_diff tool.
type GetPullRequestDiffImpl struct {
	Client *tools.Client
}

// Definition describes the `get_pull_request_diff` tool.
func (GetPullRequestDiffImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_pull_request_diff",
		Title:       "Get Pull Request Diff",
		Description: "Get the raw unified diff of a pull request (truncated for very large diffs).",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":  {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":   {Type: "string", Description: "Repository name"},
				"index":  {Type: "integer", Description: "Pull request index number"},
				"binary": {Type: "boolean", Description: "Include binary file changes in the diff (optional, defaults to false)"},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

// Handler fetches a PR's raw diff via the SDK.
func (impl GetPullRequestDiffImpl) Handler() mcp.ToolHandlerFor[GetPullRequestDiffParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetPullRequestDiffParams) (*mcp.CallToolResult, any, error) {
		p := args

		raw, _, err := impl.Client.GetPullRequestDiff(p.Owner, p.Repo, int64(p.Index), forgejo.PullRequestDiffOptions{Binary: p.Binary})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get pull request diff: %w", err)
		}

		d := string(raw)
		truncated := false
		if len(d) > maxPullRequestDiffChars {
			d = d[:maxPullRequestDiffChars]
			truncated = true
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Diff for %s/%s#%d:\n\n", p.Owner, p.Repo, p.Index)
		if truncated {
			b.WriteString("*(diff truncated, showing first 20000 characters)*\n\n")
		}
		b.WriteString("```diff\n")
		b.WriteString(d)
		if !strings.HasSuffix(d, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```")
		return prTextResult(b.String()), nil, nil
	}
}

func prTextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func pfloat(f float64) *float64 { return &f }
