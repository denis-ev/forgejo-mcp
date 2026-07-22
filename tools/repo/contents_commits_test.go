// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

const testForgejoVersion = "16.0.1+gitea-1.22.0"

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("expected content, got none")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

func versionHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
		json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
		return true
	}
	return false
}

func TestGetFileContents_File(t *testing.T) {
	content := "hello\nworld\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/v1/repos/o/r/contents/README.md" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		enc := "base64"
		b64 := base64.StdEncoding.EncodeToString([]byte(content))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":     "README.md",
			"path":     "README.md",
			"sha":      "abcdef1234567890",
			"type":     "file",
			"size":     len(content),
			"encoding": enc,
			"content":  b64,
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetFileContentsImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetFileContentsParams{
		Owner: "o", Repo: "r", Path: "README.md",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Errorf("expected decoded content, got: %q", got)
	}
	if !strings.Contains(got, "File: README.md") {
		t.Errorf("expected file header, got: %q", got)
	}
}

func TestGetFileContents_Directory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		// A file-style GetContents on a directory returns an array, which the
		// SDK's single-object GetContents cannot decode -> triggers fallback.
		if r.URL.Path == "/api/v1/repos/o/r/contents/src" {
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"name": "main.go", "path": "src/main.go", "type": "file", "size": 100, "sha": "aaa"},
				{"name": "pkg", "path": "src/pkg", "type": "dir", "sha": "bbb"},
			})
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetFileContentsImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetFileContentsParams{
		Owner: "o", Repo: "r", Path: "src",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "Directory src") {
		t.Errorf("expected directory header, got: %q", got)
	}
	if !strings.Contains(got, "main.go") || !strings.Contains(got, "pkg") {
		t.Errorf("expected entries, got: %q", got)
	}
}

func TestListCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/repos/o/r/commits") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"sha": "1111111111111111",
				"commit": map[string]interface{}{
					"message": "first commit\n\nbody",
					"author":  map[string]interface{}{"name": "Alice", "email": "a@x", "date": "2026-01-01T00:00:00Z"},
				},
			},
			{
				"sha": "2222222222222222",
				"commit": map[string]interface{}{
					"message": "second commit",
					"author":  map[string]interface{}{"name": "Bob", "email": "b@x", "date": "2026-01-02T00:00:00Z"},
				},
			},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := ListCommitsImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, ListCommitsParams{Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "Found 2 commits") {
		t.Errorf("expected count, got: %q", got)
	}
	if !strings.Contains(got, "first commit") || strings.Contains(got, "body") {
		t.Errorf("expected only first line of message, got: %q", got)
	}
	if !strings.Contains(got, "Alice") {
		t.Errorf("expected author, got: %q", got)
	}
}

func TestGetCommit_WithDiff(t *testing.T) {
	diff := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		switch {
		case r.URL.Path == "/api/v1/repos/o/r/git/commits/deadbeef.diff":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(diff))
		case r.URL.Path == "/api/v1/repos/o/r/git/commits/deadbeef":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": "deadbeef0000",
				"commit": map[string]interface{}{
					"message": "fix things",
					"author":  map[string]interface{}{"name": "Carol", "email": "c@x", "date": "2026-01-03T00:00:00Z"},
				},
				"stats": map[string]interface{}{"total": 2, "additions": 1, "deletions": 1},
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetCommitImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetCommitParams{
		Owner: "o", Repo: "r", SHA: "deadbeef", IncludeDiff: true,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "fix things") {
		t.Errorf("expected message, got: %q", got)
	}
	if !strings.Contains(got, "+new") || !strings.Contains(got, "```diff") {
		t.Errorf("expected diff block, got: %q", got)
	}
	if !strings.Contains(got, "+1 -1") {
		t.Errorf("expected stats, got: %q", got)
	}
}
