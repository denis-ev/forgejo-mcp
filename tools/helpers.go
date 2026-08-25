// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"fmt"
	"time"
)

// Helper functions for creating pointers to basic types, primarily for use in
// constructing jsonschema.Schema objects where optional fields require pointers.

// BoolPtr creates a pointer to a bool value.
// This is useful for setting optional boolean fields in structs that will be
// serialized to JSON, such as in MCP tool definitions.
func BoolPtr(b bool) *bool {
	return &b
}

// IntPtr creates a pointer to an int value.
// This is useful for setting optional integer fields in structs that will be
// serialized to JSON, such as in MCP tool definitions.
func IntPtr(i int) *int {
	return &i
}

// Float64Ptr creates a pointer to a float64 value.
// This is useful for setting optional number fields in structs that will be
// serialized to JSON, such as in MCP tool definitions.
func Float64Ptr(f float64) *float64 {
	return &f
}

// ParseOptionalRFC3339 parses an optional RFC 3339 timestamp string, such as
// a `due_date` tool parameter. A nil or empty s means "not provided" and
// returns (nil, nil). Keeping these parameters as plain strings on the Params
// struct (rather than a bare time.Time field) means a malformed value is
// decoded successfully and reported here as a normal tool error, instead of
// failing during argument unmarshaling with an opaque encoding/json error.
func ParseOptionalRFC3339(field string, s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, fmt.Errorf("invalid %s format (expected RFC 3339, e.g. 2024-12-31T23:59:59Z): %w", field, err)
	}
	return &t, nil
}
