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

// maxCommitDiffChars bounds how much of a commit diff is returned inline.
const maxCommitDiffChars = 20000

// ListCommitsParams defines the parameters for the list_commits tool.
type ListCommitsParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// SHA is an optional branch name or commit SHA to start listing from.
	SHA string `json:"sha,omitempty"`
	// Path, if set, limits results to commits touching that file/dir.
	Path string `json:"path,omitempty"`
	// Page is the 1-based page number.
	Page int `json:"page,omitempty"`
	// Limit is the page size (max 50).
	Limit int `json:"limit,omitempty"`
}

// ListCommitsImpl implements the read-only list_commits tool.
type ListCommitsImpl struct {
	Client *tools.Client
}

// Definition describes the `list_commits` tool.
func (ListCommitsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_commits",
		Title:       "List Commits",
		Description: "List commits in a repository, optionally starting from a branch/SHA and/or filtered to commits touching a given path, with pagination.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"sha":   {Type: "string", Description: "Branch name or commit SHA to start listing from (optional, defaults to the default branch)"},
				"path":  {Type: "string", Description: "Only list commits that touch this file or directory path (optional)"},
				"page":  {Type: "integer", Description: "Page number for pagination (optional, defaults to 1)", Minimum: ptrFloat(1)},
				"limit": {Type: "integer", Description: "Number of commits per page (optional, defaults to 20, max 50)", Minimum: ptrFloat(1), Maximum: ptrFloat(50)},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// Handler lists commits via the SDK's ListRepoCommits.
func (impl ListCommitsImpl) Handler() mcp.ToolHandlerFor[ListCommitsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListCommitsParams) (*mcp.CallToolResult, any, error) {
		p := args
		opt := forgejo.ListCommitOptions{
			SHA:  p.SHA,
			Path: p.Path,
			// Keep responses lean: skip per-commit file lists and verification.
			Stat:         false,
			Files:        false,
			Verification: false,
		}
		if p.Page > 0 {
			opt.ListOptions.Page = p.Page
		}
		if p.Limit > 0 {
			opt.ListOptions.PageSize = p.Limit
		}

		commits, _, err := impl.Client.ListRepoCommits(p.Owner, p.Repo, opt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list commits: %w", err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Found %d commits\n\n", len(commits))
		for i, c := range commits {
			fmt.Fprintf(&b, "%d. %s\n", i+1, commitOneLine(c))
		}
		return textResult(b.String()), nil, nil
	}
}

// GetCommitParams defines the parameters for the get_commit tool.
type GetCommitParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// SHA is the commit SHA (or branch/tag ref that resolves to a commit).
	SHA string `json:"sha"`
	// IncludeDiff appends the raw unified diff when true.
	IncludeDiff bool `json:"include_diff,omitempty"`
}

// GetCommitImpl implements the read-only get_commit tool.
type GetCommitImpl struct {
	Client *tools.Client
}

// Definition describes the `get_commit` tool.
func (GetCommitImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_commit",
		Title:       "Get Commit",
		Description: "Get a single commit's metadata (author, message, stats), optionally including its raw unified diff (truncated for large diffs).",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":        {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":         {Type: "string", Description: "Repository name"},
				"sha":          {Type: "string", Description: "Commit SHA (or a branch/tag ref that resolves to a commit)"},
				"include_diff": {Type: "boolean", Description: "Include the commit's raw unified diff (optional, defaults to false)"},
			},
			Required: []string{"owner", "repo", "sha"},
		},
	}
}

// Handler fetches a single commit and, optionally, its diff.
func (impl GetCommitImpl) Handler() mcp.ToolHandlerFor[GetCommitParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetCommitParams) (*mcp.CallToolResult, any, error) {
		p := args

		commit, _, err := impl.Client.GetSingleCommit(p.Owner, p.Repo, p.SHA)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get commit: %w", err)
		}

		var b strings.Builder
		b.WriteString(commitDetail(commit))

		if p.IncludeDiff {
			diff, _, derr := impl.Client.GetCommitDiff(p.Owner, p.Repo, p.SHA)
			if derr != nil {
				fmt.Fprintf(&b, "\n\n*(could not fetch diff: %v)*", derr)
			} else {
				d := string(diff)
				truncated := false
				if len(d) > maxCommitDiffChars {
					d = d[len(d)-maxCommitDiffChars:]
					truncated = true
				}
				b.WriteString("\n\n")
				if truncated {
					b.WriteString("*(diff truncated, showing last 20000 characters)*\n\n")
				}
				b.WriteString("```diff\n")
				b.WriteString(d)
				if !strings.HasSuffix(d, "\n") {
					b.WriteString("\n")
				}
				b.WriteString("```")
			}
		}

		return textResult(b.String()), nil, nil
	}
}

// commitOneLine renders a compact one-line summary of a commit.
func commitOneLine(c *forgejo.Commit) string {
	sha := ""
	if c.CommitMeta != nil {
		sha = shortSHA(c.CommitMeta.SHA)
	}
	msg := ""
	if c.RepoCommit != nil {
		msg = firstLine(c.RepoCommit.Message)
	}
	author := "?"
	if c.RepoCommit != nil && c.RepoCommit.Author != nil && c.RepoCommit.Author.Name != "" {
		author = c.RepoCommit.Author.Name
	} else if c.Author != nil && c.Author.UserName != "" {
		author = c.Author.UserName
	}
	return fmt.Sprintf("`%s` %s — %s", sha, msg, author)
}

// commitDetail renders a full metadata view of a commit.
func commitDetail(c *forgejo.Commit) string {
	var b strings.Builder
	sha := ""
	if c.CommitMeta != nil {
		sha = c.CommitMeta.SHA
	}
	fmt.Fprintf(&b, "Commit %s\n", sha)

	if c.RepoCommit != nil {
		if c.RepoCommit.Author != nil {
			fmt.Fprintf(&b, "Author: %s <%s> at %s\n", c.RepoCommit.Author.Name, c.RepoCommit.Author.Email, c.RepoCommit.Author.Date)
		}
		if c.RepoCommit.Committer != nil {
			fmt.Fprintf(&b, "Committer: %s <%s> at %s\n", c.RepoCommit.Committer.Name, c.RepoCommit.Committer.Email, c.RepoCommit.Committer.Date)
		}
	}
	if c.Stats != nil {
		fmt.Fprintf(&b, "Stats: +%d -%d (total %d)\n", c.Stats.Additions, c.Stats.Deletions, c.Stats.Total)
	}
	if len(c.Parents) > 0 {
		parents := make([]string, 0, len(c.Parents))
		for _, pm := range c.Parents {
			parents = append(parents, shortSHA(pm.SHA))
		}
		fmt.Fprintf(&b, "Parents: %s\n", strings.Join(parents, ", "))
	}
	if c.RepoCommit != nil {
		fmt.Fprintf(&b, "\n%s", c.RepoCommit.Message)
	}
	return strings.TrimRight(b.String(), "\n")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func ptrFloat(f float64) *float64 { return &f }
