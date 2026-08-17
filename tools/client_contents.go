// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package tools

import (
	"fmt"

	"github.com/raohwork/forgejo-mcp/types"
)

// MyChangeFiles creates, updates and deletes several files in one commit.
// POST /repos/{owner}/{repo}/contents
//
// The Forgejo SDK exposes single-file CreateFile/UpdateFile/DeleteFile but has
// no binding for the multi-file endpoint, so this issues the request directly
// while reusing the SDK's authentication settings.
func (c *Client) MyChangeFiles(owner, repo string, options types.MyChangeFilesOptions) (*types.MyFilesResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/repos/%s/%s/contents", owner, repo)

	var result types.MyFilesResponse
	if err := c.sendSimpleRequest("POST", endpoint, options, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
