// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package help

import (
	"context"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
	"github.com/raohwork/forgejo-mcp/tools/action"
	"github.com/raohwork/forgejo-mcp/tools/issue"
	"github.com/raohwork/forgejo-mcp/tools/label"
	"github.com/raohwork/forgejo-mcp/tools/pullreq"
	"github.com/raohwork/forgejo-mcp/tools/repo"
)

// registerSample registers a representative slice of the real tool set so the
// generated inventory is exercised against genuine definitions rather than
// stubs. It deliberately spans several domains.
func registerSample(t *testing.T) {
	t.Helper()

	s := mcp.NewServer(&mcp.Implementation{Title: "test", Version: "0.0.0"}, nil)
	tools.Register(s, &HelpImpl{})
	tools.Register(s, &issue.ListRepoIssuesImpl{})
	tools.Register(s, &issue.AddIssueLabelsImpl{})
	tools.Register(s, &label.ListRepoLabelsImpl{})
	tools.Register(s, &pullreq.MergePullRequestImpl{})
	tools.Register(s, &repo.GetCIStatusImpl{})
	tools.Register(s, &action.GetActionJobLogsImpl{})
	tools.Register(s, &action.ListActionRunJobsImpl{})
}

func TestRenderOverview(t *testing.T) {
	registerSample(t)

	got := Render("")

	t.Run("lists_registered_tools", func(t *testing.T) {
		// The inventory is generated, so every registered tool must appear
		// without anyone maintaining a list.
		for _, name := range []string{
			"help",
			"list_repo_issues",
			"add_issue_labels",
			"list_repo_labels",
			"merge_pull_request",
			"get_ci_status",
			"get_action_job_logs",
		} {
			if !strings.Contains(got, name) {
				t.Errorf("expected %q in overview", name)
			}
		}
	})

	t.Run("groups_by_domain", func(t *testing.T) {
		for _, heading := range []string{"Issues", "Labels", "Pull requests", "Repository", "Actions"} {
			if !strings.Contains(got, heading) {
				t.Errorf("expected %q section in overview", heading)
			}
		}
	})

	t.Run("covers_id_conventions", func(t *testing.T) {
		for _, want := range []string{"ID conventions", "index", "database ID", "label gotcha"} {
			if !strings.Contains(got, want) {
				t.Errorf("expected %q in overview", want)
			}
		}
	})

	t.Run("covers_ci_triage_workflow", func(t *testing.T) {
		// The acceptance criterion of issue #11: a single help call has to
		// spell out the CI triage sequence.
		if !strings.Contains(got, "Triage a CI failure") {
			t.Fatal("expected CI triage workflow in overview")
		}
		iStatus := strings.Index(got, "get_ci_status")
		iLogs := strings.Index(got, "get_action_job_logs")
		iComment := strings.Index(got, "create_issue_comment")
		if iStatus < 0 || iLogs < 0 || iComment < 0 {
			t.Fatalf("expected the whole CI triage chain, got indices %d/%d/%d", iStatus, iLogs, iComment)
		}
	})

	t.Run("covers_pagination_and_version_gates", func(t *testing.T) {
		for _, want := range []string{"Pagination", "limit", "Version-gated tools", "v16.0.1", "404"} {
			if !strings.Contains(got, want) {
				t.Errorf("expected %q in overview", want)
			}
		}
	})

	t.Run("covers_known_gaps", func(t *testing.T) {
		if !strings.Contains(got, "Known gaps") {
			t.Error("expected known gaps section in overview")
		}
	})
}

