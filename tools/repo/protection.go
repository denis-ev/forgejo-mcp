// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/raohwork/forgejo-mcp/tools"
)

// protectionRuleProperties is the JSON schema of the settable rule fields,
// shared by create_branch_protection and edit_branch_protection. Keys match
// the json tags of tools.BranchProtectionRules.
func protectionRuleProperties() map[string]*jsonschema.Schema {
	strList := func(desc string) *jsonschema.Schema {
		return &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: desc}
	}
	boolean := func(desc string) *jsonschema.Schema { return &jsonschema.Schema{Type: "boolean", Description: desc} }
	return map[string]*jsonschema.Schema{
		"enable_push":                       boolean("Allow direct pushes to matching branches. false (the default on create) means only merging a pull request updates the branch"),
		"enable_push_whitelist":             boolean("Restrict direct pushes to the push whitelist (requires enable_push=true)"),
		"push_whitelist_usernames":          strList("Users allowed to push when enable_push_whitelist is on"),
		"push_whitelist_teams":              strList("Organization teams allowed to push when enable_push_whitelist is on"),
		"push_whitelist_deploy_keys":        boolean("Also allow deploy keys with write access to push when enable_push_whitelist is on"),
		"enable_merge_whitelist":            boolean("Restrict who may merge pull requests into matching branches to the merge whitelist"),
		"merge_whitelist_usernames":         strList("Users allowed to merge when enable_merge_whitelist is on"),
		"merge_whitelist_teams":             strList("Organization teams allowed to merge when enable_merge_whitelist is on"),
		"enable_status_check":               boolean("Require status checks to pass before merging. Turned on automatically when status_check_contexts is given"),
		"status_check_contexts":             strList("Required status check contexts, exactly as CI reports them (see get_ci_status); glob patterns are allowed"),
		"required_approvals":                {Type: "integer", Description: "Number of approving reviews required before merging (0 disables)", Minimum: ptrFloat(0)},
		"enable_approvals_whitelist":        boolean("Only count approvals from the approvals whitelist"),
		"approvals_whitelist_username":      strList("Users whose approvals count when enable_approvals_whitelist is on"),
		"approvals_whitelist_teams":         strList("Teams whose approvals count when enable_approvals_whitelist is on"),
		"block_on_rejected_reviews":         boolean("Block merging while a review requests changes"),
		"block_on_official_review_requests": boolean("Block merging while an official review request is pending"),
		"block_on_outdated_branch":          boolean("Block merging while the head branch is behind the base branch"),
		"dismiss_stale_approvals":           boolean("Dismiss approvals when new commits are pushed"),
		"ignore_stale_approvals":            boolean("Do not count approvals made on older commits"),
		"require_signed_commits":            boolean("Reject unsigned or unverifiable commits"),
		"apply_to_admins":                   boolean("Apply the rule to repository administrators too, so they cannot bypass it"),
		"protected_file_patterns":           {Type: "string", Description: "Semicolon-separated glob patterns of files that may not be changed, even by whitelisted pushers"},
		"unprotected_file_patterns":         {Type: "string", Description: "Semicolon-separated glob patterns of files that may be pushed directly even when pushing is disabled"},
	}
}

// withAutoStatusCheck turns on enable_status_check when contexts are given
// without an explicit enable_status_check, since contexts alone do nothing.
func withAutoStatusCheck(r tools.BranchProtectionRules) tools.BranchProtectionRules {
	if r.EnableStatusCheck == nil && r.StatusCheckContexts != nil && len(*r.StatusCheckContexts) > 0 {
		t := true
		r.EnableStatusCheck = &t
	}
	return r
}

