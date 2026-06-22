// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// FederationRuleWorkspace is a single workspace a federation rule is enabled
// in. It records an explicit per-workspace enablement; rules created with
// applies_to_all_workspaces or a legacy single workspace_id expose those on the
// rule itself rather than here.
type FederationRuleWorkspace struct {
	Type             string `json:"type"`
	FederationRuleID string `json:"federation_rule_id"`
	WorkspaceID      string `json:"workspace_id"`
	WorkspaceName    string `json:"workspace_name,omitempty"`
	CreatedAt        string `json:"created_at"`
	CreatedByActorID string `json:"created_by_actor_id,omitempty"`
}

// AddFederationRuleWorkspaceRequest is the body for enabling a rule in a
// workspace.
type AddFederationRuleWorkspaceRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// federationRuleWorkspaceListResponse is the envelope returned by the list
// endpoint. The endpoint returns every enablement in one response, so NextPage
// is always empty.
type federationRuleWorkspaceListResponse struct {
	Data     []FederationRuleWorkspace `json:"data"`
	NextPage string                    `json:"next_page"`
}

func federationRuleWorkspacesPath(ruleID string) string {
	return fmt.Sprintf("%s/%s/workspaces", federationRulesPath, ruleID)
}

func federationRuleWorkspacePath(ruleID, workspaceID string) string {
	return fmt.Sprintf("%s/%s/workspaces/%s", federationRulesPath, ruleID, workspaceID)
}

// AddFederationRuleWorkspace enables a federation rule in a workspace.
// Enablement is idempotent; re-enabling returns the existing enablement.
func (c *Client) AddFederationRuleWorkspace(ctx context.Context, ruleID, workspaceID string) (*FederationRuleWorkspace, error) {
	var out FederationRuleWorkspace
	if err := c.Do(ctx, http.MethodPost, federationRuleWorkspacesPath(ruleID), AddFederationRuleWorkspaceRequest{WorkspaceID: workspaceID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListFederationRuleWorkspaces returns every workspace the rule is explicitly
// enabled in. The endpoint does not paginate, but the cursor loop is kept for
// consistency with the other list methods.
func (c *Client) ListFederationRuleWorkspaces(ctx context.Context, ruleID string) ([]FederationRuleWorkspace, error) {
	var all []FederationRuleWorkspace
	cursor := ""

	for {
		q := url.Values{}
		q.Set("limit", "100")
		if cursor != "" {
			q.Set("page", cursor)
		}

		path := fmt.Sprintf("%s?%s", federationRuleWorkspacesPath(ruleID), q.Encode())

		var page federationRuleWorkspaceListResponse
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

// GetFederationRuleWorkspace returns a single rule-workspace enablement, or an
// *APIError with a 404 status if the rule is not enabled in the workspace.
// There is no per-enablement GET endpoint, so it filters the list.
func (c *Client) GetFederationRuleWorkspace(ctx context.Context, ruleID, workspaceID string) (*FederationRuleWorkspace, error) {
	enablements, err := c.ListFederationRuleWorkspaces(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	for i := range enablements {
		if enablements[i].WorkspaceID == workspaceID {
			return &enablements[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Body: fmt.Sprintf("federation rule %s is not enabled in workspace %s", ruleID, workspaceID)}
}

// RemoveFederationRuleWorkspace disables a federation rule in a workspace.
// Removal is idempotent.
func (c *Client) RemoveFederationRuleWorkspace(ctx context.Context, ruleID, workspaceID string) error {
	return c.Do(ctx, http.MethodDelete, federationRuleWorkspacePath(ruleID, workspaceID), nil, nil)
}
