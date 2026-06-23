// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ServiceAccountWorkspace is a service account's membership in a single
// workspace. Every service account has an implicit membership in the
// organization's default workspace (Implicit is true); explicit memberships in
// other workspaces are added through this sub-resource.
type ServiceAccountWorkspace struct {
	Type             string `json:"type"`
	ServiceAccountID string `json:"service_account_id"`
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceRole    string `json:"workspace_role"`
	Implicit         bool   `json:"implicit"`
	CreatedByActorID string `json:"created_by_actor_id,omitempty"`
}

// AddServiceAccountWorkspaceRequest is the body for adding a service account to
// a workspace.
type AddServiceAccountWorkspaceRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceRole string `json:"workspace_role"`
}

// serviceAccountWorkspaceListResponse is the envelope returned by the list
// endpoint.
type serviceAccountWorkspaceListResponse struct {
	Data     []ServiceAccountWorkspace `json:"data"`
	NextPage string                    `json:"next_page"`
}

func serviceAccountWorkspacesPath(serviceAccountID string) string {
	return fmt.Sprintf("%s/%s/workspaces", serviceAccountsPath, serviceAccountID)
}

func serviceAccountWorkspacePath(serviceAccountID, workspaceID string) string {
	return fmt.Sprintf("%s/%s/workspaces/%s", serviceAccountsPath, serviceAccountID, workspaceID)
}

// AddServiceAccountWorkspace adds a service account to a workspace with the
// given role. If the service account is already an explicit member its role is
// replaced, so the call doubles as an update.
func (c *Client) AddServiceAccountWorkspace(ctx context.Context, serviceAccountID string, req AddServiceAccountWorkspaceRequest) (*ServiceAccountWorkspace, error) {
	var out ServiceAccountWorkspace
	if err := c.Do(ctx, http.MethodPost, serviceAccountWorkspacesPath(serviceAccountID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListServiceAccountWorkspaces returns every workspace the service account is a
// member of, following pagination cursors. The implicit default-workspace
// membership is included.
func (c *Client) ListServiceAccountWorkspaces(ctx context.Context, serviceAccountID string) ([]ServiceAccountWorkspace, error) {
	var all []ServiceAccountWorkspace
	cursor := ""

	for {
		q := url.Values{}
		q.Set("limit", "100")
		if cursor != "" {
			q.Set("page", cursor)
		}

		path := fmt.Sprintf("%s?%s", serviceAccountWorkspacesPath(serviceAccountID), q.Encode())

		var page serviceAccountWorkspaceListResponse
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

// GetServiceAccountWorkspace returns the service account's membership in a
// single workspace, or an *APIError with a 404 status if no such membership
// exists. There is no per-membership GET endpoint, so it filters the list.
func (c *Client) GetServiceAccountWorkspace(ctx context.Context, serviceAccountID, workspaceID string) (*ServiceAccountWorkspace, error) {
	memberships, err := c.ListServiceAccountWorkspaces(ctx, serviceAccountID)
	if err != nil {
		return nil, err
	}
	for i := range memberships {
		if memberships[i].WorkspaceID == workspaceID {
			return &memberships[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Body: fmt.Sprintf("service account %s is not a member of workspace %s", serviceAccountID, workspaceID)}
}

// RemoveServiceAccountWorkspace removes a service account from a workspace.
// Removal is idempotent. Removing the implicit default-workspace membership is
// a no-op server-side.
func (c *Client) RemoveServiceAccountWorkspace(ctx context.Context, serviceAccountID, workspaceID string) error {
	return c.Do(ctx, http.MethodDelete, serviceAccountWorkspacePath(serviceAccountID, workspaceID), nil, nil)
}
