// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const organizationRateLimitsPath = "/v1/organizations/rate_limits"

// RateLimit represents a single rate limit group returned by the Rate Limits
// API. Each group covers a category of resources (identified by GroupType) and
// carries the configured limiters in Limits.
//
// The API is read-only: rate limits cannot be created, updated, or deleted
// programmatically. To change a workspace's limits, use the Limits tab in the
// Claude Console.
type RateLimit struct {
	// Type is the object type: "rate_limit" for organization limits or
	// "workspace_rate_limit" for workspace overrides.
	Type string `json:"type"`
	// GroupType identifies the category of limits: "model_group", "batch",
	// "token_count", "files", "skills", or "web_search".
	GroupType string `json:"group_type"`
	// Models lists every model ID and alias that counts against a
	// "model_group" entry. It is null for all other group types.
	Models []string `json:"models"`
	// Limits is the set of configured limiters for the group.
	Limits []RateLimitValue `json:"limits"`
}

// RateLimitValue is a single configured limiter within a rate limit group.
type RateLimitValue struct {
	// Type is the limiter, such as "requests_per_minute",
	// "input_tokens_per_minute", "output_tokens_per_minute", or
	// "enqueued_batch_requests".
	Type string `json:"type"`
	// Value is the configured limit.
	Value int64 `json:"value"`
	// OrgLimit is the organization-level value for the same limiter on
	// workspace override responses, or null when the organization has no
	// configured limit for that limiter. It is always null for organization
	// rate limits.
	OrgLimit *int64 `json:"org_limit,omitempty"`
}

// rateLimitListResponse is the envelope returned by the rate limit endpoints.
type rateLimitListResponse struct {
	Data     []RateLimit `json:"data"`
	NextPage string      `json:"next_page"`
}

// ListOrganizationRateLimits fetches the rate limits applied at the
// organization level. When model is non-empty only the group containing that
// model ID or alias is returned (the API responds 404 if no group matches).
// When groupType is non-empty the response is restricted to that category.
func (c *Client) ListOrganizationRateLimits(ctx context.Context, model, groupType string) ([]RateLimit, error) {
	return c.listRateLimits(ctx, organizationRateLimitsPath, func(q url.Values) {
		if model != "" {
			q.Set("model", model)
		}
		if groupType != "" {
			q.Set("group_type", groupType)
		}
	})
}

// ListWorkspaceRateLimits fetches the rate limit overrides configured for a
// single workspace. The response includes only overrides; anything absent is
// inherited from the organization. When groupType is non-empty the response is
// restricted to that category.
func (c *Client) ListWorkspaceRateLimits(ctx context.Context, workspaceID, groupType string) ([]RateLimit, error) {
	path := fmt.Sprintf("%s/%s/rate_limits", workspacesPath, workspaceID)
	return c.listRateLimits(ctx, path, func(q url.Values) {
		if groupType != "" {
			q.Set("group_type", groupType)
		}
	})
}

// listRateLimits performs a paginated GET against a rate limit endpoint,
// following next_page cursors until the server signals there are no more
// pages. setParams contributes endpoint-specific query parameters.
func (c *Client) listRateLimits(ctx context.Context, basePath string, setParams func(url.Values)) ([]RateLimit, error) {
	var all []RateLimit
	cursor := ""

	for {
		q := url.Values{}
		setParams(q)
		if cursor != "" {
			q.Set("page", cursor)
		}

		path := basePath
		if encoded := q.Encode(); encoded != "" {
			path = fmt.Sprintf("%s?%s", basePath, encoded)
		}

		var page rateLimitListResponse
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
