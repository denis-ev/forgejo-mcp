// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MergePullRequestBody is the POST body of the merge endpoint. Forgejo names
// several of these fields in its form style (Do, MergeTitleField, ...).
type MergePullRequestBody struct {
	Do                     string `json:"Do"`
	MergeTitleField        string `json:"MergeTitleField,omitempty"`
	MergeMessageField      string `json:"MergeMessageField,omitempty"`
	DeleteBranchAfterMerge bool   `json:"delete_branch_after_merge,omitempty"`
	HeadCommitID           string `json:"head_commit_id,omitempty"`
	MergeWhenChecksSucceed bool   `json:"merge_when_checks_succeed,omitempty"`
}

// MergeOutcome classifies the response of the merge endpoint.
type MergeOutcome int

const (
	// MergeOutcomeMerged: the pull request was merged (HTTP 200).
	MergeOutcomeMerged MergeOutcome = iota
	// MergeOutcomeScheduled: an auto-merge was scheduled and will run once
	// the required checks succeed (HTTP 201).
	MergeOutcomeScheduled
	// MergeOutcomeMergedBranchKept: the merge succeeded but deleting the head
	// branch was refused (HTTP 403 raised after the merge).
	MergeOutcomeMergedBranchKept
	// MergeOutcomeRejected: Forgejo refused to merge; see MergeResult.Reason.
	MergeOutcomeRejected
)

// MergeResult is the interpreted response of the merge endpoint.
type MergeResult struct {
	Outcome MergeOutcome
	// Status is the HTTP status code Forgejo answered with.
	Status int
	// Reason is Forgejo's own explanation for a rejection or a kept branch.
	Reason string
}

// MyMergePullRequest merges (or schedules an auto-merge of) a pull request.
//
// The SDK's MergePullRequest only treats HTTP 200 as success and drops the
// response body, so it reports a scheduled auto-merge (201) as "not merged"
// and hides why Forgejo rejected a merge (405/409). This calls the endpoint
// directly and classifies every documented response.
// POST /repos/{owner}/{repo}/pulls/{index}/merge
func (c *Client) MyMergePullRequest(owner, repo string, index int64, body MergePullRequestBody) (*MergeResult, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", url.PathEscape(owner), url.PathEscape(repo), index)
	status, data, err := c.sendStatusRequest("POST", endpoint, body)
	if err != nil {
		return nil, err
	}
	reason := apiErrorMessage(data)
	switch {
	case status == http.StatusOK:
		return &MergeResult{Outcome: MergeOutcomeMerged, Status: status}, nil
	case status == http.StatusCreated:
		return &MergeResult{Outcome: MergeOutcomeScheduled, Status: status}, nil
	case status == http.StatusForbidden && body.DeleteBranchAfterMerge && isPostMergeBranchError(reason):
		// Forgejo merges first and only then tries to delete the head branch;
		// these 403s are raised after the merge has already landed.
		return &MergeResult{Outcome: MergeOutcomeMergedBranchKept, Status: status, Reason: reason}, nil
	case status >= 400:
		if reason == "" {
			reason = http.StatusText(status)
		}
		return &MergeResult{Outcome: MergeOutcomeRejected, Status: status, Reason: reason}, nil
	default:
		return nil, fmt.Errorf("unexpected HTTP %d from merge endpoint", status)
	}
}

// MyCancelAutoMerge cancels a scheduled auto-merge of a pull request.
// DELETE /repos/{owner}/{repo}/pulls/{index}/merge
func (c *Client) MyCancelAutoMerge(owner, repo string, index int64) error {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/merge", url.PathEscape(owner), url.PathEscape(repo), index)
	_, err := c.sendRawRequest("DELETE", endpoint, nil)
	return err
}

// isPostMergeBranchError reports whether a 403 message is one of the errors
// Forgejo raises while deleting the head branch after a successful merge.
func isPostMergeBranchError(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "head branch")
}

// sendStatusRequest sends a JSON request and returns the status code and raw
// body without treating 4xx/5xx as an error, for endpoints whose status code
// carries the result.
func (c *Client) sendStatusRequest(method, endpoint string, paramObj any) (int, []byte, error) {
	u, err := url.Parse(c.base + endpoint)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid URL: %w", err)
	}

	var body io.Reader
	if paramObj != nil {
		jsonData, err := json.Marshal(paramObj)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if paramObj != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	resp, err := c.cl.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response: %w", err)
	}
	return resp.StatusCode, data, nil
}

// apiErrorMessage extracts a human-readable message from a Forgejo error
// body. Forgejo answers {"message": "...", "errors": [...]} for most errors;
// anything else is returned trimmed and capped.
func apiErrorMessage(data []byte) string {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return ""
	}
	var apiErr struct {
		Message string   `json:"message"`
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(data, &apiErr); err == nil && (apiErr.Message != "" || len(apiErr.Errors) > 0) {
		parts := []string{}
		if apiErr.Message != "" {
			parts = append(parts, apiErr.Message)
		}
		for _, e := range apiErr.Errors {
			if e != "" && e != apiErr.Message {
				parts = append(parts, e)
			}
		}
		return strings.Join(parts, ": ")
	}
	s := string(data)
	if s == "{}" || s == "null" {
		return ""
	}
	if len(s) > 500 {
		s = s[:500] + "…"
	}
	return s
}
