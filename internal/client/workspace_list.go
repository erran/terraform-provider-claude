// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// workspaceListResponse is the envelope returned by the list-workspaces endpoint.
type workspaceListResponse struct {
	Data     []Workspace `json:"data"`
	NextPage string      `json:"next_page"`
}

// ListWorkspaces fetches all workspaces in the organization, following
// pagination cursors until the server signals there are no more pages.
// When includeArchived is true, archived workspaces are included in the
// results.
func (c *Client) ListWorkspaces(ctx context.Context, includeArchived bool) ([]Workspace, error) {
	var all []Workspace
	cursor := ""

	for {
		q := url.Values{}
		q.Set("limit", "100")
		if cursor != "" {
			q.Set("page", cursor)
		}
		if includeArchived {
			q.Set("include_archived", "true")
		}

		path := fmt.Sprintf("%s?%s", workspacesPath, q.Encode())

		var page workspaceListResponse
		if err := c.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
			return nil, err
		}

		all = append(all, page.Data...)

		if page.NextPage == "" {
			break
		}
		cursor = page.NextPage
	}

	return all, nil
}
