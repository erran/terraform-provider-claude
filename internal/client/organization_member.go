// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const organizationUsersPath = "/v1/organizations/users"

// OrganizationMember represents an existing user within the organization. Its
// id is prefixed with "user_".
type OrganizationMember struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	AddedAt string `json:"added_at"`
}

// UpdateOrganizationMemberRequest is the body for updating an organization
// member's role. The role cannot be set to "admin" via the API.
type UpdateOrganizationMemberRequest struct {
	Role string `json:"role"`
}

// GetOrganizationMember fetches a single organization member by user id.
func (c *Client) GetOrganizationMember(ctx context.Context, userID string) (*OrganizationMember, error) {
	var out OrganizationMember
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", organizationUsersPath, userID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateOrganizationMember updates the role of an existing organization member
// in place. This is also used during Create to set the desired role on a user
// who has already joined via an invite.
func (c *Client) UpdateOrganizationMember(ctx context.Context, userID string, req UpdateOrganizationMemberRequest) (*OrganizationMember, error) {
	var out OrganizationMember
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s", organizationUsersPath, userID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteOrganizationMember removes a user from the organization. Note that
// admin users cannot be removed via the API. A 404 response indicates the
// member has already been removed; callers may treat that as success.
func (c *Client) DeleteOrganizationMember(ctx context.Context, userID string) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("%s/%s", organizationUsersPath, userID), nil, nil)
}
