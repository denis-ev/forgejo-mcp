// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/raohwork/forgejo-mcp/tools"
)

// capturedRequest records what a write tool actually sent, so the assertions
// can check the wire format rather than only the rendered output.
type capturedRequest struct {
	method string
	path   string
	body   map[string]any
}

// writeServer serves a single content-writing endpoint, records the request and
// replies with resp. A nil resp means "204 No Content", as DELETE returns.
func writeServer(t *testing.T, capture *capturedRequest, status int, resp any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		capture.method = r.Method
		capture.path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &capture.body); err != nil {
				t.Errorf("request body is not JSON: %v (%s)", err, raw)
			}
		}
		w.WriteHeader(status)
		if resp != nil {
			json.NewEncoder(w).Encode(resp)
		}
	}))
}

// decodedContent pulls the base64 "content" field back out of a captured body.
func decodedContent(t *testing.T, body map[string]any) string {
	t.Helper()
	enc, ok := body["content"].(string)
	if !ok {
		t.Fatalf("no string content field in body: %#v", body)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("content is not base64 (%q): %v", enc, err)
	}
	return string(raw)
}

func fileResponseJSON(commitSHA, contentSHA string) map[string]any {
	return map[string]any{
		"content": map[string]any{"path": "docs/new.md", "sha": contentSHA, "type": "file"},
		"commit": map[string]any{
			"sha":      commitSHA,
			"html_url": "https://forge.example/o/r/commit/" + commitSHA,
			"message":  "add docs",
		},
	}
}

