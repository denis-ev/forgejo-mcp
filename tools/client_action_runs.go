// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"fmt"
	"net/url"

	"github.com/raohwork/forgejo-mcp/types"
)

// MyListActionRunsOptions holds optional filters for listing action runs.
type MyListActionRunsOptions struct {
	Page   int
	Limit  int
	Status string
}

// MyListActionRuns lists Forgejo Actions workflow runs in a repository.
// This is the modern replacement for the legacy /actions/tasks endpoint,
// added in Forgejo's newer Actions API (verified against Forgejo v16.0.1 /
// Gitea 1.22 API surface). It exposes run-level status/timing that the old
// tasks endpoint does not, and is the entry point for drilling down into
// jobs and logs.
// GET /repos/{owner}/{repo}/actions/runs
func (c *Client) MyListActionRuns(owner, repo string, opt MyListActionRunsOptions) (*types.MyActionRunListResponse, error) {
	q := url.Values{}
	if opt.Page > 0 {
		q.Set("page", fmt.Sprintf("%d", opt.Page))
	}
	if opt.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opt.Limit))
	}
	if opt.Status != "" {
		q.Add("status", opt.Status)
	}

	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs", owner, repo)
	if enc := q.Encode(); enc != "" {
		endpoint += "?" + enc
	}

	var result types.MyActionRunListResponse
	if err := c.sendSimpleRequest("GET", endpoint, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MyGetActionRun fetches a single Forgejo Actions workflow run by ID.
// GET /repos/{owner}/{repo}/actions/runs/{run_id}
func (c *Client) MyGetActionRun(owner, repo string, runID int64) (*types.MyActionRun, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d", owner, repo, runID)

	var result types.MyActionRun
	if err := c.sendSimpleRequest("GET", endpoint, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MyListActionRunJobs lists the jobs belonging to a specific Actions run.
// The run ID and job/task IDs are distinct numbering schemes in Forgejo;
// this call is required to discover the job_id needed by MyGetActionJobLogs.
// GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs
func (c *Client) MyListActionRunJobs(owner, repo string, runID int64) (types.ActionRunJobList, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID)

	var result types.ActionRunJobList
	if err := c.sendSimpleRequest("GET", endpoint, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// MyGetActionJobLogs fetches the plaintext execution log for a single
// Actions job. This is a genuinely new capability compared to older Forgejo
// releases: raw step-by-step logs (including failure stack traces) are now
// retrievable over the REST API rather than only through the web UI.
// GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs
func (c *Client) MyGetActionJobLogs(owner, repo string, jobID int64) (string, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/actions/jobs/%d/logs", owner, repo, jobID)

	data, err := c.sendRawRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
