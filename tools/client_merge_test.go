// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIErrorMessage(t *testing.T) {
	cases := map[string]string{
		``:                                   "",
		`{}`:                                 "",
		`null`:                               "",
		`{"message":"m","url":"u"}`:          "m",
		`{"message":"m","errors":["e","m"]}`: "m: e",
		`plain text body`:                    "plain text body",
	}
	for in, want := range cases {
		if got := apiErrorMessage([]byte(in)); got != want {
			t.Errorf("apiErrorMessage(%q) = %q, want %q", in, got, want)
		}
	}
	long := strings.Repeat("x", 600)
	if got := apiErrorMessage([]byte(long)); len(got) > 510 || !strings.HasSuffix(got, "…") {
		t.Errorf("long body should be capped, got %d bytes", len(got))
	}
}

func TestSendSimpleRequestIncludesForgejoMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"branch protection already exist","url":"x"}`))
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL, "tok", "16.0.1+gitea-1.22.0", server.Client())

	var out map[string]any
	err := cl.sendSimpleRequest("POST", "/api/v1/x", map[string]string{}, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 422") || !strings.Contains(err.Error(), "branch protection already exist") {
		t.Errorf("error should carry status and Forgejo's message, got %q", err)
	}
}

func TestSendSimpleRequestNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL, "tok", "16.0.1+gitea-1.22.0", server.Client())
	var out map[string]any
	if err := cl.sendSimpleRequest("DELETE", "/api/v1/x", nil, &out); err != nil {
		t.Errorf("204 must not be a decode error, got %v", err)
	}
}
