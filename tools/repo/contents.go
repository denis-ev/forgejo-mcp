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
	"sort"
	"strings"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// forgejoContents aliases the SDK contents response for brevity in this file.
type forgejoContents = forgejo.ContentsResponse

// maxFileContentChars bounds how much decoded file content is returned inline,
// so reading a large file cannot blow up the client's context window.
const maxFileContentChars = 50000

// GetFileContentsParams defines the parameters for the get_file_contents tool.
type GetFileContentsParams struct {
	// Owner is the username or organization name that owns the repository.
	Owner string `json:"owner"`
	// Repo is the name of the repository.
	Repo string `json:"repo"`
	// Path is the file or directory path within the repository. Empty means root.
	Path string `json:"path,omitempty"`
	// Ref is an optional branch, tag, or commit SHA (defaults to the default branch).
	Ref string `json:"ref,omitempty"`
}

// GetFileContentsImpl implements the read-only get_file_contents tool. It reads a
// file's decoded contents, or lists a directory's entries, from a Forgejo repo.
type GetFileContentsImpl struct {
	Client *tools.Client
}

// Definition describes the `get_file_contents` tool.
func (GetFileContentsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_file_contents",
		Title:       "Get File Contents",
		Description: "Read a file's contents, or list a directory's entries, from a repository at an optional ref (branch, tag, or commit SHA). Large files are truncated.",
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
				"path": {
					Type:        "string",
					Description: "File or directory path within the repository (optional; empty means the repository root)",
				},
				"ref": {
					Type:        "string",
					Description: "Branch, tag, or commit SHA to read from (optional, defaults to the default branch)",
				},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// Handler reads a file or lists a directory. Forgejo returns a JSON object for a
// file and a JSON array for a directory; the SDK's GetContents targets the
// former, so a failure there is retried as a directory listing.
func (impl GetFileContentsImpl) Handler() mcp.ToolHandlerFor[GetFileContentsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetFileContentsParams) (*mcp.CallToolResult, any, error) {
		p := args

		// Try as a file first.
		fc, _, err := impl.Client.GetContents(p.Owner, p.Repo, p.Ref, p.Path)
		if err == nil && fc != nil && fc.Type == "file" {
			return textResult(renderFile(fc)), nil, nil
		}

		// Fall back to a directory listing.
		entries, _, derr := impl.Client.ListContents(p.Owner, p.Repo, p.Ref, p.Path)
		if derr == nil {
			return textResult(renderDir(p.Path, entries)), nil, nil
		}

		// Neither worked: surface the more meaningful (file) error if present.
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read path %q: %w", p.Path, err)
		}
		return nil, nil, fmt.Errorf("failed to read path %q: %w", p.Path, derr)
	}
}

func renderFile(fc *forgejoContents) string {
	var b strings.Builder
	fmt.Fprintf(&b, "File: %s (%d bytes, sha %s)\n\n", fc.Path, fc.Size, shortSHA(fc.SHA))

	if fc.Content == nil {
		b.WriteString("*(no inline content returned)*")
		return b.String()
	}

	raw := *fc.Content
	if fc.Encoding != nil && *fc.Encoding == "base64" {
		decoded, decErr := base64.StdEncoding.DecodeString(raw)
		if decErr != nil {
			fmt.Fprintf(&b, "*(could not decode base64 content: %v)*", decErr)
			return b.String()
		}
		raw = string(decoded)
	}

	if !isProbablyText(raw) {
		fmt.Fprintf(&b, "*(binary file; %d bytes, not shown)*", fc.Size)
		return b.String()
	}

	truncated := false
	if len(raw) > maxFileContentChars {
		raw = raw[:maxFileContentChars]
		truncated = true
	}
	if truncated {
		b.WriteString("*(content truncated)*\n\n")
	}
	b.WriteString("```\n")
	b.WriteString(raw)
	if !strings.HasSuffix(raw, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```")
	return b.String()
}

func renderDir(path string, entries []*forgejoContents) string {
	var b strings.Builder
	label := path
	if label == "" {
		label = "(root)"
	}
	fmt.Fprintf(&b, "Directory %s — %d entries\n\n", label, len(entries))

	sorted := make([]*forgejoContents, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		// directories first, then alphabetical
		if (sorted[i].Type == "dir") != (sorted[j].Type == "dir") {
			return sorted[i].Type == "dir"
		}
		return sorted[i].Name < sorted[j].Name
	})

	for _, e := range sorted {
		marker := "📄"
		switch e.Type {
		case "dir":
			marker = "📁"
		case "symlink":
			marker = "🔗"
		case "submodule":
			marker = "📦"
		}
		if e.Type == "file" {
			fmt.Fprintf(&b, "%s %s (%d bytes)\n", marker, e.Name, e.Size)
		} else {
			fmt.Fprintf(&b, "%s %s\n", marker, e.Name)
		}
	}
	return b.String()
}

// isProbablyText reports whether s looks like UTF-8 text rather than binary,
// using the presence of NUL bytes as a cheap heuristic.
func isProbablyText(s string) bool {
	if strings.IndexByte(s, 0x00) >= 0 {
		return false
	}
	return true
}

func shortSHA(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
