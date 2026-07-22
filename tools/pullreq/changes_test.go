// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package pullreq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raohwork/forgejo-mcp/tools"
)

func TestGetPullRequestFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
			json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
			return
		}
		if r.URL.Path != "/api/v1/repos/o/r/pulls/5/files" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"filename": "a.go", "status": "modified", "additions": 3, "deletions": 1},
			{"filename": "b.go", "previous_filename": "old_b.go", "status": "renamed", "additions": 0, "deletions": 0},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetPullRequestFilesImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetPullRequestFilesParams{
		Owner: "o", Repo: "r", Index: 5,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "Found 2 changed files") {
		t.Errorf("expected count, got: %q", got)
	}
	if !strings.Contains(got, "a.go (+3 -1)") {
		t.Errorf("expected file stats, got: %q", got)
	}
	if !strings.Contains(got, "renamed from old_b.go") {
		t.Errorf("expected rename note, got: %q", got)
	}
}

func TestGetPullRequestDiff(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
			json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/repos/o/r/pulls/5.diff") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(diff))
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetPullRequestDiffImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetPullRequestDiffParams{
		Owner: "o", Repo: "r", Index: 5,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "```diff") || !strings.Contains(got, "+y") {
		t.Errorf("expected diff block, got: %q", got)
	}
}
