// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raohwork/forgejo-mcp/tools"
)

func TestGetCommitStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v1/repos/o/r/commits/main/status") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"state":       "success",
			"sha":         "abcdef123456",
			"total_count": 2,
			"statuses": []map[string]interface{}{
				{"status": "success", "context": "ci/build", "description": "built ok"},
				{"status": "success", "context": "ci/test", "description": "tests passed"},
			},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := GetCommitStatusImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, GetCommitStatusParams{
		Owner: "o", Repo: "r", Ref: "main",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "success (2 contexts)") {
		t.Errorf("expected combined state, got: %q", got)
	}
	if !strings.Contains(got, "ci/build") || !strings.Contains(got, "tests passed") {
		t.Errorf("expected individual contexts, got: %q", got)
	}
}

func TestCreateCommitStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if r.Method != "POST" || r.URL.Path != "/api/v1/repos/o/r/statuses/deadbeef" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["state"] != "success" {
			t.Errorf("expected state=success, got %v", body["state"])
		}
		if body["context"] != "ci/deploy" {
			t.Errorf("expected context=ci/deploy, got %v", body["context"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"context": "ci/deploy",
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := CreateCommitStatusImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, CreateCommitStatusParams{
		Owner: "o", Repo: "r", SHA: "deadbeef", State: "success", Context: "ci/deploy", Description: "deployed",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if got := textOf(t, res); !strings.Contains(got, "Created success status") {
		t.Errorf("unexpected text: %q", got)
	}
}
