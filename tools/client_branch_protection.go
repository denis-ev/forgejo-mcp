// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package tools

import (
	"fmt"
	"net/url"
	"time"

	forgejo "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// BranchProtectionRules holds the settable fields of a branch protection
// rule. Every field is a pointer so a nil value is left out of the request:
// on create Forgejo applies its default, on edit the current value is kept.
//
// The SDK's option structs are not used because they lack fields current
// Forgejo accepts (apply_to_admins, ignore_stale_approvals) and, for create,
// serialize every unset field as false/null.
type BranchProtectionRules struct {
	EnablePush                    *bool     `json:"enable_push,omitempty"`
	EnablePushWhitelist           *bool     `json:"enable_push_whitelist,omitempty"`
	PushWhitelistUsernames        *[]string `json:"push_whitelist_usernames,omitempty"`
	PushWhitelistTeams            *[]string `json:"push_whitelist_teams,omitempty"`
	PushWhitelistDeployKeys       *bool     `json:"push_whitelist_deploy_keys,omitempty"`
	EnableMergeWhitelist          *bool     `json:"enable_merge_whitelist,omitempty"`
	MergeWhitelistUsernames       *[]string `json:"merge_whitelist_usernames,omitempty"`
	MergeWhitelistTeams           *[]string `json:"merge_whitelist_teams,omitempty"`
	EnableStatusCheck             *bool     `json:"enable_status_check,omitempty"`
	StatusCheckContexts           *[]string `json:"status_check_contexts,omitempty"`
	RequiredApprovals             *int64    `json:"required_approvals,omitempty"`
	EnableApprovalsWhitelist      *bool     `json:"enable_approvals_whitelist,omitempty"`
	ApprovalsWhitelistUsernames   *[]string `json:"approvals_whitelist_username,omitempty"`
	ApprovalsWhitelistTeams       *[]string `json:"approvals_whitelist_teams,omitempty"`
	BlockOnRejectedReviews        *bool     `json:"block_on_rejected_reviews,omitempty"`
	BlockOnOfficialReviewRequests *bool     `json:"block_on_official_review_requests,omitempty"`
	BlockOnOutdatedBranch         *bool     `json:"block_on_outdated_branch,omitempty"`
	DismissStaleApprovals         *bool     `json:"dismiss_stale_approvals,omitempty"`
	IgnoreStaleApprovals          *bool     `json:"ignore_stale_approvals,omitempty"`
	RequireSignedCommits          *bool     `json:"require_signed_commits,omitempty"`
	ApplyToAdmins                 *bool     `json:"apply_to_admins,omitempty"`
	ProtectedFilePatterns         *string   `json:"protected_file_patterns,omitempty"`
	UnprotectedFilePatterns       *string   `json:"unprotected_file_patterns,omitempty"`
}

// BranchProtection is a branch protection rule as Forgejo returns it.
type BranchProtection struct {
	BranchName                    string    `json:"branch_name"`
	RuleName                      string    `json:"rule_name"`
	EnablePush                    bool      `json:"enable_push"`
	EnablePushWhitelist           bool      `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string  `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string  `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool      `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          bool      `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string  `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string  `json:"merge_whitelist_teams"`
	EnableStatusCheck             bool      `json:"enable_status_check"`
	StatusCheckContexts           []string  `json:"status_check_contexts"`
	RequiredApprovals             int64     `json:"required_approvals"`
	EnableApprovalsWhitelist      bool      `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string  `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string  `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        bool      `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool      `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         bool      `json:"block_on_outdated_branch"`
	DismissStaleApprovals         bool      `json:"dismiss_stale_approvals"`
	IgnoreStaleApprovals          bool      `json:"ignore_stale_approvals"`
	RequireSignedCommits          bool      `json:"require_signed_commits"`
	ApplyToAdmins                 bool      `json:"apply_to_admins"`
	ProtectedFilePatterns         string    `json:"protected_file_patterns"`
	UnprotectedFilePatterns       string    `json:"unprotected_file_patterns"`
	Created                       time.Time `json:"created_at"`
	Updated                       time.Time `json:"updated_at"`
}

// Name returns the rule name, falling back to the legacy branch_name field.
func (bp *BranchProtection) Name() string {
	if bp.RuleName != "" {
		return bp.RuleName
	}
	return bp.BranchName
}

// createBranchProtectionBody is the POST body: the rule name plus the rules.
type createBranchProtectionBody struct {
	RuleName string `json:"rule_name"`
	BranchProtectionRules
}

func protectionEndpoint(owner, repo string) string {
	return fmt.Sprintf("/api/v1/repos/%s/%s/branch_protections", url.PathEscape(owner), url.PathEscape(repo))
}

func protectionRuleEndpoint(owner, repo, name string) string {
	return protectionEndpoint(owner, repo) + "/" + url.PathEscape(name)
}

// MyListBranchProtections lists a repository's branch protection rules.
// GET /repos/{owner}/{repo}/branch_protections
func (c *Client) MyListBranchProtections(owner, repo string) ([]*BranchProtection, error) {
	var out []*BranchProtection
	if err := c.sendSimpleRequest("GET", protectionEndpoint(owner, repo), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MyGetBranchProtection gets one branch protection rule by name.
// GET /repos/{owner}/{repo}/branch_protections/{name}
func (c *Client) MyGetBranchProtection(owner, repo, name string) (*BranchProtection, error) {
	var out BranchProtection
	if err := c.sendSimpleRequest("GET", protectionRuleEndpoint(owner, repo, name), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MyCreateBranchProtection creates a branch protection rule. ruleName is the
// branch name or glob pattern the rule applies to.
// POST /repos/{owner}/{repo}/branch_protections
func (c *Client) MyCreateBranchProtection(owner, repo, ruleName string, rules BranchProtectionRules) (*BranchProtection, error) {
	var out BranchProtection
	body := createBranchProtectionBody{RuleName: ruleName, BranchProtectionRules: rules}
	if err := c.sendSimpleRequest("POST", protectionEndpoint(owner, repo), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MyEditBranchProtection changes the given fields of a branch protection
// rule; nil fields keep their current value.
// PATCH /repos/{owner}/{repo}/branch_protections/{name}
func (c *Client) MyEditBranchProtection(owner, repo, name string, rules BranchProtectionRules) (*BranchProtection, error) {
	var out BranchProtection
	if err := c.sendSimpleRequest("PATCH", protectionRuleEndpoint(owner, repo, name), rules, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MyDeleteBranchProtection deletes a branch protection rule.
// DELETE /repos/{owner}/{repo}/branch_protections/{name}
func (c *Client) MyDeleteBranchProtection(owner, repo, name string) error {
	_, err := c.sendRawRequest("DELETE", protectionRuleEndpoint(owner, repo, name), nil)
	return err
}

// EditRepositoryOptions holds the repository settings edit_repository can
// change. Nil fields are left out of the PATCH and keep their current value.
type EditRepositoryOptions struct {
	Description                   *string `json:"description,omitempty"`
	Website                       *string `json:"website,omitempty"`
	Private                       *bool   `json:"private,omitempty"`
	Template                      *bool   `json:"template,omitempty"`
	Archived                      *bool   `json:"archived,omitempty"`
	DefaultBranch                 *string `json:"default_branch,omitempty"`
	HasIssues                     *bool   `json:"has_issues,omitempty"`
	HasWiki                       *bool   `json:"has_wiki,omitempty"`
	HasPullRequests               *bool   `json:"has_pull_requests,omitempty"`
	HasProjects                   *bool   `json:"has_projects,omitempty"`
	HasReleases                   *bool   `json:"has_releases,omitempty"`
	HasPackages                   *bool   `json:"has_packages,omitempty"`
	HasActions                    *bool   `json:"has_actions,omitempty"`
	AllowMergeCommits             *bool   `json:"allow_merge_commits,omitempty"`
	AllowRebase                   *bool   `json:"allow_rebase,omitempty"`
	AllowRebaseExplicit           *bool   `json:"allow_rebase_explicit,omitempty"`
	AllowSquashMerge              *bool   `json:"allow_squash_merge,omitempty"`
	AllowFastForwardOnlyMerge     *bool   `json:"allow_fast_forward_only_merge,omitempty"`
	AllowRebaseUpdate             *bool   `json:"allow_rebase_update,omitempty"`
	DefaultMergeStyle             *string `json:"default_merge_style,omitempty"`
	DefaultDeleteBranchAfterMerge *bool   `json:"default_delete_branch_after_merge,omitempty"`
	DefaultAllowMaintainerEdit    *bool   `json:"default_allow_maintainer_edit,omitempty"`
}

// MyEditRepository changes repository settings.
// PATCH /repos/{owner}/{repo}
func (c *Client) MyEditRepository(owner, repo string, opt EditRepositoryOptions) (*forgejo.Repository, error) {
	var out forgejo.Repository
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	if err := c.sendSimpleRequest("PATCH", endpoint, opt, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
