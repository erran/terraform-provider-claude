// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"net/http"
)

// Organization represents the current Claude organization.
type Organization struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// GetOrganization fetches the current organization from the Admin API.
func (c *Client) GetOrganization(ctx context.Context) (*Organization, error) {
	var out Organization
	if err := c.Do(ctx, http.MethodGet, "/v1/organizations/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
