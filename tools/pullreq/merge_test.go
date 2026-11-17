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
			// 405 is what Forgejo returns when a protected base branch's
			// requirements are not met; its reason is in the body.
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{"message": "Not all required status checks successful", "url": "x"})
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
		if !res.IsError {
			t.Errorf("a refused merge must be a tool error")
		}
		got := textOf(t, res)
		for _, want := range []string{"NOT merged", "HTTP 405", "Not all required status checks successful", "get_ci_status"} {
			if !strings.Contains(got, want) {
				t.Errorf("expected %q in %q", want, got)
			}
		}
	})
}

// mergeServer answers the merge endpoint with the given status and body and
// records the decoded request body.
func mergeServer(t *testing.T, status int, respBody any, gotBody *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
			json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
			return
		}
		if r.URL.Path != "/api/v1/repos/o/r/pulls/7/merge" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if gotBody != nil {
			json.NewDecoder(r.Body).Decode(gotBody)
		}
		w.WriteHeader(status)
		if respBody != nil {
			json.NewEncoder(w).Encode(respBody)
		}
	}))
}

func TestMergePullRequestOutcomes(t *testing.T) {
	cases := []struct {
		name    string
		params  MergePullRequestParams
		status  int
		resp    any
		isError bool
		want    []string
	}{
		{
			name:   "auto_merge_scheduled_201",
			params: MergePullRequestParams{Owner: "o", Repo: "r", Index: 7, MergeWhenChecksSucceed: true},
			status: http.StatusCreated,
			want:   []string{"Auto-merge scheduled", "cancel_auto_merge"},
		},
		{
			name:   "auto_merge_requested_but_checks_already_green_200",
			params: MergePullRequestParams{Owner: "o", Repo: "r", Index: 7, MergeWhenChecksSucceed: true},
			status: http.StatusOK,
			want:   []string{"merged immediately"},
		},
		{
			name:   "merged_but_head_branch_kept_403",
			params: MergePullRequestParams{Owner: "o", Repo: "r", Index: 7, DeleteBranchAfterMerge: true},
			status: http.StatusForbidden,
			resp:   map[string]string{"message": "the head branch is protected"},
			want:   []string{"merged successfully", "head branch was not deleted", "the head branch is protected"},
		},
		{
			name:    "forbidden_without_branch_deletion_is_a_refusal",
			params:  MergePullRequestParams{Owner: "o", Repo: "r", Index: 7},
			status:  http.StatusForbidden,
			resp:    map[string]string{"message": "token does not have at least one of required scope(s)"},
			isError: true,
			want:    []string{"NOT merged", "HTTP 403", "required scope"},
		},
		{
			name:    "conflict_409_without_message",
			params:  MergePullRequestParams{Owner: "o", Repo: "r", Index: 7},
			status:  http.StatusConflict,
			resp:    map[string]any{},
			isError: true,
			want:    []string{"NOT merged", "HTTP 409", "Conflict", "update the branch"},
		},
		{
			name:    "disallowed_style",
			params:  MergePullRequestParams{Owner: "o", Repo: "r", Index: 7, Style: "rebase"},
			status:  http.StatusMethodNotAllowed,
			resp:    map[string]string{"message": "Invalid merge style", "url": "x"},
			isError: true,
			want:    []string{"NOT merged", "Invalid merge style"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := mergeServer(t, tc.status, tc.resp, nil)
			defer server.Close()
			cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
			res, _, err := MergePullRequestImpl{Client: cl}.Handler()(context.Background(), nil, tc.params)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if res.IsError != tc.isError {
				t.Errorf("IsError = %v, want %v", res.IsError, tc.isError)
			}
			got := textOf(t, res)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("expected %q in %q", w, got)
				}
			}
		})
	}
}

func TestMergePullRequestBody(t *testing.T) {
	var body map[string]any
	server := mergeServer(t, http.StatusOK, nil, &body)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	_, _, err := MergePullRequestImpl{Client: cl}.Handler()(context.Background(), nil, MergePullRequestParams{
		Owner: "o", Repo: "r", Index: 7, Style: "fast-forward-only", Title: "T", Message: "M",
		DeleteBranchAfterMerge: true, HeadCommitID: "abc123",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	want := map[string]any{
		"Do": "fast-forward-only", "MergeTitleField": "T", "MergeMessageField": "M",
		"delete_branch_after_merge": true, "head_commit_id": "abc123",
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%q] = %v, want %v", k, body[k], v)
		}
	}
	if _, ok := body["merge_when_checks_succeed"]; ok {
		t.Errorf("unset merge_when_checks_succeed should be omitted, body: %v", body)
	}
}

func TestCancelAutoMerge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" || r.URL.Path != "/api/v1/repos/o/r/pulls/7/merge" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	res, _, err := CancelAutoMergeImpl{Client: cl}.Handler()(context.Background(), nil, CancelAutoMergeParams{Owner: "o", Repo: "r", Index: 7})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if got := textOf(t, res); !strings.Contains(got, "Cancelled the scheduled auto-merge") {
		t.Errorf("unexpected text: %q", got)
	}
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
