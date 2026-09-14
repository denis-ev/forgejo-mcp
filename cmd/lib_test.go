// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/raohwork/forgejo-mcp/tools"
)

// Exercise the first protocol response, not the client's pagination iterator:
// some MCP clients do not follow nextCursor during initial tool discovery.
func TestFirstToolListContainsCompleteCatalog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := createServer(nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "discovery-test", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	result, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if result.NextCursor != "" {
		t.Errorf("first tools/list response is incomplete: nextCursor=%q", result.NextCursor)
	}

	got := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		got = append(got, tool.Name)
	}
	slices.Sort(got)
	registered := tools.RegisteredTools()
	want := make([]string, 0, len(registered))
	for _, tool := range registered {
		want = append(want, tool.Definition.Name)
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("first tools/list response = %v; want full registered catalog %v", got, want)
	}
	for _, name := range []string{"merge_pull_request", "list_pull_request_reviews"} {
		if !slices.Contains(got, name) {
			t.Errorf("first tools/list response is missing %q", name)
		}
	}
}
