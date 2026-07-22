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

func TestMergePullRequest(t *testing.T) {
	t.Run("success_default_style", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			if r.Method != "POST" || r.URL.Path != "/api/v1/repos/o/r/pulls/7/merge" {
				t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["Do"] != "merge" {
				t.Errorf("expected Do=merge, got %v", body["Do"])
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		impl := MergePullRequestImpl{Client: cl}
		res, _, err := impl.Handler()(context.Background(), nil, MergePullRequestParams{
			Owner: "o", Repo: "r", Index: 7,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if got := textOf(t, res); !strings.Contains(got, "merged successfully") {
			t.Errorf("unexpected text: %q", got)
		}
	})

	t.Run("squash_style_passed_through", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if body["Do"] != "squash" {
				t.Errorf("expected Do=squash, got %v", body["Do"])
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		impl := MergePullRequestImpl{Client: cl}
		res, _, err := impl.Handler()(context.Background(), nil, MergePullRequestParams{
			Owner: "o", Repo: "r", Index: 7, Style: "squash",
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if got := textOf(t, res); !strings.Contains(got, "squash") {
			t.Errorf("expected style in message: %q", got)
		}
	})

	t.Run("not_mergeable_405", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			// 405 Method Not Allowed is what Forgejo returns when a PR is not
			// in a mergeable state. The SDK surfaces this as merged=false with
			// no transport error, so the tool reports a non-merge cleanly.
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"message": "not mergeable"})
		}))
		defer server.Close()

		cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		impl := MergePullRequestImpl{Client: cl}
		res, _, err := impl.Handler()(context.Background(), nil, MergePullRequestParams{
			Owner: "o", Repo: "r", Index: 7,
		})
		if err != nil {
			t.Fatalf("expected no transport error, got %v", err)
		}
		if got := textOf(t, res); !strings.Contains(got, "was not merged") {
			t.Errorf("expected non-merge message, got %q", got)
		}
	})
}

func TestIsPullRequestMerged(t *testing.T) {
	t.Run("merged_204", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			if r.URL.Path != "/api/v1/repos/o/r/pulls/7/merge" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		impl := IsPullRequestMergedImpl{Client: cl}
		res, _, err := impl.Handler()(context.Background(), nil, IsPullRequestMergedParams{
			Owner: "o", Repo: "r", Index: 7,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if got := textOf(t, res); !strings.Contains(got, "has been merged") {
			t.Errorf("unexpected text: %q", got)
		}
	})

	t.Run("not_merged_404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		impl := IsPullRequestMergedImpl{Client: cl}
		res, _, err := impl.Handler()(context.Background(), nil, IsPullRequestMergedParams{
			Owner: "o", Repo: "r", Index: 7,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if got := textOf(t, res); !strings.Contains(got, "has not been merged") {
			t.Errorf("unexpected text: %q", got)
		}
	})
}
