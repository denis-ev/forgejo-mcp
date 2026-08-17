// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package help

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
	"github.com/raohwork/forgejo-mcp/types"
)

// HelpParams defines the parameters for the help tool.
type HelpParams struct {
	// Topic optionally narrows the output to a single domain (e.g. "issues",
	// "ci") or a single tool name (e.g. "get_ci_status"). Empty means the
	// full orientation document.
	Topic string `json:"topic,omitempty"`
}

// HelpImpl implements the read-only help tool. It answers "what can this
// server do and how do I call it correctly" without requiring the caller to
// probe tools by trial and error.
//
// The tool inventory is generated from the registry populated by
// tools.Register, so a newly added tool shows up here automatically.
type HelpImpl struct{}

// Definition describes the `help` tool. It takes an optional `topic` and is
// both read-only and idempotent: it touches no Forgejo API at all.
func (HelpImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:  "help",
		Title: "Help",
		Description: "Orientation document for this MCP server: tool inventory grouped by domain, ID conventions, " +
			"common multi-tool workflows, pagination rules, and version-gated tools. Call with no arguments first, " +
			"then call again with a topic (a domain such as 'issues' or 'ci', or a tool name such as 'get_ci_status') " +
			"for detail. Answers locally; it never contacts the Forgejo server.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"topic": {
					Type: "string",
					Description: "Domain or tool name to describe (optional). Domains: issues, labels, milestones, " +
						"pull_requests, repository, releases, wiki, actions, ci. Any registered tool name also works, " +
						"e.g. 'add_issue_labels'. Omit for the full orientation document.",
				},
			},
		},
	}
}

// Handler renders the requested help text. It never returns an error for an
// unknown topic: an agent that guesses wrong gets the list of valid topics
// back, which is more useful than a failed call.
func (impl HelpImpl) Handler() mcp.ToolHandlerFor[HelpParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args HelpParams) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: Render(args.Topic)},
			},
		}, nil, nil
	}
}

// Render produces the help document for a topic. An empty topic yields the
// overview. It is exported so tests, and any future non-MCP entry point, can
// render the same text the tool returns.
func Render(topic string) string {
	t := normalizeTopic(topic)
	if t == "" {
		return renderOverview()
	}

	if d, ok := lookupDomain(t); ok {
		return renderDomain(d)
	}

	if rec, ok := tools.LookupRegisteredTool(t); ok {
		return renderTool(rec)
	}

	return renderUnknownTopic(topic)
}

// normalizeTopic lowercases and trims a topic so that "Issues", " issues " and
// "issues" all resolve identically.
func normalizeTopic(topic string) string {
	return strings.ToLower(strings.TrimSpace(topic))
}

