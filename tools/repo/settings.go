// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// EditRepositoryParams defines the parameters for the edit_repository tool.
type EditRepositoryParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	tools.EditRepositoryOptions
}

// EditRepositoryImpl implements the edit_repository tool.
type EditRepositoryImpl struct {
	Client *tools.Client
}

// Definition describes the `edit_repository` tool.
func (EditRepositoryImpl) Definition() *mcp.Tool {
	boolean := func(desc string) *jsonschema.Schema { return &jsonschema.Schema{Type: "boolean", Description: desc} }
	str := func(desc string) *jsonschema.Schema { return &jsonschema.Schema{Type: "string", Description: desc} }
	return &mcp.Tool{
		Name:  "edit_repository",
		Title: "Edit Repository Settings",
		Description: "Change repository settings: default branch, visibility, description, enabled units (issues, wiki, PRs, ...), " +
			"allowed merge styles and pull request defaults. Options you leave out keep their current value. Requires repository admin access.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":                             str("Repository owner (username or organization name)"),
				"repo":                              str("Repository name"),
				"description":                       str("Short description of the repository"),
				"website":                           str("URL with more information about the repository"),
				"private":                           boolean("true makes the repository private, false public"),
				"template":                          boolean("Mark the repository as a template"),
				"archived":                          boolean("Archive (true) or unarchive (false) the repository; an archived repository is read-only"),
				"default_branch":                    str("Default branch; new pull requests target it by default"),
				"has_issues":                        boolean("Enable the issue tracker"),
				"has_wiki":                          boolean("Enable the wiki"),
				"has_pull_requests":                 boolean("Enable pull requests"),
				"has_projects":                      boolean("Enable projects"),
				"has_releases":                      boolean("Enable releases"),
				"has_packages":                      boolean("Enable packages"),
				"has_actions":                       boolean("Enable Forgejo Actions"),
				"allow_merge_commits":               boolean("Allow the 'merge' merge style"),
				"allow_rebase":                      boolean("Allow the 'rebase' merge style"),
				"allow_rebase_explicit":             boolean("Allow the 'rebase-merge' merge style (rebase, then merge commit)"),
				"allow_squash_merge":                boolean("Allow the 'squash' merge style"),
				"allow_fast_forward_only_merge":     boolean("Allow the 'fast-forward-only' merge style"),
				"allow_rebase_update":               boolean("Allow updating a PR branch by rebase"),
				"default_merge_style":               {Type: "string", Description: "Merge style preselected in the UI", Enum: []any{"merge", "rebase", "rebase-merge", "squash", "fast-forward-only"}},
				"default_delete_branch_after_merge": boolean("Delete the head branch after merge by default"),
				"default_allow_maintainer_edit":     boolean("Allow maintainers to edit new pull requests by default"),
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// Handler patches the repository settings.
func (impl EditRepositoryImpl) Handler() mcp.ToolHandlerFor[EditRepositoryParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args EditRepositoryParams) (*mcp.CallToolResult, any, error) {
		if args.EditRepositoryOptions == (tools.EditRepositoryOptions{}) {
			return nil, nil, fmt.Errorf("no settings given; pass at least one option to change")
		}
		r, err := impl.Client.MyEditRepository(args.Owner, args.Repo, args.EditRepositoryOptions)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to edit repository: %w", err)
		}

		var styles []string
		for _, s := range []struct {
			on   bool
			name string
		}{
			{r.AllowMerge, "merge"},
			{r.AllowRebase, "rebase"},
			{r.AllowRebaseMerge, "rebase-merge"},
			{r.AllowSquash, "squash"},
		} {
			if s.on {
				styles = append(styles, s.name)
			}
		}
		visibility := "public"
		if r.Private {
			visibility = "private"
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Updated %s.\n\n", r.FullName)
		fmt.Fprintf(&b, "- Default branch: %s\n", r.DefaultBranch)
		fmt.Fprintf(&b, "- Visibility: %s", visibility)
		if r.Archived {
			b.WriteString(", archived")
		}
		b.WriteString("\n")
		if len(styles) > 0 {
			fmt.Fprintf(&b, "- Allowed merge styles: %s\n", strings.Join(styles, ", "))
		}
		if r.DefaultMergeStyle != "" {
			fmt.Fprintf(&b, "- Default merge style: %s\n", r.DefaultMergeStyle)
		}
		return textResult(strings.TrimRight(b.String(), "\n")), nil, nil
	}
}
