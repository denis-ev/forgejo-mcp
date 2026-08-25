// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package milestone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raohwork/forgejo-mcp/tools"
)

const testForgejoVersion = "16.0.1+gitea-1.22.0"

func newVersionAwareServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/version") {
			json.NewEncoder(w).Encode(map[string]string{"version": testForgejoVersion})
			return
		}
		handle(w, r)
	}))
}

func TestCreateMilestone_DueDate(t *testing.T) {
	t.Run("valid_due_date_is_sent", func(t *testing.T) {
		server := newVersionAwareServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			// The forgejo SDK's milestone Deadline field serializes as
			// "due_on", not "due_date" (unlike issues/PRs); our tool's
			// input parameter is still named due_date.
			got, _ := body["due_on"].(string)
			if !strings.HasPrefix(got, "2024-12-31T23:59:59") {
				t.Errorf("expected due_on to start with 2024-12-31T23:59:59, got %v", body["due_on"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
		})
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		due := "2024-12-31T23:59:59Z"
		impl := CreateMilestoneImpl{Client: cl}
		_, _, err = impl.Handler()(context.Background(), nil, CreateMilestoneParams{
			Owner: "o", Repo: "r", Title: "t", DueDate: &due,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
	})

	t.Run("empty_string_due_date_is_treated_as_unset", func(t *testing.T) {
		server := newVersionAwareServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			if v, ok := body["due_on"]; ok && v != nil {
				t.Errorf("expected due_on to be unset, got %v", v)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
		})
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		empty := ""
		impl := CreateMilestoneImpl{Client: cl}
		_, _, err = impl.Handler()(context.Background(), nil, CreateMilestoneParams{
			Owner: "o", Repo: "r", Title: "t", DueDate: &empty,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
	})

	t.Run("malformed_due_date_returns_clean_error_without_calling_api", func(t *testing.T) {
		called := false
		server := newVersionAwareServer(t, func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		bad := "not-a-date"
		impl := CreateMilestoneImpl{Client: cl}
		_, _, err = impl.Handler()(context.Background(), nil, CreateMilestoneParams{
			Owner: "o", Repo: "r", Title: "t", DueDate: &bad,
		})
		if err == nil {
			t.Fatal("expected an error for malformed due_date")
		}
		if !strings.Contains(err.Error(), "due_date") {
			t.Errorf("expected field name in error, got %q", err.Error())
		}
		if called {
			t.Error("expected the create-milestone API to never be called for invalid input")
		}
	})
}

func TestEditMilestone_DueDate(t *testing.T) {
	t.Run("valid_due_date_is_sent", func(t *testing.T) {
		server := newVersionAwareServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			got, _ := body["due_on"].(string)
			if !strings.HasPrefix(got, "2024-12-31T23:59:59") {
				t.Errorf("expected due_on to start with 2024-12-31T23:59:59, got %v", body["due_on"])
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
		})
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		due := "2024-12-31T23:59:59Z"
		impl := EditMilestoneImpl{Client: cl}
		_, _, err = impl.Handler()(context.Background(), nil, EditMilestoneParams{
			Owner: "o", Repo: "r", ID: 1, DueDate: &due,
		})
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
	})

	t.Run("malformed_due_date_returns_clean_error_without_calling_api", func(t *testing.T) {
		called := false
		server := newVersionAwareServer(t, func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		defer server.Close()

		cl, err := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		bad := "not-a-date"
		impl := EditMilestoneImpl{Client: cl}
		_, _, err = impl.Handler()(context.Background(), nil, EditMilestoneParams{
			Owner: "o", Repo: "r", ID: 1, DueDate: &bad,
		})
		if err == nil {
			t.Fatal("expected an error for malformed due_date")
		}
		if !strings.Contains(err.Error(), "due_date") {
			t.Errorf("expected field name in error, got %q", err.Error())
		}
		if called {
			t.Error("expected the edit-milestone API to never be called for invalid input")
		}
	})
}
