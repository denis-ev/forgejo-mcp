// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

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

func ptr[T any](v T) *T { return &v }

// protectionServer serves the branch protection endpoints of o/r, records
// the last request method, path and decoded body, and answers with resp.
type recorded struct {
	method, path string
	body         map[string]any
}

func protectionServer(t *testing.T, status int, resp any, rec *recorded) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		rec.method, rec.path = r.Method, r.URL.Path
		rec.body = nil
		if r.Body != nil && (r.Method == "POST" || r.Method == "PATCH") {
			json.NewDecoder(r.Body).Decode(&rec.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if resp != nil {
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

var mainRule = map[string]any{
	"rule_name":             "main",
	"enable_push":           false,
	"enable_status_check":   true,
	"status_check_contexts": []string{"ci / typecheck + build", "ci / semgrep"},
	"apply_to_admins":       true,
}

func TestCreateBranchProtection(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusCreated, mainRule, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())

	contexts := []string{"ci / typecheck + build", "ci / semgrep"}
	res, _, err := CreateBranchProtectionImpl{Client: cl}.Handler()(context.Background(), nil, CreateBranchProtectionParams{
		Owner: "o", Repo: "r", RuleName: "main",
		BranchProtectionRules: tools.BranchProtectionRules{
			StatusCheckContexts: &contexts,
			ApplyToAdmins:       ptr(true),
		},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.method != "POST" || rec.path != "/api/v1/repos/o/r/branch_protections" {
		t.Errorf("unexpected request %s %s", rec.method, rec.path)
	}
	if rec.body["rule_name"] != "main" {
		t.Errorf("rule_name not sent: %v", rec.body)
	}
	if rec.body["enable_status_check"] != true {
		t.Errorf("status_check_contexts should switch on enable_status_check: %v", rec.body)
	}
	if rec.body["apply_to_admins"] != true {
		t.Errorf("apply_to_admins not sent: %v", rec.body)
	}
	if _, ok := rec.body["enable_push"]; ok {
		t.Errorf("unset enable_push must be omitted so Forgejo's default applies: %v", rec.body)
	}
	got := textOf(t, res)
	for _, want := range []string{"Created branch protection", "**main**", "Direct push: disabled", "`ci / semgrep`", "applies to admins: on"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestCreateBranchProtectionRequiresRuleName(t *testing.T) {
	cl, _ := tools.NewClient("http://127.0.0.1:1", "tok", testForgejoVersion, nil)
	_, _, err := CreateBranchProtectionImpl{Client: cl}.Handler()(context.Background(), nil, CreateBranchProtectionParams{Owner: "o", Repo: "r"})
	if err == nil || !strings.Contains(err.Error(), "rule_name") {
		t.Fatalf("expected rule_name error, got %v", err)
	}
}

func TestEditBranchProtectionSendsOnlyGivenFields(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusOK, mainRule, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())

	empty := []string{}
	_, _, err := EditBranchProtectionImpl{Client: cl}.Handler()(context.Background(), nil, EditBranchProtectionParams{
		Owner: "o", Repo: "r", Name: "release/*",
		BranchProtectionRules: tools.BranchProtectionRules{
			RequiredApprovals:      ptr(int64(0)),
			PushWhitelistUsernames: &empty,
		},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.method != "PATCH" || rec.path != "/api/v1/repos/o/r/branch_protections/release/*" {
		t.Errorf("unexpected request %s %s", rec.method, rec.path)
	}
	if len(rec.body) != 2 {
		t.Errorf("expected exactly the two given fields, got %v", rec.body)
	}
	if v, ok := rec.body["required_approvals"]; !ok || v != float64(0) {
		t.Errorf("explicit 0 must be sent: %v", rec.body)
	}
	if v, ok := rec.body["push_whitelist_usernames"].([]any); !ok || len(v) != 0 {
		t.Errorf("explicit empty list must be sent to clear it: %v", rec.body)
	}
}

func TestListBranchProtections(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusOK, []any{mainRule, map[string]any{"rule_name": "dev", "enable_push": true}}, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())

	res, _, err := ListBranchProtectionsImpl{Client: cl}.Handler()(context.Background(), nil, ListBranchProtectionsParams{Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	got := textOf(t, res)
	for _, want := range []string{"Found 2 branch protection rules", "**main**", "**dev**", "Direct push: allowed"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestListBranchProtectionsEmpty(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusOK, []any{}, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	res, _, err := ListBranchProtectionsImpl{Client: cl}.Handler()(context.Background(), nil, ListBranchProtectionsParams{Owner: "o", Repo: "r"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if got := textOf(t, res); !strings.Contains(got, "No branch protection rules") {
		t.Errorf("unexpected text: %q", got)
	}
}

func TestGetBranchProtectionSurfacesForgejoMessage(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusForbidden, map[string]string{"message": "user should be an owner or a collaborator with admin write of a repository"}, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	_, _, err := GetBranchProtectionImpl{Client: cl}.Handler()(context.Background(), nil, GetBranchProtectionParams{Owner: "o", Repo: "r", Name: "main"})
	if err == nil || !strings.Contains(err.Error(), "admin write") || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected Forgejo's 403 message in the error, got %v", err)
	}
}

func TestDeleteBranchProtection(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusNoContent, nil, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	res, _, err := DeleteBranchProtectionImpl{Client: cl}.Handler()(context.Background(), nil, DeleteBranchProtectionParams{Owner: "o", Repo: "r", Name: "main"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.method != "DELETE" || rec.path != "/api/v1/repos/o/r/branch_protections/main" {
		t.Errorf("unexpected request %s %s", rec.method, rec.path)
	}
	if got := textOf(t, res); !strings.Contains(got, `Deleted branch protection "main"`) {
		t.Errorf("unexpected text: %q", got)
	}
}

func TestEditRepository(t *testing.T) {
	var rec recorded
	server := protectionServer(t, http.StatusOK, map[string]any{
		"full_name": "o/r", "default_branch": "dev", "private": true,
		"allow_merge_commits": true, "allow_squash_merge": true,
	}, &rec)
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())

	res, _, err := EditRepositoryImpl{Client: cl}.Handler()(context.Background(), nil, EditRepositoryParams{
		Owner: "o", Repo: "r",
		EditRepositoryOptions: tools.EditRepositoryOptions{DefaultBranch: ptr("dev"), AllowRebase: ptr(false)},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.method != "PATCH" || rec.path != "/api/v1/repos/o/r" {
		t.Errorf("unexpected request %s %s", rec.method, rec.path)
	}
	if len(rec.body) != 2 || rec.body["default_branch"] != "dev" || rec.body["allow_rebase"] != false {
		t.Errorf("expected only default_branch and allow_rebase, got %v", rec.body)
	}
	got := textOf(t, res)
	for _, want := range []string{"Updated o/r", "Default branch: dev", "private", "merge, squash"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestEditRepositoryRejectsEmptyPatch(t *testing.T) {
	cl, _ := tools.NewClient("http://127.0.0.1:1", "tok", testForgejoVersion, nil)
	_, _, err := EditRepositoryImpl{Client: cl}.Handler()(context.Background(), nil, EditRepositoryParams{Owner: "o", Repo: "r"})
	if err == nil || !strings.Contains(err.Error(), "no settings") {
		t.Fatalf("expected empty-patch error, got %v", err)
	}
}
