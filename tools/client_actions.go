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

// MyListActionTasksOptions holds optional pagination parameters for listing
// action tasks. The zero value requests the server default page size.
type MyListActionTasksOptions struct {
	Page  int
	Limit int
}

// MyListActionTasks lists Forgejo Actions tasks in a repository.
//
// Page and Limit are forwarded to the API as query parameters. Omitting both
// (zero value) lets the server apply its own default page size.
//
// Forgejo only applies "limit" when "page" is present: a request carrying
// limit alone is served as an unpaginated listing and returns every task in
// the repository. Whenever a limit is requested we therefore send page=1 by
// default, so that "give me N tasks" is honoured without the caller having to
// know about this coupling.
// GET /repos/{owner}/{repo}/actions/tasks
func (c *Client) MyListActionTasks(owner, repo string, opt MyListActionTasksOptions) (*types.MyActionTaskResponse, error) {
	q := url.Values{}
	page := opt.Page
	if page <= 0 && opt.Limit > 0 {
		page = 1
	}
	if page > 0 {
		q.Set("page", fmt.Sprintf("%d", page))
	}
	if opt.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opt.Limit))
	}

	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/actions/tasks", owner, repo)
	if enc := q.Encode(); enc != "" {
		endpoint += "?" + enc
	}

	var result types.MyActionTaskResponse
	if err := c.sendSimpleRequest("GET", endpoint, nil, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
