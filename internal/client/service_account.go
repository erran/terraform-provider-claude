// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const serviceAccountsPath = "/v1/organizations/service_accounts"

// ServiceAccount is a non-human identity that a federated token acts as. Its
// id is prefixed with "svac_".
type ServiceAccount struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	Name             string `json:"name"`
	OrganizationRole string `json:"organization_role"`
	CreatedAt        string `json:"created_at"`
}

// CreateServiceAccountRequest is the body for creating a service account.
type CreateServiceAccountRequest struct {
	Name             string `json:"name"`
	OrganizationRole string `json:"organization_role"`
}

// UpdateServiceAccountRequest is the body for updating a service account.
type UpdateServiceAccountRequest struct {
	Name             string `json:"name,omitempty"`
	OrganizationRole string `json:"organization_role,omitempty"`
}

// CreateServiceAccount creates a new service account.
func (c *Client) CreateServiceAccount(ctx context.Context, req CreateServiceAccountRequest) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.Do(ctx, http.MethodPost, serviceAccountsPath, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetServiceAccount fetches a single service account by id.
func (c *Client) GetServiceAccount(ctx context.Context, id string) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", serviceAccountsPath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateServiceAccount updates a service account in place.
func (c *Client) UpdateServiceAccount(ctx context.Context, id string, req UpdateServiceAccountRequest) (*ServiceAccount, error) {
	var out ServiceAccount
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s", serviceAccountsPath, id), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveServiceAccount soft-deletes a service account. Archiving is
// idempotent, but returns 400 while a live federation rule still references
// the account.
func (c *Client) ArchiveServiceAccount(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s/archive", serviceAccountsPath, id), nil, nil)
}
