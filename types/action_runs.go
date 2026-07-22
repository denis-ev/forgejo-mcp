// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package types

import (
	"fmt"
	"time"
)

// MyActionRun represents a single Forgejo Actions workflow run.
// Used by endpoints:
//   - GET /repos/{owner}/{repo}/actions/runs
//   - GET /repos/{owner}/{repo}/actions/runs/{run_id}
type MyActionRun struct {
	ID           int64     `json:"id"`
	Index        int64     `json:"index_in_repo"`
	Title        string    `json:"title"`
	WorkflowID   string    `json:"workflow_id"`
	Event        string    `json:"event"`
	TriggerEvent string    `json:"trigger_event"`
	Status       string    `json:"status"`
	PrettyRef    string    `json:"prettyref"`
	CommitSHA    string    `json:"commit_sha"`
	IsRefDeleted bool      `json:"is_ref_deleted"`
	NeedApproval bool      `json:"need_approval"`
	HTMLURL      string    `json:"html_url"`
	Created      time.Time `json:"created"`
	Started      time.Time `json:"started"`
	Stopped      time.Time `json:"stopped"`
	Updated      time.Time `json:"updated"`
}

// ToMarkdown renders an action run with title, status and timing info.
// Example: **Build and Test** `success` - Run #12 (ci.yml)
// Ref: refs/heads/main | Event: push | Started: 2026-01-01 12:00 | Duration: 1m30s
func (r *MyActionRun) ToMarkdown() string {
	md := fmt.Sprintf("**%s** `%s` - Run #%d (%s)", r.Title, r.Status, r.Index, r.WorkflowID)
	if r.PrettyRef != "" {
		md += fmt.Sprintf("\nRef: %s", r.PrettyRef)
	}
	if r.Event != "" {
		md += fmt.Sprintf(" | Event: %s", r.Event)
	}
	if !r.Started.IsZero() {
		md += fmt.Sprintf(" | Started: %s", r.Started.Format("2006-01-02 15:04"))
	}
	if !r.Started.IsZero() && !r.Stopped.IsZero() && r.Stopped.After(r.Started) {
		md += fmt.Sprintf(" | Duration: %s", r.Stopped.Sub(r.Started).Round(time.Second).String())
	}
	md += fmt.Sprintf("\nRun ID: %d", r.ID)
	return md
}

// MyActionRunListResponse represents the response for listing action runs.
// GET /repos/{owner}/{repo}/actions/runs
type MyActionRunListResponse struct {
	TotalCount int64          `json:"total_count"`
	Entries    []*MyActionRun `json:"workflow_runs"`
}

// ActionRunList wraps MyActionRunListResponse for markdown rendering.
type ActionRunList struct {
	*MyActionRunListResponse
}

// ToMarkdown renders action runs as a numbered list.
func (l ActionRunList) ToMarkdown() string {
	if l.MyActionRunListResponse == nil || len(l.Entries) == 0 {
		return "*No action runs found*"
	}
	md := ""
	for i, r := range l.Entries {
		md += fmt.Sprintf("%d. %s\n", i+1, r.ToMarkdown())
	}
	return md
}

// MyActionRunJob represents a single job within a Forgejo Actions run.
// Used by endpoint:
//   - GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs
type MyActionRunJob struct {
	ID      int64    `json:"id"`
	RunID   int64    `json:"run_id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Attempt int64    `json:"attempt"`
	TaskID  int64    `json:"task_id"`
	RunsOn  []string `json:"runs_on"`
	Needs   []string `json:"needs"`
}

// ToMarkdown renders a job with its ID, name and status.
// Example: **mix check** `failure` (Job ID: 1049, Attempt 1)
func (j *MyActionRunJob) ToMarkdown() string {
	md := fmt.Sprintf("**%s** `%s` (Job ID: %d, Attempt %d)", j.Name, j.Status, j.ID, j.Attempt)
	if len(j.RunsOn) > 0 {
		md += fmt.Sprintf(" - runs on: %v", j.RunsOn)
	}
	return md
}

// ActionRunJobList is a list of jobs belonging to one run.
// GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs returns a bare JSON array.
type ActionRunJobList []*MyActionRunJob

// ToMarkdown renders jobs as a numbered list.
func (l ActionRunJobList) ToMarkdown() string {
	if len(l) == 0 {
		return "*No jobs found for this run*"
	}
	md := ""
	for i, j := range l {
		md += fmt.Sprintf("%d. %s\n", i+1, j.ToMarkdown())
	}
	return md
}

// ActionJobLogs represents the plaintext log output of a single Actions job.
// GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs returns raw text, not JSON;
// this type just wraps the text for markdown rendering.
type ActionJobLogs struct {
	JobID int64
	Text  string
}

// ToMarkdown renders logs inside a fenced code block. Content is truncated to
// keep responses within reasonable size for MCP clients.
func (l ActionJobLogs) ToMarkdown() string {
	if l.Text == "" {
		return "*No logs available for this job*"
	}
	const maxLen = 20000
	text := l.Text
	truncated := false
	if len(text) > maxLen {
		// Keep the tail, since failures usually show up at the end of the log.
		text = text[len(text)-maxLen:]
		truncated = true
	}
	md := fmt.Sprintf("Logs for job %d:\n\n```\n%s\n```", l.JobID, text)
	if truncated {
		md = fmt.Sprintf("*(log truncated, showing last %d characters)*\n\n", maxLen) + md
	}
	return md
}