func TestRenderDomainTopic(t *testing.T) {
	registerSample(t)

	cases := []struct {
		name  string
		topic string
		want  []string
	}{
		{
			name:  "issues",
			topic: "issues",
			want:  []string{"# Issues", "list_repo_issues", "add_issue_labels", "list_repo_labels"},
		},
		{
			name:  "alias_pr_resolves_to_pull_requests",
			topic: "pr",
			want:  []string{"# Pull requests", "merge_pull_request", "style"},
		},
		{
			name:  "alias_ci_resolves_to_actions",
			topic: "ci",
			want:  []string{"# Actions", "get_action_job_logs", "v16.0.1"},
		},
		{
			name:  "case_and_space_insensitive",
			topic: "  Issues ",
			want:  []string{"# Issues"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Render(c.topic)
			for _, want := range c.want {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in help for topic %q", want, c.topic)
				}
			}
		})
	}
}

func TestRenderToolTopic(t *testing.T) {
	registerSample(t)

	t.Run("describes_parameters", func(t *testing.T) {
		got := Render("add_issue_labels")

		for _, want := range []string{
			"# " + "`add_issue_labels`",
			"owner",
			"required",
			"labels",
			"integer[]",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("expected %q in tool help, got:\n%s", want, got)
			}
		}
	})

	t.Run("warns_about_label_ids", func(t *testing.T) {
		got := Render("add_issue_labels")
		if !strings.Contains(got, "list_repo_labels") {
			t.Error("expected the label ID resolution note")
		}
	})

	t.Run("reports_hints", func(t *testing.T) {
		got := Render("get_ci_status")
		if !strings.Contains(got, "read-only") || !strings.Contains(got, "idempotent") {
			t.Errorf("expected annotation hints, got:\n%s", got)
		}
	})

	t.Run("reports_enum_and_bounds", func(t *testing.T) {
		got := Render("list_repo_issues")
		if !strings.Contains(got, "one of: open, closed, all") {
			t.Error("expected enum values for state")
		}
		if !strings.Contains(got, "1-50") {
			t.Error("expected numeric bounds for limit")
		}
	})

	t.Run("names_owning_domain", func(t *testing.T) {
		got := Render("merge_pull_request")
		if !strings.Contains(got, "Pull requests") {
			t.Error("expected the tool's domain to be named")
		}
	})
}

func TestRenderUnknownTopic(t *testing.T) {
	registerSample(t)

	got := Render("bananas")

	if !strings.Contains(got, "Unknown help topic") {
		t.Errorf("expected an unknown-topic answer, got:\n%s", got)
	}
	// The point is to recover in one call, so the valid choices must be listed.
	if !strings.Contains(got, "issues") || !strings.Contains(got, "get_ci_status") {
		t.Error("expected valid domains and tool names to be listed")
	}
}

func TestHandlerReturnsMarkdown(t *testing.T) {
	registerSample(t)

	impl := HelpImpl{}
	res, _, err := impl.Handler()(context.Background(), nil, HelpParams{})
	if err != nil {
		t.Fatalf("help must not fail: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("expected a single content block, got %d", len(res.Content))
	}

	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if !strings.HasPrefix(text.Text, "# Forgejo MCP Server") {
		t.Errorf("expected the overview document, got %q", firstLine(text.Text))
	}
}

func TestDefinitionMatchesHouseStyle(t *testing.T) {
	def := HelpImpl{}.Definition()

	if def.Name != "help" || def.Title == "" || def.Description == "" {
		t.Errorf("expected name, title and description to be set, got %+v", def)
	}
	if def.Annotations == nil || !def.Annotations.ReadOnlyHint || !def.Annotations.IdempotentHint {
		t.Error("expected read-only and idempotent hints")
	}
	if def.InputSchema == nil {
		t.Fatal("expected an input schema")
	}
	schema, ok := def.InputSchema.(*jsonschema.Schema)
	if !ok || schema == nil {
		t.Fatalf("expected *jsonschema.Schema, got %T", def.InputSchema)
	}
	if len(schema.Required) != 0 {
		t.Errorf("topic must stay optional, got required %v", schema.Required)
	}
	topic, ok := schema.Properties["topic"]
	if !ok {
		t.Fatal("expected a topic property")
	}
	if topic.Description == "" {
		t.Error("every field needs a description")
	}
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
