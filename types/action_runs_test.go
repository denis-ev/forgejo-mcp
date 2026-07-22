// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package types

import (
	"strings"
	"testing"
	"time"
)

func TestMyActionRun_ToMarkdown(t *testing.T) {
	started := testTime()
	stopped := started.Add(90 * time.Second)

	tests := []struct {
		name     string
		run      *MyActionRun
		required []string
	}{
		{
			name: "complete run with timing",
			run: &MyActionRun{
				ID:         302,
				Index:      42,
				Title:      "Merge PR #1",
				WorkflowID: "test.yml",
				Status:     "failure",
				Event:      "push",
				PrettyRef:  "refs/heads/master",
				Started:    started,
				Stopped:    stopped,
			},
			required: []string{"Merge PR #1", "failure", "Run #42", "test.yml", "refs/heads/master", "push", "Duration: 1m30s", "Run ID: 302"},
		},
		{
			name: "run without timing",
			run: &MyActionRun{
				ID:         5,
				Index:      1,
				Title:      "initial",
				WorkflowID: "ci.yml",
				Status:     "running",
			},
			required: []string{"initial", "running", "Run #1", "Run ID: 5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.run.ToMarkdown()
			assertContains(t, out, tt.required)
		})
	}
}

func TestActionRunList_ToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		list     ActionRunList
		required []string
	}{
		{
			name: "multiple runs",
			list: ActionRunList{
				MyActionRunListResponse: &MyActionRunListResponse{
					TotalCount: 2,
					Entries: []*MyActionRun{
						{ID: 1, Index: 1, Title: "run one", Status: "success"},
						{ID: 2, Index: 2, Title: "run two", Status: "failure"},
					},
				},
			},
			required: []string{"1.", "run one", "success", "2.", "run two", "failure"},
		},
		{
			name:     "empty",
			list:     ActionRunList{MyActionRunListResponse: &MyActionRunListResponse{}},
			required: []string{"No action runs found"},
		},
		{
			name:     "nil",
			list:     ActionRunList{},
			required: []string{"No action runs found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.list.ToMarkdown()
			assertContains(t, out, tt.required)
		})
	}
}

func TestMyActionRunJob_ToMarkdown(t *testing.T) {
	job := &MyActionRunJob{
		ID: 1049, RunID: 302, Name: "mix check", Status: "failure", Attempt: 1, TaskID: 1199,
		RunsOn: []string{"ubuntu-24.04"},
	}
	out := job.ToMarkdown()
	assertContains(t, out, []string{"mix check", "failure", "Job ID: 1049", "Attempt 1", "ubuntu-24.04"})
}

func TestActionRunJobList_ToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		list     ActionRunJobList
		required []string
	}{
		{
			name: "multiple jobs",
			list: ActionRunJobList{
				{ID: 1, Name: "build", Status: "success"},
				{ID: 2, Name: "test", Status: "failure"},
			},
			required: []string{"1.", "build", "success", "2.", "test", "failure"},
		},
		{
			name:     "empty",
			list:     ActionRunJobList{},
			required: []string{"No jobs found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.list.ToMarkdown()
			assertContains(t, out, tt.required)
		})
	}
}

func TestActionJobLogs_ToMarkdown(t *testing.T) {
	t.Run("short_log", func(t *testing.T) {
		l := ActionJobLogs{JobID: 1049, Text: "line1\nline2\njob failed\n"}
		out := l.ToMarkdown()
		assertContains(t, out, []string{"1049", "line1", "job failed", "```"})
	})

	t.Run("empty_log", func(t *testing.T) {
		l := ActionJobLogs{JobID: 1049, Text: ""}
		out := l.ToMarkdown()
		assertContains(t, out, []string{"No logs available"})
	})

	t.Run("truncates_long_log_keeping_tail", func(t *testing.T) {
		var sb strings.Builder
		for i := 0; i < 30000; i++ {
			sb.WriteByte('a')
		}
		sb.WriteString("END_MARKER")
		l := ActionJobLogs{JobID: 1, Text: sb.String()}
		out := l.ToMarkdown()
		if !strings.Contains(out, "END_MARKER") {
			t.Error("expected truncated output to retain the tail (END_MARKER)")
		}
		if !strings.Contains(out, "truncated") {
			t.Error("expected truncation notice in output")
		}
	})
}
