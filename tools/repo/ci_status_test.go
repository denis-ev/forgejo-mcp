// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raohwork/forgejo-mcp/tools"
)

func TestNormalizeCIState(t *testing.T) {
	cases := map[string]string{
		"success":   ciStateSuccess,
		"failure":   ciStateFailure,
		"error":     ciStateFailure,
		"cancelled": ciStateFailure,
		"pending":   ciStatePending,
		"running":   ciStatePending,
		"waiting":   ciStatePending,
		"blocked":   ciStatePending,
		"skipped":   ciStateNeutral,
		"warning":   ciStateNeutral,
		"":          ciStateNone,
		"SUCCESS":   ciStateSuccess,
	}
	for raw, want := range cases {
		if got := normalizeCIState(raw); got != want {
			t.Errorf("normalizeCIState(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestRollupCIState(t *testing.T) {
	tests := []struct {
		name   string
		states []string
		want   string
	}{
		{"all green", []string{ciStateSuccess, ciStateSuccess}, ciStateSuccess},
		{"failure wins over pending", []string{ciStatePending, ciStateFailure}, ciStateFailure},
		{"failure wins over success", []string{ciStateSuccess, ciStateFailure}, ciStateFailure},
		{"pending wins over success", []string{ciStateSuccess, ciStatePending}, ciStatePending},
		{"neutral does not mask success", []string{ciStateSuccess, ciStateNeutral}, ciStateSuccess},
		{"only neutral", []string{ciStateNeutral}, ciStateNone},
		{"empty", nil, ciStateNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := make([]ciCheck, len(tt.states))
			for i, s := range tt.states {
				checks[i] = ciCheck{State: s}
			}
			if got := rollupCIState(checks); got != tt.want {
				t.Errorf("rollupCIState() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ciTestServer serves the endpoints get_ci_status depends on.
func ciTestServer(t *testing.T, headSHA string, runs []map[string]any, jobs map[int64][]map[string]any, statuses []map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		p := r.URL.Path
		switch {
		case strings.HasSuffix(p, "/pulls/7"):
			json.NewEncoder(w).Encode(map[string]any{
				"number": 7,
				"head":   map[string]any{"sha": headSHA, "ref": "feature"},
			})
		// Order matters: the combined-status endpoint is
		// /commits/{sha}/status, so it must be matched before the
		// bare commit-resolution route below.
		case strings.HasSuffix(p, "/status"):
			json.NewEncoder(w).Encode(map[string]any{
				"state":       "success",
				"sha":         headSHA,
				"total_count": len(statuses),
				"statuses":    statuses,
			})
		case strings.Contains(p, "/git/commits/") || strings.Contains(p, "/commits/"):
			json.NewEncoder(w).Encode(map[string]any{"sha": headSHA})
		case strings.HasSuffix(p, "/actions/runs"):
			json.NewEncoder(w).Encode(map[string]any{
				"total_count":   len(runs),
				"workflow_runs": runs,
			})
		case strings.Contains(p, "/actions/runs/") && strings.HasSuffix(p, "/jobs"):
			var runID int64
			fmt.Sscanf(p[strings.LastIndex(p, "/runs/")+len("/runs/"):], "%d", &runID)
			json.NewEncoder(w).Encode(jobs[runID])
		default:
			t.Logf("unhandled path: %s", p)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestGetCIStatusByPullNumberMergesSources(t *testing.T) {
	const sha = "abcdef1234567890"
	runs := []map[string]any{
		{"id": 11, "commit_sha": sha, "title": "CI", "workflow_id": "ci.yml", "status": "failure", "html_url": "https://git.example.com/o/r/actions/runs/11"},
		{"id": 12, "commit_sha": "otherSHA", "title": "Unrelated", "workflow_id": "other.yml", "status": "success"},
	}
	jobs := map[int64][]map[string]any{
		11: {
			{"id": 101, "run_id": 11, "name": "build", "status": "success"},
			{"id": 102, "run_id": 11, "name": "test", "status": "failure"},
		},
	}
	statuses := []map[string]any{
		{"status": "success", "context": "ci/lint", "description": "lint ok"},
	}

	server := ciTestServer(t, sha, runs, jobs, statuses)
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetCIStatusImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetCIStatusParams{
		Owner: "o", Repo: "r", PullNumber: 7,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)

	// Rolled up to FAILURE because one job failed.
	if !strings.Contains(got, "FAILURE") {
		t.Errorf("expected rolled-up FAILURE, got: %q", got)
	}
	// Commit status and Actions jobs both present.
	if !strings.Contains(got, "ci/lint") {
		t.Errorf("expected commit status context, got: %q", got)
	}
	if !strings.Contains(got, "ci.yml / test") {
		t.Errorf("expected actions job, got: %q", got)
	}
	// Job IDs surfaced for the log drill-down.
	if !strings.Contains(got, "job_id: 102") {
		t.Errorf("expected failing job id, got: %q", got)
	}
	if !strings.Contains(got, "get_action_job_logs") {
		t.Errorf("expected drill-down hint, got: %q", got)
	}
	// Runs for other commits must not leak in.
	if strings.Contains(got, "Unrelated") {
		t.Errorf("run for a different commit leaked into output: %q", got)
	}
}

func TestGetCIStatusRequiresRefOrPullNumber(t *testing.T) {
	server := ciTestServer(t, "deadbeef", nil, nil, nil)
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetCIStatusImpl{Client: cl}
	_, _, err := impl.Handler()(context.Background(), nil, GetCIStatusParams{Owner: "o", Repo: "r"})
	if err == nil {
		t.Fatal("expected error when neither ref nor pull_number is supplied")
	}
	if !strings.Contains(err.Error(), "pull_number") {
		t.Errorf("expected actionable error, got: %v", err)
	}
}

func TestGetCIStatusSurvivesActionsUnavailable(t *testing.T) {
	const sha = "abcdef1234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		p := r.URL.Path
		switch {
		case strings.Contains(p, "/actions/runs"):
			// Instance with Actions disabled.
			w.WriteHeader(http.StatusNotFound)
		case strings.Contains(p, "/status"):
			json.NewEncoder(w).Encode(map[string]any{
				"state": "success", "sha": sha, "total_count": 1,
				"statuses": []map[string]any{{"status": "success", "context": "ci/build"}},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{"sha": sha})
		}
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetCIStatusImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetCIStatusParams{
		Owner: "o", Repo: "r", Ref: sha,
	})
	if err != nil {
		t.Fatalf("Actions being unavailable must not fail the whole call: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "ci/build") {
		t.Errorf("expected commit statuses to still be reported, got: %q", got)
	}
	if !strings.Contains(got, "SUCCESS") {
		t.Errorf("expected rollup from commit statuses alone, got: %q", got)
	}
}
