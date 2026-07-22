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

	"github.com/raohwork/forgejo-mcp/types"
)

// TestNameToWikiSubURL documents the expected slug conversion for wiki page
// titles, mirroring Gitea's own wiki.NameToSubURL implementation: spaces
// become hyphens, then the result is percent-encoded.
func TestNameToWikiSubURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple title", "Getting Started", "Getting-Started"},
		{"slash in title", "architecture/signal-processing-strategies", "architecture%2Fsignal-processing-strategies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nameToWikiSubURL(tt.in)
			if got != tt.want {
				t.Errorf("nameToWikiSubURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestClient_MyGetWikiPage_TitleFallback verifies that when a caller passes
// a human-readable wiki page title (containing characters like "/") that
// the server rejects at its raw form, the client retries with the
// slug-encoded form and succeeds. This is the fix for issue #5: previously
// get_wiki_page returned 404 whenever a title (rather than sub_url slug)
// was supplied.
func TestClient_MyGetWikiPage_TitleFallback(t *testing.T) {
	const title = "architecture/signal-processing-strategies"
	const slug = "architecture%2Fsignal-processing-strategies"

	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.EscapedPath())
		wantSlugPath := "/api/v1/repos/owner/repo/wiki/page/" + slug
		if r.URL.EscapedPath() == wantSlugPath {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"title": title,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	page, err := client.MyGetWikiPage("owner", "repo", title)
	if err != nil {
		t.Fatalf("Expected fallback request to succeed, got error: %v", err)
	}
	if page.Title != title {
		t.Errorf("Expected title %q, got %q", title, page.Title)
	}
	if len(gotPaths) != 2 {
		t.Fatalf("Expected 2 requests (raw then slug), got %d: %v", len(gotPaths), gotPaths)
	}
}

// TestClient_MyGetWikiPage_SlugFirstTry verifies that when the caller
// already passes the correct slug, only a single request is made (no
// unnecessary retry).
func TestClient_MyGetWikiPage_SlugFirstTry(t *testing.T) {
	const slug = "Getting-Started"

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title": "Getting Started",
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.MyGetWikiPage("owner", "repo", slug)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if requestCount != 1 {
		t.Errorf("Expected exactly 1 request, got %d", requestCount)
	}
}

// TestClient_MyGetWikiPage_NotFoundBothForms verifies that when neither the
// raw form nor the slug form exists, the original 404 error is returned
// (not masked by the retry).
func TestClient_MyGetWikiPage_NotFoundBothForms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.MyGetWikiPage("owner", "repo", "some/missing page")
	if err == nil {
		t.Fatal("Expected error for missing page, got nil")
	}
}

// TestClient_MyEditWikiPage_TitleFallback mirrors the get-page fallback
// behavior for the edit (PATCH) endpoint.
func TestClient_MyEditWikiPage_TitleFallback(t *testing.T) {
	const title = "architecture/signal-processing-strategies"
	const slug = "architecture%2Fsignal-processing-strategies"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantSlugPath := "/api/v1/repos/owner/repo/wiki/page/" + slug
		if r.URL.EscapedPath() == wantSlugPath {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"title": title})
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.MyEditWikiPage("owner", "repo", title, types.MyCreateWikiPageOptions{
		ContentBase64: "dGVzdA==",
	})
	if err != nil {
		t.Fatalf("Expected fallback request to succeed, got error: %v", err)
	}
}

// TestClient_MyDeleteWikiPage_TitleFallback mirrors the get-page fallback
// behavior for the delete (DELETE) endpoint.
func TestClient_MyDeleteWikiPage_TitleFallback(t *testing.T) {
	const title = "architecture/signal-processing-strategies"
	const slug = "architecture%2Fsignal-processing-strategies"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantSlugPath := "/api/v1/repos/owner/repo/wiki/page/" + slug
		if r.URL.EscapedPath() == wantSlugPath {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-token", forgejo_version_to_test, server.Client())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := client.MyDeleteWikiPage("owner", "repo", title); err != nil {
		t.Fatalf("Expected fallback request to succeed, got error: %v", err)
	}
}