func TestCreateFile(t *testing.T) {
	var got capturedRequest
	server := writeServer(t, &got, http.StatusCreated, fileResponseJSON("c0ffee0000000000000000000000000000000000", "b10b0000000000000000000000000000000000ab"))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := CreateFileImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, CreateFileParams{
		Owner: "o", Repo: "r", Path: "docs/new.md",
		Content: "# Title\nplain text\n",
		Message: "add docs", Branch: "main", NewBranch: "feat/docs",
		AuthorName: "Ada", AuthorEmail: "ada@example.com",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if got.method != http.MethodPost {
		t.Errorf("expected POST, got %s", got.method)
	}
	if got.path != "/api/v1/repos/o/r/contents/docs/new.md" {
		t.Errorf("unexpected path: %s", got.path)
	}
	if content := decodedContent(t, got.body); content != "# Title\nplain text\n" {
		t.Errorf("expected plain text to be base64-encoded verbatim, got %q", content)
	}
	if got.body["message"] != "add docs" {
		t.Errorf("expected commit message, got %#v", got.body["message"])
	}
	if got.body["branch"] != "main" || got.body["new_branch"] != "feat/docs" {
		t.Errorf("expected branch and new_branch to be forwarded, got %#v / %#v", got.body["branch"], got.body["new_branch"])
	}
	author, _ := got.body["author"].(map[string]any)
	if author == nil || author["name"] != "Ada" || author["email"] != "ada@example.com" {
		t.Errorf("expected author identity, got %#v", got.body["author"])
	}

	out := textOf(t, res)
	for _, want := range []string{"Created", "docs/new.md", "feat/docs", "c0ffee0000000000000000000000000000000000", "b10b0000000000000000000000000000000000ab"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
}

func TestUpdateFile(t *testing.T) {
	var got capturedRequest
	server := writeServer(t, &got, http.StatusOK, fileResponseJSON("deadbeef00000000000000000000000000000000", "newb10b0000000000000000000000000000000000"))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := UpdateFileImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, UpdateFileParams{
		Owner: "o", Repo: "r", Path: "docs/new.md", Content: "replaced\n",
		SHA: "b10b0000000000000000000000000000000000ab", FromPath: "docs/old.md",
		Message: "move and rewrite",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if got.method != http.MethodPut {
		t.Errorf("expected PUT, got %s", got.method)
	}
	if got.body["sha"] != "b10b0000000000000000000000000000000000ab" {
		t.Errorf("expected the blob sha to be forwarded, got %#v", got.body["sha"])
	}
	if got.body["from_path"] != "docs/old.md" {
		t.Errorf("expected from_path to be forwarded, got %#v", got.body["from_path"])
	}
	if content := decodedContent(t, got.body); content != "replaced\n" {
		t.Errorf("unexpected content: %q", content)
	}

	out := textOf(t, res)
	if !strings.Contains(out, "moved from") || !strings.Contains(out, "docs/old.md") {
		t.Errorf("expected the move to be reported, got: %q", out)
	}
	if !strings.Contains(out, "newb10b0000000000000000000000000000000000") {
		t.Errorf("expected the new blob sha for the next write, got: %q", out)
	}
}

func TestDeleteFile(t *testing.T) {
	var got capturedRequest
	server := writeServer(t, &got, http.StatusOK, map[string]any{
		"commit": map[string]any{"sha": "abc1230000000000000000000000000000000000"},
	})
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := DeleteFileImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, DeleteFileParams{
		Owner: "o", Repo: "r", Path: "docs/old.md",
		SHA: "b10b0000000000000000000000000000000000ab", Message: "drop stale doc", Branch: "main",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if got.method != http.MethodDelete {
		t.Errorf("expected DELETE, got %s", got.method)
	}
	if got.path != "/api/v1/repos/o/r/contents/docs/old.md" {
		t.Errorf("unexpected path: %s", got.path)
	}
	if got.body["sha"] != "b10b0000000000000000000000000000000000ab" {
		t.Errorf("expected the blob sha to be forwarded, got %#v", got.body["sha"])
	}

	out := textOf(t, res)
	if !strings.Contains(out, "Deleted") || !strings.Contains(out, "docs/old.md") || !strings.Contains(out, "main") {
		t.Errorf("unexpected output: %q", out)
	}
}

// TestWriteToolsRejectMissingSHA proves the sha requirement is enforced before
// any request is issued: the server fails the test if it is ever reached.
func TestWriteToolsRejectMissingSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		t.Errorf("no request should be sent, got %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())

	if _, _, err := (UpdateFileImpl{Client: cl}).Handler()(context.Background(), nil, UpdateFileParams{
		Owner: "o", Repo: "r", Path: "a.md", Content: "x", Message: "m",
	}); err == nil || !strings.Contains(err.Error(), "get_file_contents") {
		t.Errorf("expected update_file to demand a sha and name the source tool, got: %v", err)
	}

	if _, _, err := (DeleteFileImpl{Client: cl}).Handler()(context.Background(), nil, DeleteFileParams{
		Owner: "o", Repo: "r", Path: "a.md", Message: "m",
	}); err == nil || !strings.Contains(err.Error(), "get_file_contents") {
		t.Errorf("expected delete_file to demand a sha and name the source tool, got: %v", err)
	}

	if _, _, err := (CreateFileImpl{Client: cl}).Handler()(context.Background(), nil, CreateFileParams{
		Owner: "o", Repo: "r", Path: "   ", Content: "x", Message: "m",
	}); err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Errorf("expected create_file to reject a blank path, got: %v", err)
	}
}

func TestPushFiles(t *testing.T) {
	var got capturedRequest
	server := writeServer(t, &got, http.StatusCreated, map[string]any{
		"commit": map[string]any{
			"sha":      "f00d000000000000000000000000000000000000",
			"html_url": "https://forge.example/o/r/commit/f00d",
		},
		"files": []map[string]any{
			{"path": "a.md", "sha": "aaa0000000000000000000000000000000000000", "type": "file"},
			{"path": "b.md", "sha": "bbb0000000000000000000000000000000000000", "type": "file"},
		},
	})
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	impl := PushFilesImpl{Client: cl}
	res, _, err := impl.Handler()(context.Background(), nil, PushFilesParams{
		Owner: "o", Repo: "r", Message: "one atomic change", NewBranch: "feat/batch",
		Files: []PushFileParams{
			{Operation: "create", Path: "a.md", Content: "alpha\n"},
			{Operation: "UPDATE", Path: "b.md", Content: "beta\n", SHA: "bbb1110000000000000000000000000000000000"},
			{Operation: "delete", Path: "c.md", SHA: "ccc1110000000000000000000000000000000000"},
		},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if got.method != http.MethodPost || got.path != "/api/v1/repos/o/r/contents" {
		t.Errorf("expected POST on the multi-file endpoint, got %s %s", got.method, got.path)
	}
	files, _ := got.body["files"].([]any)
	if len(files) != 3 {
		t.Fatalf("expected 3 operations, got %#v", got.body["files"])
	}
	first, _ := files[0].(map[string]any)
	if first["operation"] != "create" {
		t.Errorf("expected create operation, got %#v", first["operation"])
	}
	if raw, _ := base64.StdEncoding.DecodeString(first["content"].(string)); string(raw) != "alpha\n" {
		t.Errorf("expected plain text to be encoded, got %q", raw)
	}
	second, _ := files[1].(map[string]any)
	if second["operation"] != "update" {
		t.Errorf("expected the operation to be normalized to lower case, got %#v", second["operation"])
	}
	third, _ := files[2].(map[string]any)
	if _, hasContent := third["content"]; hasContent {
		t.Errorf("a delete must not carry content, got %#v", third)
	}
	if got.body["new_branch"] != "feat/batch" {
		t.Errorf("expected new_branch to be forwarded, got %#v", got.body["new_branch"])
	}
	if _, hasAuthor := got.body["author"]; hasAuthor {
		t.Errorf("an unset author must be omitted, got %#v", got.body["author"])
	}

	out := textOf(t, res)
	for _, want := range []string{"3 file operations", "create", "update", "delete", "f00d000000000000000000000000000000000000", "aaa0000000000000000000000000000000000000"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
}

func TestBuildFileOperations_Validation(t *testing.T) {
	tests := []struct {
		name  string
		files []PushFileParams
		want  string
	}{
		{
			name:  "empty list",
			files: nil,
			want:  "at least one operation",
		},
		{
			name:  "unknown operation",
			files: []PushFileParams{{Operation: "rename", Path: "a.md"}},
			want:  "expected one of: create, update, delete",
		},
		{
			name:  "update without sha",
			files: []PushFileParams{{Operation: "update", Path: "a.md", Content: "x"}},
			want:  "sha is required for an update",
		},
		{
			name:  "delete without sha",
			files: []PushFileParams{{Operation: "create", Path: "a.md"}, {Operation: "delete", Path: "b.md"}},
			want:  "files[1]",
		},
		{
			name:  "blank path",
			files: []PushFileParams{{Operation: "create", Path: " "}},
			want:  "path is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildFileOperations(tc.files)
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestBuildFileOperations_EncodesAndTrims(t *testing.T) {
	ops, err := buildFileOperations([]PushFileParams{
		{Operation: " Create ", Path: "a.md", Content: "alpha\n"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops) != 1 || ops[0].Operation != "create" {
		t.Fatalf("expected one normalized create operation, got %#v", ops)
	}
	if ops[0].ContentBase64 != base64.StdEncoding.EncodeToString([]byte("alpha\n")) {
		t.Errorf("expected base64-encoded content, got %q", ops[0].ContentBase64)
	}
}

func TestWriteErrorHint(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"stale sha", errors.New("422 Unprocessable Entity: sha does not match"), "get_file_contents"},
		{"exists", errors.New("422: file already exists [path: a.md]"), "update_file"},
		{"protected", errors.New("403 Forbidden: branch is protected"), "new_branch"},
		{"not found", errors.New("HTTP 404: Not Found"), "check owner/repo"},
		{"unknown", errors.New("500 Internal Server Error"), ""},
		{"nil", nil, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := writeErrorHint(tc.err)
			if tc.want == "" {
				if got != "" {
					t.Errorf("expected no hint, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("expected hint mentioning %q, got %q", tc.want, got)
			}
		})
	}
}

// TestCreateFile_ErrorCarriesHint checks the hint reaches the caller, not just
// the helper: a duplicate path must point at update_file.
func TestCreateFile_ErrorCarriesHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"repository file already exists [path: a.md]"}`))
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	_, _, err := (CreateFileImpl{Client: cl}).Handler()(context.Background(), nil, CreateFileParams{
		Owner: "o", Repo: "r", Path: "a.md", Content: "x", Message: "m",
	})
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !strings.Contains(err.Error(), "update_file") {
		t.Errorf("expected the hint to name update_file, got: %v", err)
	}
}

// TestGetFileContents_ReportsFullSHA guards the read-then-write path: the write
// tools reject an abbreviated sha, so the reader must print the whole thing.
func TestGetFileContents_ReportsFullSHA(t *testing.T) {
	const fullSHA = "0123456789abcdef0123456789abcdef01234567"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if versionHandler(w, r) {
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"name": "a.md", "path": "a.md", "sha": fullSHA, "type": "file",
			"size": 2, "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("hi")),
		})
	}))
	defer server.Close()

	cl, _ := tools.NewClient(server.URL, "tok", testForgejoVersion, server.Client())
	res, _, err := (GetFileContentsImpl{Client: cl}).Handler()(context.Background(), nil, GetFileContentsParams{
		Owner: "o", Repo: "r", Path: "a.md",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !strings.Contains(textOf(t, res), fullSHA) {
		t.Errorf("expected the full blob sha in the output, got: %q", textOf(t, res))
	}
}
