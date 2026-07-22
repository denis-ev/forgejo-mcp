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

func TestClient_MyListActionRuns(t *testing.T) {
	t.Run("success_with_filters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/repos/owner/repo/actions/runs" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("page") != "2" {
				t.Errorf("expected page=2, got %s", q.Get("page"))
			}
			if q.Get("limit") != "10" {
				t.Errorf("expected limit=10, got %s", q.Get("limit"))
			}
			if q.Get("status") != "failure" {
				t.Errorf("expected status=failure, got %s", q.Get("status"))
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"total_count": 1,
				"workflow_runs": []map[string]any{
					{"id": 302, "index_in_repo": 42, "title": "test run", "status": "failure"},
				},
			})
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		resp, err := client.MyListActionRuns("owner", "repo", MyListActionRunsOptions{Page: 2, Limit: 10, Status: "failure"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.TotalCount != 1 || len(resp.Entries) != 1 {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if resp.Entries[0].ID != 302 {
			t.Errorf("expected run id 302, got %d", resp.Entries[0].ID)
		}
	})

	t.Run("no_filters_omits_query", func(t *testing.T) {
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

		if _, err := client.MyListActionRuns("owner", "repo", MyListActionRunsOptions{}); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

func TestClient_MyGetActionRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/owner/repo/actions/runs/302" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": 302, "index_in_repo": 42, "title": "test run", "status": "failure",
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	run, err := client.MyGetActionRun("owner", "repo", 302)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if run.ID != 302 || run.Status != "failure" {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestClient_MyListActionRunJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/owner/repo/actions/runs/302/jobs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1049, "run_id": 302, "name": "mix check", "status": "failure", "task_id": 1199, "attempt": 1},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	jobs, err := client.MyListActionRunJobs("owner", "repo", 302)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != 1049 || jobs[0].TaskID != 1199 {
		t.Errorf("unexpected jobs: %+v", jobs)
	}
}

func TestClient_MyGetActionJobLogs(t *testing.T) {
	t.Run("success_plaintext", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/repos/owner/repo/actions/jobs/1049/logs" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("line 1\nline 2\njob failed\n"))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		logs, err := client.MyGetActionJobLogs("owner", "repo", 1049)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if logs != "line 1\nline 2\njob failed\n" {
			t.Errorf("unexpected logs: %q", logs)
		}
	})

	t.Run("404_not_found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		_, err = client.MyGetActionJobLogs("owner", "repo", 999)
		if err == nil {
			t.Fatal("expected error for 404 response, got nil")
		}
		if !isNotFoundErr(err) {
			t.Errorf("expected isNotFoundErr to be true, got err=%v", err)
		}
	})
}
