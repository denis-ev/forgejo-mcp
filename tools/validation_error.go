// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// schemaRegistry records the accepted input field names for every registered
// tool, keyed by tool name. The MCP SDK rejects unknown fields inside
// unmarshalSchema, before our handler is ever invoked, so the schema is no
// longer in scope at the point the error surfaces. Keeping this small index at
// registration time is what lets the middleware below name the accepted
// fields.
var schemaRegistry = struct {
	sync.RWMutex
	fields map[string][]string
}{fields: map[string][]string{}}

// registerSchemaFields indexes the accepted field names of a tool definition.
// Required fields are listed first, each group sorted alphabetically, so the
// resulting error messages are stable and lead with the fields a caller must
// supply.
func registerSchemaFields(t *mcp.Tool) {
	if t == nil || t.InputSchema == nil {
		return
	}

	required := map[string]bool{}
	for _, name := range t.InputSchema.Required {
		required[name] = true
	}

	var req, opt []string
	for name := range t.InputSchema.Properties {
		if required[name] {
			req = append(req, name)
		} else {
			opt = append(opt, name)
		}
	}
	sort.Strings(req)
	sort.Strings(opt)

	schemaRegistry.Lock()
	defer schemaRegistry.Unlock()
	schemaRegistry.fields[t.Name] = append(req, opt...)
}

// acceptedFields returns the indexed field names for a tool.
func acceptedFields(tool string) []string {
	schemaRegistry.RLock()
	defer schemaRegistry.RUnlock()
	return schemaRegistry.fields[tool]
}

// knownAliases maps field names from the Forgejo/GitHub REST API to the
// equivalent parameter in this server, for cases where the names deliberately
// differ. A suggestion is only emitted if the target actually exists in the
// tool's schema, so an entry can never invent a field for a tool that lacks
// it.
var knownAliases = map[string]string{
	"do":          "style",
	"merge_style": "style",
	"pull_number": "index",
	"number":      "index",
	"body":        "message",
	"per_page":    "limit",
	"page_size":   "limit",
	"state":       "status",
}

// unknownFieldRe matches the encoding/json error text produced when a decoder
// configured with DisallowUnknownFields encounters an unexpected key.
var unknownFieldRe = regexp.MustCompile(`unknown field "([^"]+)"`)

// enrichUnknownFieldError rewrites the SDK's terminal `unknown field "x"`
// error into one that also names the accepted fields, and suggests a specific
// replacement when the rejected name is a known REST alias or a near miss.
//
// It returns err unchanged when the error is not an unknown-field error or
// when the tool's schema was never indexed, so unrelated failures pass through
// untouched.
func enrichUnknownFieldError(tool string, err error) error {
	if err == nil {
		return nil
	}

	m := unknownFieldRe.FindStringSubmatch(err.Error())
	if m == nil {
		return err
	}
	bad := m[1]

	fields := acceptedFields(tool)
	if len(fields) == 0 {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "unknown field %q", bad)
	if tool != "" {
		fmt.Fprintf(&b, " for %s", tool)
	}

	if hint := suggestField(bad, fields); hint != "" {
		fmt.Fprintf(&b, " — did you mean %q?", hint)
	}

	fmt.Fprintf(&b, "; expected one of: %s", strings.Join(fields, ", "))

	return fmt.Errorf("%s", b.String())
}

// suggestField picks the most plausible replacement for a rejected field name.
// An explicit alias wins over a fuzzy match; fuzzy matches are only accepted
// when they are close enough to be worth printing.
func suggestField(bad string, fields []string) string {
	has := func(name string) bool {
		for _, f := range fields {
			if f == name {
				return true
			}
		}
		return false
	}

	if alias, ok := knownAliases[strings.ToLower(bad)]; ok && has(alias) {
		return alias
	}

	// Fall back to nearest neighbour by edit distance. The threshold scales
	// with the length of the input so that short names do not match everything.
	best, bestDist := "", -1
	limit := len(bad)/2 + 1
	if limit > 3 {
		limit = 3
	}
	for _, f := range fields {
		d := editDistance(strings.ToLower(bad), strings.ToLower(f))
		if d > limit {
			continue
		}
		if bestDist == -1 || d < bestDist {
			best, bestDist = f, d
		}
	}
	return best
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

// ValidationErrorMiddleware returns MCP receiving middleware that enriches
// schema validation failures for tools/call.
//
// The unknown-field rejection happens inside the SDK's own argument decoding,
// upstream of any tool handler, so this is the earliest point in our own code
// where the error can be observed and rewritten.
func ValidationErrorMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if err == nil || method != "tools/call" {
				return res, err
			}

			params, ok := req.GetParams().(*mcp.CallToolParamsRaw)
			if !ok {
				return res, err
			}

			return res, enrichUnknownFieldError(params.Name, err)
		}
	}
}
