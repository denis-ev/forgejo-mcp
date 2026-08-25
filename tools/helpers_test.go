// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"strings"
	"testing"
	"time"
)

func TestParseOptionalRFC3339(t *testing.T) {
	t.Run("nil_is_unset", func(t *testing.T) {
		got, err := ParseOptionalRFC3339("due_date", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty_string_is_unset", func(t *testing.T) {
		s := ""
		got, err := ParseOptionalRFC3339("due_date", &s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("valid_rfc3339_is_parsed", func(t *testing.T) {
		s := "2024-12-31T23:59:59Z"
		got, err := ParseOptionalRFC3339("due_date", &s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected a parsed time, got nil")
		}
		want, _ := time.Parse(time.RFC3339, s)
		if !got.Equal(want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("malformed_string_returns_clean_error", func(t *testing.T) {
		s := "not-a-date"
		got, err := ParseOptionalRFC3339("due_date", &s)
		if got != nil {
			t.Errorf("expected nil time on error, got %v", got)
		}
		if err == nil {
			t.Fatal("expected an error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "due_date") {
			t.Errorf("expected field name in error, got %q", msg)
		}
		if !strings.Contains(msg, "RFC 3339") {
			t.Errorf("expected format hint in error, got %q", msg)
		}
	})
}
