// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerTestTool(t *testing.T) {
	t.Helper()
	registerSchemaFields(&mcp.Tool{
		Name: "merge_pull_request",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner":                     {Type: "string"},
				"repo":                      {Type: "string"},
				"index":                     {Type: "integer"},
				"style":                     {Type: "string"},
				"title":                     {Type: "string"},
				"message":                   {Type: "string"},
				"delete_branch_after_merge": {Type: "boolean"},
				"merge_when_checks_succeed": {Type: "boolean"},
			},
			Required: []string{"owner", "repo", "index"},
		},
	})
}

func TestEnrichUnknownFieldError(t *testing.T) {
	registerTestTool(t)

	t.Run("suggests_style_for_do", func(t *testing.T) {
		// The exact failure reported in issue #5.
		in := errors.New(`unmarshaling: json: unknown field "do"`)
		got := enrichUnknownFieldError("merge_pull_request", in).Error()

		if !strings.Contains(got, `unknown field "do"`) {
			t.Errorf("expected rejected field in message, got %q", got)
		}
		if !strings.Contains(got, "merge_pull_request") {
			t.Errorf("expected tool name in message, got %q", got)
		}
		if !strings.Contains(got, `did you mean "style"`) {
			t.Errorf("expected style suggestion, got %q", got)
		}
		if !strings.Contains(got, "expected one of:") {
			t.Errorf("expected accepted field list, got %q", got)
		}
		for _, f := range []string{"owner", "repo", "index", "style", "title", "message"} {
			if !strings.Contains(got, f) {
				t.Errorf("expected field %q in list, got %q", f, got)
			}
		}
	})

	t.Run("required_fields_listed_first", func(t *testing.T) {
		got := enrichUnknownFieldError("merge_pull_request", errors.New(`unknown field "bogus"`)).Error()
		list := got[strings.Index(got, "expected one of:"):]
		if !strings.HasPrefix(list, "expected one of: index, owner, repo,") {
			t.Errorf("expected required fields first, got %q", list)
		}
	})

	t.Run("fuzzy_match_typo", func(t *testing.T) {
		got := enrichUnknownFieldError("merge_pull_request", errors.New(`unknown field "titel"`)).Error()
		if !strings.Contains(got, `did you mean "title"`) {
			t.Errorf("expected title suggestion, got %q", got)
		}
	})

	t.Run("no_suggestion_when_unrelated", func(t *testing.T) {
		got := enrichUnknownFieldError("merge_pull_request", errors.New(`unknown field "quantum_flux_capacitor"`)).Error()
		if strings.Contains(got, "did you mean") {
			t.Errorf("expected no suggestion for unrelated field, got %q", got)
		}
		if !strings.Contains(got, "expected one of:") {
			t.Errorf("expected field list, got %q", got)
		}
	})

	t.Run("passes_through_other_errors", func(t *testing.T) {
		in := errors.New("connection reset by peer")
		if got := enrichUnknownFieldError("merge_pull_request", in); got != in {
			t.Errorf("expected original error, got %v", got)
		}
	})

	t.Run("passes_through_nil", func(t *testing.T) {
		if got := enrichUnknownFieldError("merge_pull_request", nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("unknown_tool_left_alone", func(t *testing.T) {
		in := errors.New(`unknown field "do"`)
		if got := enrichUnknownFieldError("never_registered", in); got != in {
			t.Errorf("expected original error for unindexed tool, got %v", got)
		}
	})

	t.Run("alias_not_suggested_when_absent_from_schema", func(t *testing.T) {
		registerSchemaFields(&mcp.Tool{
			Name: "no_style_tool",
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{"owner": {Type: "string"}},
			},
		})
		got := enrichUnknownFieldError("no_style_tool", errors.New(`unknown field "do"`)).Error()
		if strings.Contains(got, "did you mean") {
			t.Errorf("expected no alias suggestion when target absent, got %q", got)
		}
	})
}

func TestRegisterSchemaFields_NilSafe(t *testing.T) {
	registerSchemaFields(nil)
	registerSchemaFields(&mcp.Tool{Name: "no_schema"})
	if f := acceptedFields("no_schema"); len(f) != 0 {
		t.Errorf("expected no fields, got %v", f)
	}
}

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"titel", "title", 2},
		{"do", "style", 5},
		{"", "abc", 3},
	}
	for _, c := range cases {
		if got := editDistance(c.a, c.b); got != c.want {
			t.Errorf("editDistance(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
