// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package repo

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
	"github.com/raohwork/forgejo-mcp/types"
)

// Content crossing the MCP boundary is plain UTF-8 text: every write tool takes
// the file body as-is and base64-encodes it before calling the API, because an
// agent holds text, not base64. Binary content is out of scope for these tools.

// writeCommonProps returns the schema properties shared by every file-writing
// tool. Building them in one place keeps the four tools from drifting in
// wording, which is what an agent reads to decide how to call them.
func writeCommonProps() map[string]*jsonschema.Schema {
	return map[string]*jsonschema.Schema{
		"owner":   {Type: "string", Description: "Repository owner (username or organization name)"},
		"repo":    {Type: "string", Description: "Repository name"},
		"message": {Type: "string", Description: "Commit message describing the change"},
		"branch": {
			Type:        "string",
			Description: "Branch to commit to (optional, defaults to the repository default branch)",
		},
		"new_branch": {
			Type:        "string",
			Description: "Create this branch from `branch` and commit there instead (optional); use it to prepare a pull request without touching the base branch",
		},
		"author_name":     {Type: "string", Description: "Commit author name (optional, defaults to the authenticated user)"},
		"author_email":    {Type: "string", Description: "Commit author email (optional, defaults to the authenticated user)"},
		"committer_name":  {Type: "string", Description: "Committer name (optional, defaults to the author, then the authenticated user)"},
		"committer_email": {Type: "string", Description: "Committer email (optional, defaults to the author, then the authenticated user)"},
	}
}

// withProps merges tool-specific properties into the shared ones.
func withProps(extra map[string]*jsonschema.Schema) map[string]*jsonschema.Schema {
	props := writeCommonProps()
	for k, v := range extra {
		props[k] = v
	}
	return props
}

// fileOptions builds the SDK's shared file options from the flat parameters the
// tools expose. Author and committer are flattened into separate name/email
// fields because a nested object is awkward for a tool caller to fill in.
func fileOptions(message, branch, newBranch, authorName, authorEmail, committerName, committerEmail string) forgejo.FileOptions {
	return forgejo.FileOptions{
		Message:       message,
		BranchName:    branch,
		NewBranchName: newBranch,
		Author:        forgejo.Identity{Name: authorName, Email: authorEmail},
		Committer:     forgejo.Identity{Name: committerName, Email: committerEmail},
	}
}

// identityOrNil returns an identity only when at least one field is set, so an
// empty author is omitted from the request instead of being sent as blank.
func identityOrNil(name, email string) *types.MyIdentity {
	if name == "" && email == "" {
		return nil
	}
	return &types.MyIdentity{Name: name, Email: email}
}

// writeErrorHint turns the common Forgejo write failures into an actionable
// next step. The API reports them as bare 4xx bodies, which an agent cannot act
// on without knowing the convention.
func writeErrorHint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "sha does not match"), strings.Contains(msg, "sha mismatch"):
		return " — the blob sha is stale; re-read the file with get_file_contents and retry with the sha it reports"
	case strings.Contains(msg, "already exists"):
		return " — a file already exists at this path; use update_file with its sha instead"
	case strings.Contains(msg, "protected"):
		return " — the target branch is protected; commit to a new branch via new_branch and open a pull request"
	case strings.Contains(msg, "404"), strings.Contains(msg, "not found"):
		return " — check owner/repo, the branch, and that the path exists on that branch"
	}
	return ""
}

// requirePath rejects an empty path before a request is issued, since the API
// answers a missing path with a confusing 404 on the repository itself.
func requirePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required and must not be empty")
	}
	return nil
}

// requireSHA rejects an empty blob sha before a request is issued.
func requireSHA(sha, tool string) error {
	if strings.TrimSpace(sha) == "" {
		return fmt.Errorf("sha is required by %s: read the file with get_file_contents first and pass the sha it reports", tool)
	}
	return nil
}

