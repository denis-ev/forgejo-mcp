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

func TestListBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/v1/repos/o/r/branches" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "main", "protected": true, "commit": map[string]interface{}{"id": "aaaaaaaaaaaa"}},
			{"name": "dev", "protected": false, "commit": map[string]interface{}{"id": "bbbbbbbbbbbb"}},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := ListBranchesImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, ListBranchesParams{Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "Found 2 branches") {
		t.Errorf("expected count, got: %q", got)
	}
	if !strings.Contains(got, "main") || !strings.Contains(got, "[protected]") {
		t.Errorf("expected protected main, got: %q", got)
	}
}

func TestCreateBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if r.Method != "POST" || r.URL.Path != "/api/v1/repos/o/r/branches" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["new_branch_name"] != "feature" {
			t.Errorf("expected new_branch_name=feature, got %v", body["new_branch_name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":   "feature",
			"commit": map[string]interface{}{"id": "cccccccccccc"},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := CreateBranchImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, CreateBranchParams{
		Owner: "o", Repo: "r", Branch: "feature", OldBranch: "main",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if got := textOf(t, res); !strings.Contains(got, `Created branch "feature"`) {
		t.Errorf("unexpected text: %q", got)
	}
}

func TestListTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if r.URL.Path != "/api/v1/repos/o/r/tags" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "v1.0.0", "message": "release one\nmore", "commit": map[string]interface{}{"sha": "1234567890ab"}},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := ListTagsImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, ListTagsParams{Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	if !strings.Contains(got, "Found 1 tags") || !strings.Contains(got, "v1.0.0") {
		t.Errorf("unexpected text: %q", got)
	}
	if !strings.Contains(got, "release one") || strings.Contains(got, "more") {
		t.Errorf("expected only first line of message, got: %q", got)
	}
}

func TestCreateTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		if r.Method != "POST" || r.URL.Path != "/api/v1/repos/o/r/tags" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["tag_name"] != "v2.0.0" {
			t.Errorf("expected tag_name=v2.0.0, got %v", body["tag_name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":   "v2.0.0",
			"commit": map[string]interface{}{"sha": "abcabcabcabc"},
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := CreateTagImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, CreateTagParams{
		Owner: "o", Repo: "r", Tag: "v2.0.0", Target: "main", Message: "second",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if got := textOf(t, res); !strings.Contains(got, `Created tag "v2.0.0"`) {
		t.Errorf("unexpected text: %q", got)
	}
}
