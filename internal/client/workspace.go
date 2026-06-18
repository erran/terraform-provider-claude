// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const workspacesPath = "/v1/organizations/workspaces"

// Workspace represents a Claude organization workspace (`wrkspc_...`).
type Workspace struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	DisplayColor  string `json:"display_color"`
	CompartmentID string `json:"compartment_id"`
	ExternalKeyID string `json:"external_key_id,omitempty"`
	CreatedAt     string `json:"created_at"`
	ArchivedAt    string `json:"archived_at,omitempty"`
}

// CreateWorkspaceRequest is the body for creating a workspace.
type CreateWorkspaceRequest struct {
	Name          string            `json:"name"`
	ExternalKeyID string            `json:"external_key_id,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// UpdateWorkspaceRequest is the body for updating a workspace.
type UpdateWorkspaceRequest struct {
	Name          string            `json:"name,omitempty"`
	ExternalKeyID string            `json:"external_key_id,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

// CreateWorkspace creates a new workspace.
func (c *Client) CreateWorkspace(ctx context.Context, req CreateWorkspaceRequest) (*Workspace, error) {
	var out Workspace
	if err := c.Do(ctx, http.MethodPost, workspacesPath, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWorkspace fetches a single workspace by id.
func (c *Client) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	var out Workspace
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", workspacesPath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWorkspace updates a workspace in place.
func (c *Client) UpdateWorkspace(ctx context.Context, id string, req UpdateWorkspaceRequest) (*Workspace, error) {
	var out Workspace
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s", workspacesPath, id), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveWorkspace soft-deletes a workspace. The default workspace cannot be
// archived; archived workspaces reject member mutations with 400.
func (c *Client) ArchiveWorkspace(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s/archive", workspacesPath, id), nil, nil)
}
