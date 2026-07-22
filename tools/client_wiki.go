// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/raohwork/forgejo-mcp/types"
)

// nameToWikiSubURL converts a wiki page title (as displayed by the Forgejo/
// Gitea UI and returned by list_wiki_pages) into the URL-encoded sub_url
// slug expected by the wiki page endpoints:
//   - Spaces become hyphens
//   - Each path segment is percent-encoded and joined with "%2F"
//   - Nested pages (title contains "/") get a ".-" suffix appended, matching
//     what Gitea/Forgejo actually returns as sub_url (verified against a
//     live Gitea 1.25.4 instance; see raohwork/forgejo-mcp#6)
//   - Input that already looks like an encoded sub_url (contains "%2f",
//     case-insensitive) is returned unchanged to avoid double-encoding
func nameToWikiSubURL(name string) string {
	if strings.Contains(strings.ToLower(name), "%2f") {
		return name
	}

	slug := strings.ReplaceAll(name, " ", "-")

	if !strings.Contains(slug, "/") {
		return url.PathEscape(slug)
	}

	parts := strings.Split(slug, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	result := strings.Join(parts, "%2F")

	if !strings.HasSuffix(result, ".-") {
		result += ".-"
	}

	return result
}

// MyListWikiPages lists all wiki pages in a repository.
// It paginates through all results since the Gitea/Forgejo API defaults
// to returning only 30 items per page.
// GET /repos/{owner}/{repo}/wiki/pages
func (c *Client) MyListWikiPages(owner, repo string) ([]*types.MyWikiPageMetaData, error) {
	var allPages []*types.MyWikiPageMetaData
	page := 1
	limit := 50

	for {
		endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/pages?page=%d&limit=%d", owner, repo, page, limit)

		var result []*types.MyWikiPageMetaData
		err := c.sendSimpleRequest("GET", endpoint, nil, &result)
		if err != nil {
			// First page error means wiki not initialized or other failure
			if page == 1 {
				return nil, err
			}
			// Later page errors mean we've exhausted results
			break
		}

		allPages = append(allPages, result...)

		// If we got fewer than limit, we've reached the last page
		if len(result) < limit {
			break
		}
		page++
	}

	return allPages, nil
}

// MyGetWikiPage gets a single wiki page by name.
// GET /repos/{owner}/{repo}/wiki/page/{pageName}
//
// pageName may be either the raw sub_url slug (as returned by
// list_wiki_pages) or the human-readable page title (as displayed by the
// UI). The API only accepts the slug form, so if the first request 404s,
// pageName is converted via nameToWikiSubURL and retried once. This avoids
// forcing callers to know about Gitea's internal slug encoding.
func (c *Client) MyGetWikiPage(owner, repo, pageName string) (*types.MyWikiPage, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", owner, repo, pageName)

	var result types.MyWikiPage
	err := c.sendSimpleRequest("GET", endpoint, nil, &result)
	if err == nil {
		return &result, nil
	}
	if !isNotFoundErr(err) {
		return nil, err
	}

	// Retry with the slug form derived from the title.
	slug := nameToWikiSubURL(pageName)
	if slug == pageName {
		return nil, err
	}
	endpoint = fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", owner, repo, slug)
	result = types.MyWikiPage{}
	if err2 := c.sendSimpleRequest("GET", endpoint, nil, &result); err2 != nil {
		return nil, err
	}

	return &result, nil
}

// MyCreateWikiPage creates a new wiki page.
// POST /repos/{owner}/{repo}/wiki/new
func (c *Client) MyCreateWikiPage(owner, repo string, options types.MyCreateWikiPageOptions) (*types.MyWikiPage, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/new", owner, repo)

	var result types.MyWikiPage
	err := c.sendSimpleRequest("POST", endpoint, options, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// MyDeleteWikiPage deletes a wiki page.
// DELETE /repos/{owner}/{repo}/wiki/page/{pageName}
//
// See MyGetWikiPage for why pageName is retried as a slug on 404.
func (c *Client) MyDeleteWikiPage(owner, repo, pageName string) error {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", owner, repo, pageName)

	// DELETE returns 204 No Content on success
	var result interface{}
	err := c.sendSimpleRequest("DELETE", endpoint, nil, &result)
	if err == nil {
		return nil
	}
	if !isNotFoundErr(err) {
		return err
	}

	slug := nameToWikiSubURL(pageName)
	if slug == pageName {
		return err
	}
	endpoint = fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", owner, repo, slug)
	result = nil
	if err2 := c.sendSimpleRequest("DELETE", endpoint, nil, &result); err2 != nil {
		return err
	}

	return nil
}

// MyEditWikiPage edits an existing wiki page.
// PATCH /repos/{owner}/{repo}/wiki/page/{pageName}
//
// See MyGetWikiPage for why pageName is retried as a slug on 404.
func (c *Client) MyEditWikiPage(owner, repo, pageName string, options types.MyCreateWikiPageOptions) (*types.MyWikiPage, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", owner, repo, pageName)

	var result types.MyWikiPage
	err := c.sendSimpleRequest("PATCH", endpoint, options, &result)
	if err == nil {
		return &result, nil
	}
	if !isNotFoundErr(err) {
		return nil, err
	}

	slug := nameToWikiSubURL(pageName)
	if slug == pageName {
		return nil, err
	}
	endpoint = fmt.Sprintf("/api/v1/repos/%s/%s/wiki/page/%s", owner, repo, slug)
	result = types.MyWikiPage{}
	if err2 := c.sendSimpleRequest("PATCH", endpoint, options, &result); err2 != nil {
		return nil, err
	}

	return &result, nil
}
