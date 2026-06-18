// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// jwtBearerGrant is the RFC 7523 grant type used to exchange a workload's
// OIDC JWT for an Anthropic access token.
const jwtBearerGrant = "urn:ietf:params:oauth:grant-type:jwt-bearer"

// TokenExchangeRequest holds the inputs for a Workload Identity Federation
// token exchange. The granted OAuth scope (e.g. org:admin) is determined by the
// federation rule identified by FederationRuleID, not by this request.
type TokenExchangeRequest struct {
	// Assertion is the OIDC JWT issued to the workload (for GitLab CI, the
	// id_token value).
	Assertion string
	// FederationRuleID selects the federation rule (fdrl_...) to evaluate the
	// assertion against. Its oauth_scope determines the minted token's scope.
	FederationRuleID string
	// OrganizationID is the Anthropic organization UUID.
	OrganizationID string
	// ServiceAccountID is the target service account (svac_...).
	ServiceAccountID string
	// WorkspaceID is optional; required only when the rule covers more than one
	// workspace.
	WorkspaceID string
}

// Valid reports whether the request has the fields required to attempt an
// exchange.
func (r TokenExchangeRequest) Valid() bool {
	return r.Assertion != "" && r.FederationRuleID != "" && r.OrganizationID != "" && r.ServiceAccountID != ""
}

// tokenExchangeBody is the JSON payload posted to /v1/oauth/token.
type tokenExchangeBody struct {
	GrantType        string `json:"grant_type"`
	Assertion        string `json:"assertion"`
	FederationRuleID string `json:"federation_rule_id"`
	OrganizationID   string `json:"organization_id"`
	ServiceAccountID string `json:"service_account_id"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
}

// tokenExchangeResponse follows RFC 6749 §5.1.
type tokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeToken performs the Workload Identity Federation token exchange,
// returning a short-lived Anthropic access token (sk-ant-oat01-...) bound to
// the federation rule's service account. baseURL may be empty for the default
// endpoint.
func ExchangeToken(ctx context.Context, httpClient *http.Client, baseURL, userAgent string, req TokenExchangeRequest) (string, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	payload, err := json.Marshal(tokenExchangeBody{
		GrantType:        jwtBearerGrant,
		Assertion:        req.Assertion,
		FederationRuleID: req.FederationRuleID,
		OrganizationID:   req.OrganizationID,
		ServiceAccountID: req.ServiceAccountID,
		WorkspaceID:      req.WorkspaceID,
	})
	if err != nil {
		return "", fmt.Errorf("encoding token exchange request: %w", err)
	}

	url := strings.TrimRight(baseURL, "/") + "/v1/oauth/token"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building token exchange request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	if userAgent != "" {
		httpReq.Header.Set("user-agent", userAgent)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("performing token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token exchange response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var out tokenExchangeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decoding token exchange response: %w", err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("token exchange response did not contain an access_token")
	}

	return out.AccessToken, nil
}
