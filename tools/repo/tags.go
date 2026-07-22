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

// ListTagsParams defines the parameters for the list_tags tool.
type ListTagsParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Page  int    `json:"page,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// ListTagsImpl implements the read-only list_tags tool.
type ListTagsImpl struct {
	Client *tools.Client
}

// Definition describes the `list_tags` tool.
func (ListTagsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_tags",
		Title:       "List Tags",
		Description: "List a repository's git tags, including the tagged commit and any tag message, with pagination.",
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
				"limit": {Type: "integer", Description: "Number of tags per page (optional, defaults to 20, max 50)", Minimum: ptrFloat(1), Maximum: ptrFloat(50)},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// Handler lists tags via the SDK.
func (impl ListTagsImpl) Handler() mcp.ToolHandlerFor[ListTagsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListTagsParams) (*mcp.CallToolResult, any, error) {
		p := args
		opt := forgejo.ListRepoTagsOptions{}
		if p.Page > 0 {
			opt.ListOptions.Page = p.Page
		}
		if p.Limit > 0 {
			opt.ListOptions.PageSize = p.Limit
		}

		tags, _, err := impl.Client.ListRepoTags(p.Owner, p.Repo, opt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list tags: %w", err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d tags\n\n", len(tags))
		for i, tg := range tags {
			commit := ""
			if tg.Commit != nil {
				commit = shortSHA(tg.Commit.SHA)
			}
			line := fmt.Sprintf("%d. %s (commit: `%s`)", i+1, tg.Name, commit)
			if msg := firstLine(strings.TrimSpace(tg.Message)); msg != "" {
				line += " — " + msg
			}
			b.WriteString(line + "\n")
		}
		return textResult(b.String()), nil, nil
	}
}

// CreateTagParams defines the parameters for the create_tag tool.
type CreateTagParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// Tag is the tag name to create.
	Tag string `json:"tag"`
	// Target is an optional commit SHA or branch to tag (defaults to the
	// repository default branch).
	Target string `json:"target,omitempty"`
	// Message is an optional annotation message; when set an annotated tag is created.
	Message string `json:"message,omitempty"`
}

// CreateTagImpl implements the create_tag tool.
type CreateTagImpl struct {
	Client *tools.Client
}

// Definition describes the `create_tag` tool.
func (CreateTagImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "create_tag",
		Title:       "Create Tag",
		Description: "Create a new git tag in a repository, optionally targeting a specific commit/branch and including an annotation message.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: boolFalse(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":   {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":    {Type: "string", Description: "Repository name"},
				"tag":     {Type: "string", Description: "Tag name to create"},
				"target":  {Type: "string", Description: "Commit SHA or branch to tag (optional, defaults to the repository default branch)"},
				"message": {Type: "string", Description: "Annotation message; when set, an annotated tag is created (optional)"},
			},
			Required: []string{"owner", "repo", "tag"},
		},
	}
}

// Handler creates a tag via the SDK.
func (impl CreateTagImpl) Handler() mcp.ToolHandlerFor[CreateTagParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CreateTagParams) (*mcp.CallToolResult, any, error) {
		p := args
		tg, _, err := impl.Client.CreateTag(p.Owner, p.Repo, forgejo.CreateTagOption{
			TagName: p.Tag,
			Target:  p.Target,
			Message: p.Message,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tag: %w", err)
		}

		commit := ""
		if tg != nil && tg.Commit != nil {
			commit = shortSHA(tg.Commit.SHA)
		}
		name := p.Tag
		if tg != nil && tg.Name != "" {
			name = tg.Name
		}
		return textResult(fmt.Sprintf("Created tag %q in %s/%s (commit: `%s`).", name, p.Owner, p.Repo, commit)), nil, nil
	}
}