// renderCommitFooter appends the commit identity of a successful write.
func renderCommitFooter(b *strings.Builder, commit *forgejo.FileCommitResponse) {
	if commit == nil {
		return
	}
	if commit.SHA != "" {
		fmt.Fprintf(b, "- Commit: `%s`\n", commit.SHA)
	}
	if commit.HTMLURL != "" {
		fmt.Fprintf(b, "- URL: %s\n", commit.HTMLURL)
	}
}

// renderTarget names the branch a write landed on.
func renderTarget(branch, newBranch string) string {
	switch {
	case newBranch != "":
		return fmt.Sprintf(" on new branch `%s`", newBranch)
	case branch != "":
		return fmt.Sprintf(" on branch `%s`", branch)
	}
	return " on the default branch"
}

// CreateFileParams defines the parameters for the create_file tool.
type CreateFileParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// Path is the new file's path within the repository.
	Path string `json:"path"`
	// Content is the file body as plain text; it is base64-encoded internally.
	Content string `json:"content"`
	// Message is the commit message.
	Message string `json:"message"`
	// Branch is the branch to commit to (optional).
	Branch string `json:"branch,omitempty"`
	// NewBranch creates a branch from Branch and commits there (optional).
	NewBranch      string `json:"new_branch,omitempty"`
	AuthorName     string `json:"author_name,omitempty"`
	AuthorEmail    string `json:"author_email,omitempty"`
	CommitterName  string `json:"committer_name,omitempty"`
	CommitterEmail string `json:"committer_email,omitempty"`
}

// CreateFileImpl implements the create_file tool.
type CreateFileImpl struct {
	Client *tools.Client
}

// Definition describes the `create_file` tool.
func (CreateFileImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:  "create_file",
		Title: "Create File",
		Description: "Create a new file in a repository and commit it. Content is plain text and is encoded for you. " +
			"Fails if the path already exists — use update_file for that.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolFalse(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: withProps(map[string]*jsonschema.Schema{
				"path":    {Type: "string", Description: "Path of the new file within the repository, e.g. `docs/setup.md`"},
				"content": {Type: "string", Description: "File content as plain text (not base64; the server encodes it)"},
			}),
			Required: []string{"owner", "repo", "path", "content", "message"},
		},
	}
}

// Handler creates a file via the SDK.
func (impl CreateFileImpl) Handler() mcp.ToolHandlerFor[CreateFileParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CreateFileParams) (*mcp.CallToolResult, any, error) {
		p := args
		if err := requirePath(p.Path); err != nil {
			return nil, nil, err
		}

		resp, _, err := impl.Client.CreateFile(p.Owner, p.Repo, p.Path, forgejo.CreateFileOptions{
			FileOptions: fileOptions(p.Message, p.Branch, p.NewBranch, p.AuthorName, p.AuthorEmail, p.CommitterName, p.CommitterEmail),
			Content:     base64.StdEncoding.EncodeToString([]byte(p.Content)),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create file %q: %w%s", p.Path, err, writeErrorHint(err))
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Created `%s` in %s/%s%s.\n\n", p.Path, p.Owner, p.Repo, renderTarget(p.Branch, p.NewBranch))
		if resp != nil {
			renderCommitFooter(&b, resp.Commit)
			if resp.Content != nil && resp.Content.SHA != "" {
				fmt.Fprintf(&b, "- File sha (pass to update_file/delete_file): `%s`\n", resp.Content.SHA)
			}
		}
		return textResult(b.String()), nil, nil
	}
}

// UpdateFileParams defines the parameters for the update_file tool.
type UpdateFileParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Path  string `json:"path"`
	// Content is the full new file body as plain text; it replaces the file.
	Content string `json:"content"`
	// SHA is the blob sha of the file being replaced.
	SHA string `json:"sha"`
	// FromPath renames/moves an existing file to Path (optional).
	FromPath       string `json:"from_path,omitempty"`
	Message        string `json:"message"`
	Branch         string `json:"branch,omitempty"`
	NewBranch      string `json:"new_branch,omitempty"`
	AuthorName     string `json:"author_name,omitempty"`
	AuthorEmail    string `json:"author_email,omitempty"`
	CommitterName  string `json:"committer_name,omitempty"`
	CommitterEmail string `json:"committer_email,omitempty"`
}

