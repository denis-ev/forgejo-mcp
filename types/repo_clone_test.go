// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package types

import (
	"strings"
	"testing"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

// TestRepositoryToMarkdownDoesNotLeakCredentials guards the rule that clone
// URLs are echoed from the API verbatim. A token must never be interpolated
// into a rendered URL, because tool output lands in agent transcripts, shell
// history and CI logs.
func TestRepositoryToMarkdownDoesNotLeakCredentials(t *testing.T) {
	const token = "s3cr3t-token-value"

	repo := &Repository{
		Repository: &forgejo.Repository{
			FullName: "owner/repo-name",
			HTMLURL:  "https://git.example.com/owner/repo-name",
			CloneURL: "https://git.example.com/owner/repo-name.git",
			SSHURL:   "ssh://git@ssh.example.com:2222/owner/repo-name.git",
		},
	}

	output := repo.ToMarkdown()

	if strings.Contains(output, token) {
		t.Errorf("token leaked into repository markdown: %q", output)
	}
	// A credential-bearing URL takes the form scheme://user:pass@host. The
	// only '@' we legitimately emit is the SSH user, which carries no secret.
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "://") {
			continue
		}
		authority := line[strings.Index(line, "://")+3:]
		if i := strings.Index(authority, "/"); i >= 0 {
			authority = authority[:i]
		}
		if at := strings.Index(authority, "@"); at >= 0 {
			userinfo := authority[:at]
			if strings.Contains(userinfo, ":") {
				t.Errorf("credential-bearing userinfo in emitted URL: %q", line)
			}
		}
	}
}

// TestRepositoryToMarkdownOmitsEmptyCloneURLs verifies we do not emit empty
// labels when the API did not return clone URLs.
func TestRepositoryToMarkdownOmitsEmptyCloneURLs(t *testing.T) {
	repo := &Repository{
		Repository: &forgejo.Repository{FullName: "owner/repo-name"},
	}
	output := repo.ToMarkdown()
	if strings.Contains(output, "Clone (HTTPS):") || strings.Contains(output, "Clone (SSH):") {
		t.Errorf("empty clone URLs should be omitted, got: %q", output)
	}
}
