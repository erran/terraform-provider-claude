// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
)

const federationIssuersPath = "/v1/organizations/federation_issuers"

// JWKS is the discriminated union describing how Anthropic obtains an issuer's
// signing keys. Type is one of "discovery", "explicit_url", or "inline".
type JWKS struct {
	Type string `json:"type"`
	// URL is the JWKS endpoint for the "explicit_url" type.
	URL string `json:"url,omitempty"`
	// DiscoveryBase overrides the discovery URL for the "discovery" type.
	DiscoveryBase string `json:"discovery_base,omitempty"`
	// CACertPEM is an optional custom CA for TLS verification of the fetch.
	CACertPEM string `json:"ca_cert_pem,omitempty"`
	// Keys holds inline JWK objects for the "inline" type.
	Keys []map[string]any `json:"keys,omitempty"`
}

// FederationIssuer registers an external OIDC identity provider (fdis_...).
type FederationIssuer struct {
	ID                    string `json:"id"`
	Type                  string `json:"type"`
	Name                  string `json:"name"`
	IssuerURL             string `json:"issuer_url"`
	CheckJTI              bool   `json:"check_jti"`
	MaxJWTLifetimeSeconds int64  `json:"max_jwt_lifetime_seconds"`
	JWKS                  *JWKS  `json:"jwks,omitempty"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
	ArchivedAt            string `json:"archived_at,omitempty"`
}

// CreateFederationIssuerRequest is the body for creating an issuer.
type CreateFederationIssuerRequest struct {
	Name                  string `json:"name"`
	IssuerURL             string `json:"issuer_url"`
	CheckJTI              *bool  `json:"check_jti,omitempty"`
	MaxJWTLifetimeSeconds *int64 `json:"max_jwt_lifetime_seconds,omitempty"`
	JWKS                  *JWKS  `json:"jwks,omitempty"`
}

// UpdateFederationIssuerRequest is the body for updating an issuer. Setting
// JWKS replaces the entire JWKS configuration.
type UpdateFederationIssuerRequest struct {
	Name                  string `json:"name,omitempty"`
	IssuerURL             string `json:"issuer_url,omitempty"`
	CheckJTI              *bool  `json:"check_jti,omitempty"`
	MaxJWTLifetimeSeconds *int64 `json:"max_jwt_lifetime_seconds,omitempty"`
	JWKS                  *JWKS  `json:"jwks,omitempty"`
}

func (c *Client) CreateFederationIssuer(ctx context.Context, req CreateFederationIssuerRequest) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.Do(ctx, http.MethodPost, federationIssuersPath, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetFederationIssuer(ctx context.Context, id string) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.Do(ctx, http.MethodGet, fmt.Sprintf("%s/%s", federationIssuersPath, id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateFederationIssuer(ctx context.Context, id string, req UpdateFederationIssuerRequest) (*FederationIssuer, error) {
	var out FederationIssuer
	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s", federationIssuersPath, id), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveFederationIssuer soft-deletes an issuer. It is rejected with 400 while
// any live federation rule still references the issuer; archive those first.
func (c *Client) ArchiveFederationIssuer(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodPost, fmt.Sprintf("%s/%s/archive", federationIssuersPath, id), nil, nil)
}
