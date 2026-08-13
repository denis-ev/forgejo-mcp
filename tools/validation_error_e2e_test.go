// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeMergeParams mirrors the shape of a real tool's params struct.
type fakeMergeParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Index int    `json:"index"`
	Style string `json:"style,omitempty"`
	Title string `json:"title,omitempty"`
}

type fakeMergeImpl struct{}

func (fakeMergeImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "merge_pull_request",
		Title:       "Merge Pull Request",
		Description: "test double",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string"},
				"repo":  {Type: "string"},
				"index": {Type: "integer"},
				"style": {Type: "string"},
				"title": {Type: "string"},
			},
			Required: []string{"owner", "repo", "index"},
		},
	}
}

func (fakeMergeImpl) Handler() mcp.ToolHandlerFor[fakeMergeParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args fakeMergeParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "merged with style=" + args.Style}},
		}, nil, nil
	}
}

// TestValidationErrorMiddleware_EndToEnd drives a real client/server session
// over an in-memory transport, so it exercises the actual SDK decode path that
// produces the unknown-field error rather than a hand-built error value.
func TestValidationErrorMiddleware_EndToEnd(t *testing.T) {
	ctx := context.Background()

	newSession := func(t *testing.T) *mcp.ClientSession {
		t.Helper()
		server := mcp.NewServer(&mcp.Implementation{Title: "test", Version: "0.0.0"}, nil)
		server.AddReceivingMiddleware(ValidationErrorMiddleware())
		Register(server, fakeMergeImpl{})

		st, ct := mcp.NewInMemoryTransports()
		if _, err := server.Connect(ctx, st, nil); err != nil {
			t.Fatalf("server connect: %v", err)
		}
		client := mcp.NewClient(&mcp.Implementation{Title: "test-client", Version: "0.0.0"}, nil)
		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		t.Cleanup(func() { cs.Close() })
		return cs
	}

	t.Run("rest_alias_is_enriched", func(t *testing.T) {
		cs := newSession(t)

		// "do" is the REST API's field name; this server calls it "style".
		_, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name: "merge_pull_request",
			Arguments: map[string]any{
				"owner": "o", "repo": "r", "index": 1, "do": "merge",
			},
		})
		if err == nil {
			t.Fatal("expected an error for unknown field")
		}
		msg := err.Error()
		if !strings.Contains(msg, `did you mean "style"`) {
			t.Errorf("expected style suggestion in %q", msg)
		}
		if !strings.Contains(msg, "expected one of:") {
			t.Errorf("expected accepted field list in %q", msg)
		}
		if !strings.Contains(msg, "index, owner, repo") {
			t.Errorf("expected required fields first in %q", msg)
		}
	})

	t.Run("valid_call_still_succeeds", func(t *testing.T) {
		cs := newSession(t)

		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name: "merge_pull_request",
			Arguments: map[string]any{
				"owner": "o", "repo": "r", "index": 1, "style": "merge",
			},
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if text != "merged with style=merge" {
			t.Errorf("unexpected result: %q", text)
		}
	})
}
