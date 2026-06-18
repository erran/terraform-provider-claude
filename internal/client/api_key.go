// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const apiKeysPath = "/v1/organizations/api_keys"

// APIKeyActor identifies the user or service account that created an API key.
type APIKeyActor struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// APIKey represents a Claude organization API key as returned by the Admin API.
// Keys are created and deleted only via the Anthropic Console; the API only
// supports reading and updating name/status.
type APIKey struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"`
	Name           string      `json:"name"`
	Status         string      `json:"status"`
	WorkspaceID    string      `json:"workspace_id"`
	CreatedAt      string      `json:"created_at"`
	CreatedBy      APIKeyActor `json:"created_by"`
	ExpiresAt      string      `json:"expires_at"`
	PartialKeyHint string      `json:"partial_key_hint"`
}

// UpdateAPIKeyRequest is the body for the update-API-key endpoint.
// Both fields are optional; omit a field to leave it unchanged.
type UpdateAPIKeyRequest struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

// GetAPIKey fetches a single API key by id (e.g. "apikey_...").
func (c *Client) GetAPIKey(ctx context.Context, id string) (*APIKey, error) {
	var out APIKey
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", apiKeysPath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAPIKey updates the name and/or status of an existing API key in place.
func (c *Client) UpdateAPIKey(ctx context.Context, id string, req UpdateAPIKeyRequest) (*APIKey, error) {
	var out APIKey
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s", apiKeysPath, id), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