// renderProtection formats one rule as markdown.
func renderProtection(bp *tools.BranchProtection) string {
	var b strings.Builder
	onOff := func(v bool) string {
		if v {
			return "on"
		}
		return "off"
	}
	list := func(v []string) string {
		if len(v) == 0 {
			return "(none)"
		}
		return "`" + strings.Join(v, "`, `") + "`"
	}

	fmt.Fprintf(&b, "**%s**\n", bp.Name())
	switch {
	case !bp.EnablePush:
		b.WriteString("- Direct push: disabled (only merging a PR updates the branch)\n")
	case bp.EnablePushWhitelist:
		fmt.Fprintf(&b, "- Direct push: whitelist only — users %s, teams %s, deploy keys %s\n",
			list(bp.PushWhitelistUsernames), list(bp.PushWhitelistTeams), onOff(bp.PushWhitelistDeployKeys))
	default:
		b.WriteString("- Direct push: allowed for anyone with write access\n")
	}
	if bp.EnableStatusCheck {
		fmt.Fprintf(&b, "- Required status checks: %s\n", list(bp.StatusCheckContexts))
	} else {
		b.WriteString("- Required status checks: off\n")
	}
	fmt.Fprintf(&b, "- Required approvals: %d", bp.RequiredApprovals)
	if bp.EnableApprovalsWhitelist {
		fmt.Fprintf(&b, " (counted from users %s, teams %s)", list(bp.ApprovalsWhitelistUsernames), list(bp.ApprovalsWhitelistTeams))
	}
	b.WriteString("\n")
	if bp.EnableMergeWhitelist {
		fmt.Fprintf(&b, "- Merge restricted to: users %s, teams %s\n", list(bp.MergeWhitelistUsernames), list(bp.MergeWhitelistTeams))
	}
	fmt.Fprintf(&b, "- Block on rejected reviews: %s, official review requests: %s, outdated branch: %s\n",
		onOff(bp.BlockOnRejectedReviews), onOff(bp.BlockOnOfficialReviewRequests), onOff(bp.BlockOnOutdatedBranch))
	fmt.Fprintf(&b, "- Dismiss stale approvals: %s, ignore stale approvals: %s\n", onOff(bp.DismissStaleApprovals), onOff(bp.IgnoreStaleApprovals))
	fmt.Fprintf(&b, "- Signed commits required: %s, applies to admins: %s\n", onOff(bp.RequireSignedCommits), onOff(bp.ApplyToAdmins))
	if bp.ProtectedFilePatterns != "" {
		fmt.Fprintf(&b, "- Protected files: `%s`\n", bp.ProtectedFilePatterns)
	}
	if bp.UnprotectedFilePatterns != "" {
		fmt.Fprintf(&b, "- Unprotected files: `%s`\n", bp.UnprotectedFilePatterns)
	}
	return strings.TrimRight(b.String(), "\n")
}

// ---- list_branch_protections ----

// ListBranchProtectionsParams defines the parameters for list_branch_protections.
type ListBranchProtectionsParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// ListBranchProtectionsImpl implements the read-only list_branch_protections tool.
type ListBranchProtectionsImpl struct {
	Client *tools.Client
}

// Definition describes the `list_branch_protections` tool.
func (ListBranchProtectionsImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_branch_protections",
		Title:       "List Branch Protections",
		Description: "List a repository's branch protection rules with their push, status-check, approval and review settings. Requires repository admin access.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
			},
			Required: []string{"owner", "repo"},
		},
	}
}

// Handler lists the rules.
func (impl ListBranchProtectionsImpl) Handler() mcp.ToolHandlerFor[ListBranchProtectionsParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args ListBranchProtectionsParams) (*mcp.CallToolResult, any, error) {
		rules, err := impl.Client.MyListBranchProtections(args.Owner, args.Repo)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list branch protections: %w", err)
		}
		if len(rules) == 0 {
			return textResult(fmt.Sprintf("No branch protection rules in %s/%s.", args.Owner, args.Repo)), nil, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Found %d branch protection rules in %s/%s\n", len(rules), args.Owner, args.Repo)
		for _, bp := range rules {
			b.WriteString("\n" + renderProtection(bp) + "\n")
		}
		return textResult(strings.TrimRight(b.String(), "\n")), nil, nil
	}
}

// ---- get_branch_protection ----

// GetBranchProtectionParams defines the parameters for get_branch_protection.
type GetBranchProtectionParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Name  string `json:"name"`
}

// GetBranchProtectionImpl implements the read-only get_branch_protection tool.
type GetBranchProtectionImpl struct {
	Client *tools.Client
}

// Definition describes the `get_branch_protection` tool.
func (GetBranchProtectionImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_branch_protection",
		Title:       "Get Branch Protection",
		Description: "Get one branch protection rule by its rule name (the branch name or pattern it protects). Requires repository admin access.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"name":  {Type: "string", Description: "Rule name: the branch name or glob pattern, e.g. 'main' or 'release/*'"},
			},
			Required: []string{"owner", "repo", "name"},
		},
	}
}

// Handler fetches the rule.
func (impl GetBranchProtectionImpl) Handler() mcp.ToolHandlerFor[GetBranchProtectionParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetBranchProtectionParams) (*mcp.CallToolResult, any, error) {
		bp, err := impl.Client.MyGetBranchProtection(args.Owner, args.Repo, args.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get branch protection %q: %w", args.Name, err)
		}
		return textResult(renderProtection(bp)), nil, nil
	}
}

// ---- create_branch_protection ----

// CreateBranchProtectionParams defines the parameters for create_branch_protection.
type CreateBranchProtectionParams struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	RuleName string `json:"rule_name"`
	tools.BranchProtectionRules
}

// CreateBranchProtectionImpl implements the create_branch_protection tool.
type CreateBranchProtectionImpl struct {
	Client *tools.Client
}