// UpdateFileImpl implements the update_file tool.
type UpdateFileImpl struct {
	Client *tools.Client
}

// Definition describes the `update_file` tool.
func (UpdateFileImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:  "update_file",
		Title: "Update File",
		Description: "Replace an existing file's contents and commit the change. Content is plain text and replaces the " +
			"whole file. Requires the current blob sha, which get_file_contents reports.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolFalse(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: withProps(map[string]*jsonschema.Schema{
				"path":    {Type: "string", Description: "Path of the file to update within the repository"},
				"content": {Type: "string", Description: "Full new file content as plain text (not base64; it replaces the whole file)"},
				"sha": {
					Type:        "string",
					Description: "Blob sha of the file being replaced, as reported by get_file_contents; a stale sha is rejected",
				},
				"from_path": {
					Type:        "string",
					Description: "Original path of a file to move or rename to `path` (optional)",
				},
			}),
			Required: []string{"owner", "repo", "path", "content", "sha", "message"},
		},
	}
}

// Handler updates a file via the SDK.
func (impl UpdateFileImpl) Handler() mcp.ToolHandlerFor[UpdateFileParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args UpdateFileParams) (*mcp.CallToolResult, any, error) {
		p := args
		if err := requirePath(p.Path); err != nil {
			return nil, nil, err
		}
		if err := requireSHA(p.SHA, "update_file"); err != nil {
			return nil, nil, err
		}

		resp, _, err := impl.Client.UpdateFile(p.Owner, p.Repo, p.Path, forgejo.UpdateFileOptions{
			FileOptions: fileOptions(p.Message, p.Branch, p.NewBranch, p.AuthorName, p.AuthorEmail, p.CommitterName, p.CommitterEmail),
			SHA:         p.SHA,
			Content:     base64.StdEncoding.EncodeToString([]byte(p.Content)),
			FromPath:    p.FromPath,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update file %q: %w%s", p.Path, err, writeErrorHint(err))
		}

		var b strings.Builder
		if p.FromPath != "" {
			fmt.Fprintf(&b, "Updated `%s` (moved from `%s`) in %s/%s%s.\n\n", p.Path, p.FromPath, p.Owner, p.Repo, renderTarget(p.Branch, p.NewBranch))
		} else {
			fmt.Fprintf(&b, "Updated `%s` in %s/%s%s.\n\n", p.Path, p.Owner, p.Repo, renderTarget(p.Branch, p.NewBranch))
		}
		if resp != nil {
			renderCommitFooter(&b, resp.Commit)
			if resp.Content != nil && resp.Content.SHA != "" {
				fmt.Fprintf(&b, "- New file sha (pass to the next update_file/delete_file): `%s`\n", resp.Content.SHA)
			}
		}
		return textResult(b.String()), nil, nil
	}
}

// DeleteFileParams defines the parameters for the delete_file tool.
type DeleteFileParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Path  string `json:"path"`
	// SHA is the blob sha of the file being deleted.
	SHA            string `json:"sha"`
	Message        string `json:"message"`
	Branch         string `json:"branch,omitempty"`
	NewBranch      string `json:"new_branch,omitempty"`
	AuthorName     string `json:"author_name,omitempty"`
	AuthorEmail    string `json:"author_email,omitempty"`
	CommitterName  string `json:"committer_name,omitempty"`
	CommitterEmail string `json:"committer_email,omitempty"`
}

// DeleteFileImpl implements the delete_file tool.
type DeleteFileImpl struct {
	Client *tools.Client
}

