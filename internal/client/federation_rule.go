// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const federationRulesPath = "/v1/organizations/federation_rules"

// RuleMatch holds the conditions a verified JWT must satisfy. At least one of
// SubjectPrefix, Claims, or Condition is required.
type RuleMatch struct {
	SubjectPrefix string            `json:"subject_prefix,omitempty"`
	Audience      string            `json:"audience,omitempty"`
	Claims        map[string]string `json:"claims,omitempty"`
	Condition     string            `json:"condition,omitempty"`
}

// RuleTarget is the identity tokens minted via the rule act as. Currently
// always a service_account target.
type RuleTarget struct {
	Type             string `json:"type,omitempty"`
	ServiceAccountID string `json:"service_account_id"`
	// ServiceAccountName is read-only; ignored on writes.
	ServiceAccountName string `json:"service_account_name,omitempty"`
}

// FederationRule binds an issuer to a service account (fdrl_...).
type FederationRule struct {
	ID                     string      `json:"id"`
	Type                   string      `json:"type"`
	Name                   string      `json:"name"`
	Description            string      `json:"description,omitempty"`
	IssuerID               string      `json:"issuer_id"`
	IssuerName             string      `json:"issuer_name,omitempty"`
	Match                  *RuleMatch  `json:"match,omitempty"`
	Target                 *RuleTarget `json:"target,omitempty"`
	OAuthScope             string      `json:"oauth_scope"`
	TokenLifetimeSeconds   int64       `json:"token_lifetime_seconds"`
	AppliesToAllWorkspaces bool        `json:"applies_to_all_workspaces"`
	WorkspaceID            string      `json:"workspace_id,omitempty"`
	WorkspaceIDs           []string    `json:"workspace_ids,omitempty"`
	CreatedAt              string      `json:"created_at"`
	UpdatedAt              string      `json:"updated_at"`
	ArchivedAt             string      `json:"archived_at,omitempty"`
}

// CreateFederationRuleRequest is the body for creating a rule. Either
// WorkspaceID or AppliesToAllWorkspaces must be set.
type CreateFederationRuleRequest struct {
	Name                   string      `json:"name"`
	IssuerID               string      `json:"issuer_id"`
	Match                  *RuleMatch  `json:"match"`
	Target                 *RuleTarget `json:"target"`
	OAuthScope             string      `json:"oauth_scope"`
	Description            string      `json:"description,omitempty"`
	TokenLifetimeSeconds   *int64      `json:"token_lifetime_seconds,omitempty"`
	AppliesToAllWorkspaces *bool       `json:"applies_to_all_workspaces,omitempty"`
	WorkspaceID            string      `json:"workspace_id,omitempty"`
}

// UpdateFederationRuleRequest is the body for updating a rule. The issuer of a
// rule cannot be changed.
type UpdateFederationRuleRequest struct {
	Name                   string      `json:"name,omitempty"`
	Match                  *RuleMatch  `json:"match,omitempty"`
	Target                 *RuleTarget `json:"target,omitempty"`
	OAuthScope             string      `json:"oauth_scope,omitempty"`
	Description            *string     `json:"description,omitempty"`
	TokenLifetimeSeconds   *int64      `json:"token_lifetime_seconds,omitempty"`
	AppliesToAllWorkspaces *bool       `json:"applies_to_all_workspaces,omitempty"`
}

func (c *Client) CreateFederationRule(ctx context.Context, req CreateFederationRuleRequest) (*FederationRule, error) {
	var out FederationRule
	if err := c.Do(ctx, http.MethodPost, federationRulesPath, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetFederationRule(ctx context.Context, id string) (*FederationRule, error) {
	var out FederationRule
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", federationRulesPath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateFederationRule(ctx context.Context, id string, req UpdateFederationRuleRequest) (*FederationRule, error) {
	var out FederationRule
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s", federationRulesPath, id), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveFederationRule soft-deletes a rule. Archive rules before archiving the
// issuers or service accounts they reference.
func (c *Client) ArchiveFederationRule(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s/archive", federationRulesPath, id), nil, nil)
}
