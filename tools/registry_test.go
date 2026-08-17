// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeImpl stands in for a real tool implementation. It lives in package
// tools, so domainOf must report "tools" for it.
type fakeImpl struct{}

func newRegistryTool(name string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: "test tool",
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{"owner": {Type: "string"}},
			Required:   []string{"owner"},
		},
	}
}

func TestRegisterToolMetadata(t *testing.T) {
	t.Run("records_definition_and_domain", func(t *testing.T) {
		registerToolMetadata(newRegistryTool("registry_test_alpha"), &fakeImpl{})

		rec, ok := LookupRegisteredTool("registry_test_alpha")
		if !ok {
			t.Fatal("expected tool to be registered")
		}
		if rec.Definition == nil || rec.Definition.Name != "registry_test_alpha" {
			t.Errorf("expected definition to be kept, got %+v", rec.Definition)
		}
		if rec.Domain != "tools" {
			t.Errorf("expected domain derived from package, got %q", rec.Domain)
		}
	})

	t.Run("ignores_nil_and_unnamed", func(t *testing.T) {
		// Neither must panic nor add an entry.
		registerToolMetadata(nil, &fakeImpl{})
		registerToolMetadata(newRegistryTool(""), &fakeImpl{})

		if _, ok := LookupRegisteredTool(""); ok {
			t.Error("expected unnamed tool to be skipped")
		}
	})

	t.Run("lookup_reports_missing", func(t *testing.T) {
		if _, ok := LookupRegisteredTool("registry_test_absent"); ok {
			t.Error("expected unknown tool to be reported as missing")
		}
	})

	t.Run("registered_tools_sorted_by_name", func(t *testing.T) {
		registerToolMetadata(newRegistryTool("registry_test_zulu"), &fakeImpl{})
		registerToolMetadata(newRegistryTool("registry_test_bravo"), &fakeImpl{})

		all := RegisteredTools()
		prev := ""
		for _, rec := range all {
			if prev != "" && rec.Definition.Name < prev {
				t.Fatalf("expected name-sorted output, %q came after %q", rec.Definition.Name, prev)
			}
			prev = rec.Definition.Name
		}
	})

	t.Run("re_registration_overwrites", func(t *testing.T) {
		registerToolMetadata(newRegistryTool("registry_test_dup"), &fakeImpl{})
		registerToolMetadata(newRegistryTool("registry_test_dup"), &fakeImpl{})

		n := 0
		for _, rec := range RegisteredTools() {
			if rec.Definition.Name == "registry_test_dup" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("expected a single entry per tool name, got %d", n)
		}
	})
}

func TestDomainOf(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"pointer_to_struct", &fakeImpl{}, "tools"},
		{"value_struct", fakeImpl{}, "tools"},
		{"nil_any", nil, ""},
		{"builtin_type_has_no_package", 42, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domainOf(c.in); got != c.want {
				t.Errorf("domainOf(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestRegisterIndexesMetadata verifies the wiring in Register: registering a
// tool with a real server must make it discoverable through the registry.
func TestRegisterIndexesMetadata(t *testing.T) {
	s := mcp.NewServer(&mcp.Implementation{Title: "test", Version: "0.0.0"}, nil)
	Register(s, registryTestImpl{})

	rec, ok := LookupRegisteredTool("registry_test_via_register")
	if !ok {
		t.Fatal("expected Register to index the tool")
	}
	if rec.Domain != "tools" {
		t.Errorf("expected domain %q, got %q", "tools", rec.Domain)
	}
	// Register must keep indexing accepted field names as well.
	if fields := acceptedFields("registry_test_via_register"); len(fields) == 0 {
		t.Error("expected schema fields to stay indexed")
	}
}

// registryTestImpl is a minimal ToolImpl for exercising Register end to end.
type registryTestImpl struct{}

func (registryTestImpl) Definition() *mcp.Tool {
	return newRegistryTool("registry_test_via_register")
}

func (registryTestImpl) Handler() mcp.ToolHandlerFor[struct{}, any] {
	return nil
}
