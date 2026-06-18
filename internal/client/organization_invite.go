// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const organizationInvitesPath = "/v1/organizations/invites"

// OrganizationInvite represents an invitation for a user to join the
// organization. Its id is prefixed with "invite_".
type OrganizationInvite struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	InvitedAt string `json:"invited_at"`
	ExpiresAt string `json:"expires_at"`
}

// CreateOrganizationInviteRequest is the body for creating an organization
// invite.
type CreateOrganizationInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// CreateOrganizationInvite creates a new organization invite.
func (c *Client) CreateOrganizationInvite(ctx context.Context, req CreateOrganizationInviteRequest) (*OrganizationInvite, error) {
	var out OrganizationInvite
	if err := c.Do(ctx, http.MethodPost, organizationInvitesPath, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetOrganizationInvite fetches a single organization invite by id.
func (c *Client) GetOrganizationInvite(ctx context.Context, id string) (*OrganizationInvite, error) {
	var out OrganizationInvite
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", organizationInvitesPath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteOrganizationInvite deletes an organization invite. Deleting a
// non-existent invite is treated as a no-op by the caller.
func (c *Client) DeleteOrganizationInvite(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%s", organizationInvitesPath, id), nil, nil)
}