// Definition describes the `delete_file` tool.
func (DeleteFileImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:  "delete_file",
		Title: "Delete File",
		Description: "Delete a file from a repository and commit the removal. Requires the current blob sha, which " +
			"get_file_contents reports.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolTrue(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: withProps(map[string]*jsonschema.Schema{
				"path": {Type: "string", Description: "Path of the file to delete within the repository"},
				"sha": {
					Type:        "string",
					Description: "Blob sha of the file being deleted, as reported by get_file_contents; a stale sha is rejected",
				},
			}),
			Required: []string{"owner", "repo", "path", "sha", "message"},
		},
	}
}

// Handler deletes a file via the SDK.
func (impl DeleteFileImpl) Handler() mcp.ToolHandlerFor[DeleteFileParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args DeleteFileParams) (*mcp.CallToolResult, any, error) {
		p := args
		if err := requirePath(p.Path); err != nil {
			return nil, nil, err
		}
		if err := requireSHA(p.SHA, "delete_file"); err != nil {
			return nil, nil, err
		}

		_, err := impl.Client.DeleteFile(p.Owner, p.Repo, p.Path, forgejo.DeleteFileOptions{
			FileOptions: fileOptions(p.Message, p.Branch, p.NewBranch, p.AuthorName, p.AuthorEmail, p.CommitterName, p.CommitterEmail),
			SHA:         p.SHA,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to delete file %q: %w%s", p.Path, err, writeErrorHint(err))
		}

		return textResult(fmt.Sprintf("Deleted `%s` from %s/%s%s.", p.Path, p.Owner, p.Repo, renderTarget(p.Branch, p.NewBranch))), nil, nil
	}
}

// PushFileParams is one entry of the push_files file list.
type PushFileParams struct {
	// Operation is "create", "update" or "delete".
	Operation string `json:"operation"`
	Path      string `json:"path"`
	// Content is the plain-text body, required for create and update.
	Content string `json:"content,omitempty"`
	// SHA is the current blob sha, required for update and delete.
	SHA string `json:"sha,omitempty"`
	// FromPath moves an existing file to Path (optional, update only).
	FromPath string `json:"from_path,omitempty"`
}

// PushFilesParams defines the parameters for the push_files tool.
type PushFilesParams struct {
	Owner          string           `json:"owner"`
	Repo           string           `json:"repo"`
	Files          []PushFileParams `json:"files"`
	Message        string           `json:"message"`
	Branch         string           `json:"branch,omitempty"`
	NewBranch      string           `json:"new_branch,omitempty"`
	AuthorName     string           `json:"author_name,omitempty"`
	AuthorEmail    string           `json:"author_email,omitempty"`
	CommitterName  string           `json:"committer_name,omitempty"`
	CommitterEmail string           `json:"committer_email,omitempty"`
}

// PushFilesImpl implements the push_files tool.
type PushFilesImpl struct {
	Client *tools.Client
}

// Definition describes the `push_files` tool.
func (PushFilesImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:  "push_files",
		Title: "Push Files",
		Description: "Create, update and delete several files in a single atomic commit. Prefer this over repeated " +
			"create_file/update_file calls when one logical change touches multiple files.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolFalse(),
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: withProps(map[string]*jsonschema.Schema{
				"files": {
					Type:        "array",
					Description: "File operations applied in one commit, in order",
					MinItems:    ptrInt(1),
					Items: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							"operation": {
								Type:        "string",
								Description: "What to do with the file: `create`, `update` or `delete`",
								Enum:        []any{"create", "update", "delete"},
							},
							"path":    {Type: "string", Description: "Path of the file within the repository"},
							"content": {Type: "string", Description: "File content as plain text (not base64); required for `create` and `update`"},
							"sha": {
								Type:        "string",
								Description: "Current blob sha from get_file_contents; required for `update` and `delete`",
							},
							"from_path": {Type: "string", Description: "Original path of a file being moved to `path` (optional, `update` only)"},
						},
						Required: []string{"operation", "path"},
					},
				},
			}),
			Required: []string{"owner", "repo", "files", "message"},
		},
	}
}