// Definition describes the `create_branch_protection` tool.
func (CreateBranchProtectionImpl) Definition() *mcp.Tool {
	props := protectionRuleProperties()
	props["owner"] = &jsonschema.Schema{Type: "string", Description: "Repository owner (username or organization name)"}
	props["repo"] = &jsonschema.Schema{Type: "string", Description: "Repository name"}
	props["rule_name"] = &jsonschema.Schema{Type: "string", Description: "Branch name or glob pattern the rule protects, e.g. 'main' or 'release/*'"}
	return &mcp.Tool{
		Name:  "create_branch_protection",
		Title: "Create Branch Protection",
		Description: "Create a branch protection rule. Unset options take Forgejo's defaults, so direct pushes are disabled unless enable_push is true. " +
			"Use status_check_contexts to require CI checks before merging. Requires repository admin access.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolFalse()},
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: props,
			Required:   []string{"owner", "repo", "rule_name"},
		},
	}
}

// Handler creates the rule.
func (impl CreateBranchProtectionImpl) Handler() mcp.ToolHandlerFor[CreateBranchProtectionParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args CreateBranchProtectionParams) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(args.RuleName) == "" {
			return nil, nil, fmt.Errorf("rule_name is required")
		}
		bp, err := impl.Client.MyCreateBranchProtection(args.Owner, args.Repo, args.RuleName, withAutoStatusCheck(args.BranchProtectionRules))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create branch protection %q: %w", args.RuleName, err)
		}
		return textResult(fmt.Sprintf("Created branch protection in %s/%s:\n\n%s", args.Owner, args.Repo, renderProtection(bp))), nil, nil
	}
}

// ---- edit_branch_protection ----

// EditBranchProtectionParams defines the parameters for edit_branch_protection.
type EditBranchProtectionParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Name  string `json:"name"`
	tools.BranchProtectionRules
}

// EditBranchProtectionImpl implements the edit_branch_protection tool.
type EditBranchProtectionImpl struct {
	Client *tools.Client
}

// Definition describes the `edit_branch_protection` tool.
func (EditBranchProtectionImpl) Definition() *mcp.Tool {
	props := protectionRuleProperties()
	props["owner"] = &jsonschema.Schema{Type: "string", Description: "Repository owner (username or organization name)"}
	props["repo"] = &jsonschema.Schema{Type: "string", Description: "Repository name"}
	props["name"] = &jsonschema.Schema{Type: "string", Description: "Rule name of the rule to edit, e.g. 'main'"}
	return &mcp.Tool{
		Name:  "edit_branch_protection",
		Title: "Edit Branch Protection",
		Description: "Change selected settings of an existing branch protection rule; options you leave out keep their current value. " +
			"A list option (e.g. status_check_contexts) replaces the whole list. Requires repository admin access.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolFalse(), IdempotentHint: true},
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: props,
			Required:   []string{"owner", "repo", "name"},
		},
	}
}

// Handler edits the rule.
func (impl EditBranchProtectionImpl) Handler() mcp.ToolHandlerFor[EditBranchProtectionParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args EditBranchProtectionParams) (*mcp.CallToolResult, any, error) {
		bp, err := impl.Client.MyEditBranchProtection(args.Owner, args.Repo, args.Name, withAutoStatusCheck(args.BranchProtectionRules))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to edit branch protection %q: %w", args.Name, err)
		}
		return textResult(fmt.Sprintf("Updated branch protection in %s/%s:\n\n%s", args.Owner, args.Repo, renderProtection(bp))), nil, nil
	}
}

// ---- delete_branch_protection ----

// DeleteBranchProtectionParams defines the parameters for delete_branch_protection.
type DeleteBranchProtectionParams struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Name  string `json:"name"`
}

// DeleteBranchProtectionImpl implements the delete_branch_protection tool.
type DeleteBranchProtectionImpl struct {
	Client *tools.Client
}

// Definition describes the `delete_branch_protection` tool.
func (DeleteBranchProtectionImpl) Definition() *mcp.Tool {
	return &mcp.Tool{
		Name:        "delete_branch_protection",
		Title:       "Delete Branch Protection",
		Description: "Delete a branch protection rule, leaving matching branches unprotected. Requires repository admin access.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"owner": {Type: "string", Description: "Repository owner (username or organization name)"},
				"repo":  {Type: "string", Description: "Repository name"},
				"name":  {Type: "string", Description: "Rule name of the rule to delete, e.g. 'main'"},
			},
			Required: []string{"owner", "repo", "name"},
		},
	}
}

// Handler deletes the rule.
func (impl DeleteBranchProtectionImpl) Handler() mcp.ToolHandlerFor[DeleteBranchProtectionParams, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args DeleteBranchProtectionParams) (*mcp.CallToolResult, any, error) {
		if err := impl.Client.MyDeleteBranchProtection(args.Owner, args.Repo, args.Name); err != nil {
			return nil, nil, fmt.Errorf("failed to delete branch protection %q: %w", args.Name, err)
		}
		return textResult(fmt.Sprintf("Deleted branch protection %q from %s/%s.", args.Name, args.Owner, args.Repo)), nil, nil
	}
}
