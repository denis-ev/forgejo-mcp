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

func TestCreatePullRequest_DueDate(t *testing.T) {
	t.Run("omitted_due_date_sends_no_deadline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			// The forgejo SDK's Deadline field has no `omitempty`, so a nil
			// deadline still serializes as an explicit JSON null rather than
			// being omitted from the request body.
			if v, ok := body["due_date"]; ok && v != nil {
				t.Errorf("expected due_date to be unset, got %v", v)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 1})
		}))
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		impl := CreatePullRequestImpl{Client: cl}
		_, _, err = impl.Handler()(context.Background(), nil, CreatePullRequestParams{
			Owner: "o", Repo: "r", Head: "h", Base: "b", Title: "t",
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
	})

	t.Run("valid_due_date_is_sent", func(t *testing.T) {
		const due = "2024-12-31T23:59:59Z"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			got, _ := body["due_date"].(string)
			if !strings.HasPrefix(got, "2024-12-31T23:59:59") {
				t.Errorf("expected due_date to start with 2024-12-31T23:59:59, got %v", body["due_date"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 1})
		}))
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		impl := CreatePullRequestImpl{Client: cl}
		dueDate := due
		_, _, err = impl.Handler()(context.Background(), nil, CreatePullRequestParams{
			Owner: "o", Repo: "r", Head: "h", Base: "b", Title: "t", DueDate: &dueDate,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
	})

	t.Run("empty_string_due_date_is_treated_as_unset", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if v, ok := body["due_date"]; ok && v != nil {
				t.Errorf("expected due_date to be unset, got %v", v)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"number": 1})
		}))
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		impl := CreatePullRequestImpl{Client: cl}
		empty := ""
		_, _, err = impl.Handler()(context.Background(), nil, CreatePullRequestParams{
			Owner: "o", Repo: "r", Head: "h", Base: "b", Title: "t", DueDate: &empty,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
	})

	t.Run("malformed_due_date_string_returns_clean_error_without_calling_api", func(t *testing.T) {
		called := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
				json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
				return
			}
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		impl := CreatePullRequestImpl{Client: cl}
		bad := "not-a-date"
		_, _, err = impl.Handler()(context.Background(), nil, CreatePullRequestParams{
			Owner: "o", Repo: "r", Head: "h", Base: "b", Title: "t", DueDate: &bad,
		})
		if err == nil {
			t.Fatal("expected an error for malformed due_date")
		}
		if !strings.Contains(err.Error(), "due_date") {
			t.Errorf("expected field name in error, got %q", err.Error())
		}
		if called {
			t.Error("expected the create-PR API to never be called for invalid input")
		}
	})
}

// TestCreatePullRequest_DueDate_WireDecode exercises the real MCP argument
// decode path (rather than calling Handler() directly with an already-typed
// Go value) to confirm that a client sending a non-string due_date — e.g. a
// JSON object, which is what triggered issue #27 — now fails with a clean,
// field-naming decode error instead of the previous opaque
// "Time.UnmarshalJSON: input is not a JSON string" failure.
//
// go-sdk v1.7.0+ catches this at JSON-Schema validation, before decoding into
// the typed params struct, and reports it as a successful tools/call result
// with IsError set rather than a protocol-level error.
func TestCreatePullRequest_DueDate_WireDecode(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
			json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
			return
		}
		t.Error("the create-PR API must never be reached for a malformed due_date")
	}))
	defer server.Close()

	cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{Title: "test", Version: "0.0.0"}, nil)
	tools.Register(mcpServer, &CreatePullRequestImpl{Client: cl})

	st, ct := mcp.NewInMemoryTransports()
	if _, err := mcpServer.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Title: "test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_pull_request",
		Arguments: map[string]any{
			"owner": "o", "repo": "r", "head": "h", "base": "b", "title": "t",
			"due_date": map[string]any{"$date": "2026-08-25T00:00:00Z"},
		},
	})
	if err != nil {
		t.Fatalf("expected a tool-level error, not a protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected an error for a non-string due_date")
	}
	msg := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(msg, "due_date") {
		t.Errorf("expected the field name in the error, got %q", msg)
	}
}
