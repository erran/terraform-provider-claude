// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

// WorkspaceMember represents a user's membership in a Claude workspace.
type WorkspaceMember struct {
	Type          string `json:"type"`
	UserID        string `json:"user_id"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceRole string `json:"workspace_role"`
}

// AddWorkspaceMemberRequest is the body for adding a member to a workspace.
type AddWorkspaceMemberRequest struct {
	UserID        string `json:"user_id"`
	WorkspaceRole string `json:"workspace_role"`
}

// UpdateWorkspaceMemberRequest is the body for updating a workspace member's role.
type UpdateWorkspaceMemberRequest struct {
	WorkspaceRole string `json:"workspace_role"`
}

func workspaceMembersPath(workspaceID string) string {
	return fmt.Sprintf("/v1/organizations/workspaces/%s/members", workspaceID)
}

func workspaceMemberPath(workspaceID, userID string) string {
	return fmt.Sprintf("/v1/organizations/workspaces/%s/members/%s", workspaceID, userID)
}

// AddWorkspaceMember adds a user to a workspace with the given role.
func (c *Client) AddWorkspaceMember(ctx context.Context, workspaceID string, req AddWorkspaceMemberRequest) (*WorkspaceMember, error) {
	var out WorkspaceMember
	if err := c.Do(ctx, http.MethodPost, workspaceMembersPath(workspaceID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkspaceMember fetches a single workspace member by workspace and user id.
func (c *Client) GetWorkspaceMember(ctx context.Context, workspaceID, userID string) (*WorkspaceMember, error) {
	var out WorkspaceMember
	if err := c.Do(ctx, http.MethodGet, workspaceMemberPath(workspaceID, userID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWorkspaceMember updates a workspace member's role in place.
func (c *Client) UpdateWorkspaceMember(ctx context.Context, workspaceID, userID string, req UpdateWorkspaceMemberRequest) (*WorkspaceMember, error) {
	var out WorkspaceMember
	if err := c.Do(ctx, http.MethodPost, workspaceMemberPath(workspaceID, userID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWorkspaceMember removes a user from a workspace.
func (c *Client) DeleteWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	return c.Do(ctx, http.MethodDelete, workspaceMemberPath(workspaceID, userID), nil, nil)
}
