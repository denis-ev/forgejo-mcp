// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisteredTool is the record kept for every tool handed to Register.
//
// It exists so that server-side introspection (the help tool) can describe the
// tool set that is actually registered, instead of a hand-written copy of it
// that silently rots as tools are added.
type RegisteredTool struct {
	// Definition is the tool definition exactly as registered with the MCP
	// server, including its description and full input schema.
	Definition *mcp.Tool

	// Domain is the short grouping name of the tool, derived from the package
	// its implementation lives in ("issue", "pullreq", "repo", ...). Deriving
	// it from the implementation type means a new tool lands in the right
	// group automatically, with no list to keep in sync.
	Domain string
}

// toolRegistry indexes every registered tool by name. It is written once per
// tool at registration time and read afterwards, so a plain RWMutex is enough.
var toolRegistry = struct {
	sync.RWMutex
	byName map[string]RegisteredTool
}{byName: map[string]RegisteredTool{}}

// registerToolMetadata records a tool definition together with the domain
// inferred from its implementation. Re-registering the same name overwrites
// the previous record, which keeps tests that build several servers stable.
func registerToolMetadata(def *mcp.Tool, impl any) {
	if def == nil || def.Name == "" {
		return
	}

	rec := RegisteredTool{Definition: def, Domain: domainOf(impl)}

	toolRegistry.Lock()
	defer toolRegistry.Unlock()
	toolRegistry.byName[def.Name] = rec
}

// domainOf returns the last path segment of the package that declares impl,
// e.g. "issue" for *issue.GetIssueImpl. It returns an empty string when the
// package cannot be determined, which callers treat as "ungrouped".
func domainOf(impl any) string {
	t := reflect.TypeOf(impl)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return ""
	}

	pkg := t.PkgPath()
	if pkg == "" {
		return ""
	}
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		return pkg[i+1:]
	}
	return pkg
}

// RegisteredTools returns every registered tool, sorted by tool name so that
// generated documentation is deterministic.
func RegisteredTools() []RegisteredTool {
	toolRegistry.RLock()
	defer toolRegistry.RUnlock()

	out := make([]RegisteredTool, 0, len(toolRegistry.byName))
	for _, rec := range toolRegistry.byName {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Definition.Name < out[j].Definition.Name
	})
	return out
}

// LookupRegisteredTool returns the record for a single tool name.
func LookupRegisteredTool(name string) (RegisteredTool, bool) {
	toolRegistry.RLock()
	defer toolRegistry.RUnlock()

	rec, ok := toolRegistry.byName[name]
	return rec, ok
}
