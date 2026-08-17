// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
//
// Copyright © 2025 Ronmi Ren <ronmi.ren@gmail.com>

package types

// MyIdentity is a git author/committer identity. It mirrors the SDK's Identity
// type, redeclared here so the custom multi-file endpoint does not have to
// import the SDK into this package.
type MyIdentity struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// MyChangeFileOperation describes a single file operation inside a multi-file
// commit.
//
// Operation is one of "create", "update" or "delete". ContentBase64 is required
// for create and update; SHA is required for update and delete.
type MyChangeFileOperation struct {
	Operation     string `json:"operation"`
	Path          string `json:"path"`
	ContentBase64 string `json:"content,omitempty"`
	SHA           string `json:"sha,omitempty"`
	FromPath      string `json:"from_path,omitempty"`
}

// MyChangeFilesOptions is the request body of the multi-file commit endpoint.
// Used by endpoint:
//   - POST /repos/{owner}/{repo}/contents
//
// The Forgejo SDK has no binding for this endpoint, so it is issued directly.
type MyChangeFilesOptions struct {
	Files         []MyChangeFileOperation `json:"files"`
	Message       string                  `json:"message,omitempty"`
	BranchName    string                  `json:"branch,omitempty"`
	NewBranchName string                  `json:"new_branch,omitempty"`
	Author        *MyIdentity             `json:"author,omitempty"`
	Committer     *MyIdentity             `json:"committer,omitempty"`
}

// MyFileCommit is the commit created by a multi-file commit.
type MyFileCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Message string `json:"message"`
}

// MyChangedFile is one entry of the multi-file commit response. Deleted files
// are reported as null by the API and are dropped while decoding.
type MyChangedFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	SHA  string `json:"sha"`
	Type string `json:"type"`
}

// MyFilesResponse is the response of the multi-file commit endpoint.
type MyFilesResponse struct {
	Commit *MyFileCommit    `json:"commit"`
	Files  []*MyChangedFile `json:"files"`
}
