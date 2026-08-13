// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_MyListActionTasks(t *testing.T) {
	t.Run("forwards_pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/repos/owner/repo/actions/tasks" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("page") != "2" {
				t.Errorf("expected page=2, got %s", q.Get("page"))
			}
			if q.Get("limit") != "10" {
				t.Errorf("expected limit=10, got %s", q.Get("limit"))
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"total_count": 444,
				"workflow_runs": []map[string]any{
					{"id": 1, "name": "task one", "status": "success"},
				},
			})
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		resp, err := client.MyListActionTasks("owner", "repo", MyListActionTasksOptions{Page: 2, Limit: 10})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.TotalCount != 444 {
			t.Errorf("expected total_count 444, got %d", resp.TotalCount)
		}
		if len(resp.WorkflowRuns) != 1 {
			t.Fatalf("expected 1 returned task, got %d", len(resp.WorkflowRuns))
		}
	})

	// Forgejo ignores "limit" unless "page" is present, so a limit-only
	// request must still pin page=1 or the server returns everything.
	t.Run("limit_only_defaults_page_1", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("limit") != "5" {
				t.Errorf("expected limit=5, got %s", q.Get("limit"))
			}
			if q.Get("page") != "1" {
				t.Errorf("expected page=1 alongside limit, got %q", q.Get("page"))
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"total_count": 0, "workflow_runs": []any{}})
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		if _, err := client.MyListActionTasks("owner", "repo", MyListActionTasksOptions{Limit: 5}); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("no_options_omits_query", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" {
				t.Errorf("expected no query string, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"total_count": 0, "workflow_runs": []any{}})
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		if _, err := client.MyListActionTasks("owner", "repo", MyListActionTasksOptions{}); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}