// Handler commits several file operations at once through the custom
// multi-file endpoint, which the SDK does not expose.
func (impl PushFilesImpl) Handler() mcp.ToolHandlerFor[PushFilesParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args PushFilesParams) (*mcp.CallToolResult, any, error) {
		p := args
		ops, err := buildFileOperations(p.Files)
		if err != nil {
			return nil, nil, err
		}

		resp, err := impl.Client.MyChangeFiles(p.Owner, p.Repo, types.MyChangeFilesOptions{
			Files:         ops,
			Message:       p.Message,
			BranchName:    p.Branch,
			NewBranchName: p.NewBranch,
			Author:        identityOrNil(p.AuthorName, p.AuthorEmail),
			Committer:     identityOrNil(p.CommitterName, p.CommitterEmail),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to commit %d file operations: %w%s", len(ops), err, writeErrorHint(err))
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Committed %d file operations to %s/%s%s.\n\n", len(ops), p.Owner, p.Repo, renderTarget(p.Branch, p.NewBranch))
		for _, op := range ops {
			fmt.Fprintf(&b, "- %s `%s`\n", op.Operation, op.Path)
		}
		if resp != nil && resp.Commit != nil {
			b.WriteString("\n")
			if resp.Commit.SHA != "" {
				fmt.Fprintf(&b, "- Commit: `%s`\n", resp.Commit.SHA)
			}
			if resp.Commit.HTMLURL != "" {
				fmt.Fprintf(&b, "- URL: %s\n", resp.Commit.HTMLURL)
			}
		}
		if resp != nil && len(resp.Files) > 0 {
			var shas strings.Builder
			for _, f := range resp.Files {
				if f == nil || f.SHA == "" {
					continue
				}
				fmt.Fprintf(&shas, "- `%s`: `%s`\n", f.Path, f.SHA)
			}
			if shas.Len() > 0 {
				b.WriteString("\nResulting file shas:\n\n")
				b.WriteString(shas.String())
			}
		}
		return textResult(b.String()), nil, nil
	}
}

// buildFileOperations validates the requested operations and encodes their
// content. Validation happens before the request so a malformed batch reports
// which entry is wrong instead of failing as an opaque 422.
func buildFileOperations(files []PushFileParams) ([]types.MyChangeFileOperation, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("files is required and must contain at least one operation")
	}

	ops := make([]types.MyChangeFileOperation, 0, len(files))
	for i, f := range files {
		op := strings.ToLower(strings.TrimSpace(f.Operation))
		if err := requirePath(f.Path); err != nil {
			return nil, fmt.Errorf("files[%d]: %w", i, err)
		}

		entry := types.MyChangeFileOperation{
			Operation: op,
			Path:      f.Path,
			SHA:       f.SHA,
			FromPath:  f.FromPath,
		}

		switch op {
		case "create":
			entry.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(f.Content))
		case "update":
			if strings.TrimSpace(f.SHA) == "" {
				return nil, fmt.Errorf("files[%d] (%s): sha is required for an update; read the file with get_file_contents first", i, f.Path)
			}
			entry.ContentBase64 = base64.StdEncoding.EncodeToString([]byte(f.Content))
		case "delete":
			if strings.TrimSpace(f.SHA) == "" {
				return nil, fmt.Errorf("files[%d] (%s): sha is required for a delete; read the file with get_file_contents first", i, f.Path)
			}
		default:
			return nil, fmt.Errorf("files[%d] (%s): unknown operation %q, expected one of: create, update, delete", i, f.Path, f.Operation)
		}

		ops = append(ops, entry)
	}
	return ops, nil
}

func boolTrue() *bool { b := true; return &b }

func ptrInt(i int) *int { return &i }