// renderOverview builds the no-argument orientation document.
func renderOverview() string {
	var b strings.Builder

	inv := groupByDomain(tools.RegisteredTools())

	b.WriteString("# Forgejo MCP Server\n\n")
	fmt.Fprintf(&b, "Server version: %s. Developed and tested against **Forgejo v16.0.1**; most tools work "+
		"against older Forgejo/Gitea servers, but see \"Version-gated tools\" below.\n\n", types.VERSION)
	b.WriteString("Every repository tool takes `owner` and `repo`. Successful responses are markdown; " +
		"errors are plain text.\n\n")

	fmt.Fprintf(&b, "## Tool inventory (%d tools)\n\n", countTools(inv))
	for _, d := range domainOrder {
		recs := inv[d.key]
		if len(recs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s (%d)\n\n%s\n\n", d.title, len(recs), d.summary)
		for _, rec := range recs {
			fmt.Fprintf(&b, "- `%s` — %s\n", rec.Definition.Name, firstSentence(rec.Definition.Description))
		}
		fmt.Fprintf(&b, "\nDetail: `help` with `topic: \"%s\"`\n\n", d.topic)
	}
	if others := inv[""]; len(others) > 0 {
		b.WriteString("### Other\n\n")
		for _, rec := range others {
			fmt.Fprintf(&b, "- `%s` — %s\n", rec.Definition.Name, firstSentence(rec.Definition.Description))
		}
		b.WriteString("\n")
	}

	b.WriteString(idConventions)
	b.WriteString(workflows)
	b.WriteString(paginationNotes)
	b.WriteString(versionGates)
	b.WriteString(knownGaps)

	b.WriteString("## Getting more detail\n\n")
	fmt.Fprintf(&b, "Call `help` again with a `topic`: a domain (%s) or any tool name, "+
		"e.g. `topic: \"add_issue_labels\"`.\n", strings.Join(domainTopics(), ", "))

	return b.String()
}

// renderDomain describes one domain: its tools with full descriptions and
// parameters, plus the notes that apply to the whole domain.
func renderDomain(d domain) string {
	var b strings.Builder

	recs := groupByDomain(tools.RegisteredTools())[d.key]

	fmt.Fprintf(&b, "# %s\n\n%s\n\n", d.title, d.summary)
	if d.notes != "" {
		b.WriteString(d.notes)
		b.WriteString("\n")
	}

	if len(recs) == 0 {
		b.WriteString("No tools are registered in this domain.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "## Tools (%d)\n\n", len(recs))
	for _, rec := range recs {
		fmt.Fprintf(&b, "### `%s`\n\n%s\n\n", rec.Definition.Name, rec.Definition.Description)
		b.WriteString(renderParams(rec.Definition))
		if note, ok := toolNotes[rec.Definition.Name]; ok {
			fmt.Fprintf(&b, "\n%s\n", note)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderTool describes a single tool in full: description, behavioural hints,
// and every parameter with type, requiredness, constraints and description.
func renderTool(rec tools.RegisteredTool) string {
	var b strings.Builder

	def := rec.Definition
	fmt.Fprintf(&b, "# `%s`\n\n%s\n\n", def.Name, def.Description)

	if hints := describeHints(def); hints != "" {
		fmt.Fprintf(&b, "Hints: %s\n\n", hints)
	}

	b.WriteString(renderParams(def))

	if note, ok := toolNotes[def.Name]; ok {
		fmt.Fprintf(&b, "\n## Notes\n\n%s\n", note)
	}

	if d, ok := domainByKey[rec.Domain]; ok {
		fmt.Fprintf(&b, "\nDomain: %s — `help` with `topic: \"%s\"`\n", d.title, d.topic)
	}

	return b.String()
}

// describeHints renders the MCP tool annotations that tell a caller whether
// the tool is safe to retry or safe to call speculatively.
func describeHints(def *mcp.Tool) string {
	if def.Annotations == nil {
		return ""
	}

	var hints []string
	if def.Annotations.ReadOnlyHint {
		hints = append(hints, "read-only")
	}
	if def.Annotations.IdempotentHint {
		hints = append(hints, "idempotent")
	}
	if def.Annotations.DestructiveHint != nil && *def.Annotations.DestructiveHint {
		hints = append(hints, "destructive")
	}
	return strings.Join(hints, ", ")
}

// renderParams formats a tool's input schema as a parameter list. Required
// parameters come first so a caller reads the mandatory arguments before the
// optional ones.
func renderParams(def *mcp.Tool) string {
	if def.InputSchema == nil || len(def.InputSchema.Properties) == 0 {
		return "Parameters: none.\n"
	}

	required := map[string]bool{}
	for _, name := range def.InputSchema.Required {
		required[name] = true
	}

	var req, opt []string
	for name := range def.InputSchema.Properties {
		if required[name] {
			req = append(req, name)
		} else {
			opt = append(opt, name)
		}
	}
	sort.Strings(req)
	sort.Strings(opt)

	var b strings.Builder
	b.WriteString("Parameters:\n\n")
	for _, name := range append(req, opt...) {
		s := def.InputSchema.Properties[name]
		fmt.Fprintf(&b, "- `%s` (%s)", name, describeType(s, required[name]))
		if s != nil && s.Description != "" {
			fmt.Fprintf(&b, ": %s", s.Description)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// describeType renders a parameter's type together with its requiredness and
// the constraints that most often cause a rejected call: enum values and
// numeric bounds.
func describeType(s *jsonschema.Schema, required bool) string {
	var parts []string

	if s == nil {
		parts = append(parts, "any")
	} else {
		t := s.Type
		if t == "" {
			t = "any"
		}
		if t == "array" && s.Items != nil && s.Items.Type != "" {
			t = s.Items.Type + "[]"
		}
		parts = append(parts, t)
	}

	if required {
		parts = append(parts, "required")
	} else {
		parts = append(parts, "optional")
	}

	if s != nil {
		if len(s.Enum) > 0 {
			vals := make([]string, 0, len(s.Enum))
			for _, v := range s.Enum {
				vals = append(vals, fmt.Sprintf("%v", v))
			}
			parts = append(parts, "one of: "+strings.Join(vals, ", "))
		}
		switch {
		case s.Minimum != nil && s.Maximum != nil:
			parts = append(parts, fmt.Sprintf("%g-%g", *s.Minimum, *s.Maximum))
		case s.Minimum != nil:
			parts = append(parts, fmt.Sprintf("min %g", *s.Minimum))
		case s.Maximum != nil:
			parts = append(parts, fmt.Sprintf("max %g", *s.Maximum))
		}
	}

	return strings.Join(parts, ", ")
}

// renderUnknownTopic answers an unrecognised topic with the valid choices
// instead of an error, so a wrong guess costs one call rather than a retry
// loop.
func renderUnknownTopic(topic string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Unknown help topic %q\n\n", topic)
	fmt.Fprintf(&b, "Valid domain topics: %s.\n\nOr pass any tool name:\n\n", strings.Join(domainTopics(), ", "))

	for _, rec := range tools.RegisteredTools() {
		fmt.Fprintf(&b, "- `%s`\n", rec.Definition.Name)
	}
	b.WriteString("\nCall `help` with no arguments for the full overview.\n")

	return b.String()
}

// groupByDomain buckets registered tools by domain key, preserving the
// name-sorted order produced by tools.RegisteredTools. Tools whose domain is
// not a documented one fall into the "" bucket so they are still listed.
func groupByDomain(recs []tools.RegisteredTool) map[string][]tools.RegisteredTool {
	out := map[string][]tools.RegisteredTool{}
	for _, rec := range recs {
		key := rec.Domain
		if _, known := domainByKey[key]; !known {
			key = ""
		}
		out[key] = append(out[key], rec)
	}
	return out
}

// countTools totals the tools across all buckets.
func countTools(inv map[string][]tools.RegisteredTool) int {
	n := 0
	for _, recs := range inv {
		n += len(recs)
	}
	return n
}

// firstSentence shortens a tool description to its first sentence for the
// compact inventory listing.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(no description)"
	}
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}
