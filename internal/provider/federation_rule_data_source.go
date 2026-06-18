// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

// Ensure the data source satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &federationRuleDataSource{}
	_ datasource.DataSourceWithConfigure = &federationRuleDataSource{}
)

// NewFederationRuleDataSource is the constructor registered with the provider.
func NewFederationRuleDataSource() datasource.DataSource {
	return &federationRuleDataSource{}
}

type federationRuleDataSource struct {
	client *client.Client
}

type ruleMatchDataModel struct {
	SubjectPrefix types.String `tfsdk:"subject_prefix"`
	Audience      types.String `tfsdk:"audience"`
	Claims        types.Map    `tfsdk:"claims"`
	Condition     types.String `tfsdk:"condition"`
}

type ruleTargetDataModel struct {
	ServiceAccountID   types.String `tfsdk:"service_account_id"`
	Type               types.String `tfsdk:"type"`
	ServiceAccountName types.String `tfsdk:"service_account_name"`
}

type federationRuleDataSourceModel struct {
	ID                     types.String         `tfsdk:"id"`
	Name                   types.String         `tfsdk:"name"`
	Description            types.String         `tfsdk:"description"`
	IssuerID               types.String         `tfsdk:"issuer_id"`
	IssuerName             types.String         `tfsdk:"issuer_name"`
	OAuthScope             types.String         `tfsdk:"oauth_scope"`
	TokenLifetimeSeconds   types.Int64          `tfsdk:"token_lifetime_seconds"`
	AppliesToAllWorkspaces types.Bool           `tfsdk:"applies_to_all_workspaces"`
	WorkspaceID            types.String         `tfsdk:"workspace_id"`
	WorkspaceIDs           types.List           `tfsdk:"workspace_ids"`
	Match                  *ruleMatchDataModel  `tfsdk:"match"`
	Target                 *ruleTargetDataModel `tfsdk:"target"`
	Type                   types.String         `tfsdk:"type"`
	CreatedAt              types.String         `tfsdk:"created_at"`
	UpdatedAt              types.String         `tfsdk:"updated_at"`
	ArchivedAt             types.String         `tfsdk:"archived_at"`
}

func (d *federationRuleDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule"
}

func (d *federationRuleDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description: desc,
			Computed:    true,
		}
	}

	resp.Schema = schema.Schema{
		Description: "Fetches a single Workload Identity Federation rule by its id (`fdrl_...`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the federation rule (`fdrl_...`).",
				Required:    true,
			},
			"name":        computedString("Slug identifier of the federation rule."),
			"description": computedString("Optional free-text description."),
			"issuer_id":   computedString("Federation issuer (`fdis_...`) whose tokens this rule accepts."),
			"issuer_name": computedString("Issuer's display name at read time."),
			"oauth_scope": computedString("Space-separated OAuth scopes granted on the minted token."),
			"token_lifetime_seconds": schema.Int64Attribute{
				Description: "Lifetime in seconds of tokens minted via this rule.",
				Computed:    true,
			},
			"applies_to_all_workspaces": schema.BoolAttribute{
				Description: "Whether this rule is enabled for every workspace in the org.",
				Computed:    true,
			},
			"workspace_id": computedString("Workspace (`wrkspc_...`) this rule is primarily associated with."),
			"workspace_ids": schema.ListAttribute{
				Description: "Workspaces this rule is enabled for.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"match": schema.SingleNestedAttribute{
				Description: "Conditions the verified JWT must satisfy.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"subject_prefix": computedString("Match against the JWT `sub` claim."),
					"audience":       computedString("Exact match against the `aud` claim."),
					"claims": schema.MapAttribute{
						Description: "Exact-match `{claim: value}` pairs against top-level string claims.",
						ElementType: types.StringType,
						Computed:    true,
					},
					"condition": computedString("CEL expression over the `claims` variable."),
				},
			},
			"target": schema.SingleNestedAttribute{
				Description: "Identity that tokens minted via this rule act as.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"service_account_id":   computedString("Service account (`svac_...`) to mint tokens for."),
					"type":                 computedString("Target type, always `service_account`."),
					"service_account_name": computedString("Service account's display name at read time."),
				},
			},
			"type":        computedString("Object type, always `federation_rule`."),
			"created_at":  computedString("RFC 3339 timestamp of when the rule was created."),
			"updated_at":  computedString("RFC 3339 timestamp of when the rule was last updated."),
			"archived_at": computedString("RFC 3339 timestamp of when the rule was archived, if archived."),
		},
	}
}

func (d *federationRuleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *federationRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config federationRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := d.client.GetFederationRule(ctx, config.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError(
				"Federation Rule Not Found",
				fmt.Sprintf("No federation rule with id %q was found.", config.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to read federation rule", err.Error())
		return
	}

	state, diags := d.modelFromRule(ctx, rule)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (d *federationRuleDataSource) modelFromRule(ctx context.Context, rule *client.FederationRule) (federationRuleDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := federationRuleDataSourceModel{
		ID:                     types.StringValue(rule.ID),
		Name:                   types.StringValue(rule.Name),
		Description:            optionalString(rule.Description),
		IssuerID:               types.StringValue(rule.IssuerID),
		IssuerName:             optionalString(rule.IssuerName),
		OAuthScope:             types.StringValue(rule.OAuthScope),
		TokenLifetimeSeconds:   types.Int64Value(rule.TokenLifetimeSeconds),
		AppliesToAllWorkspaces: types.BoolValue(rule.AppliesToAllWorkspaces),
		WorkspaceID:            optionalString(rule.WorkspaceID),
		Type:                   types.StringValue(rule.Type),
		CreatedAt:              types.StringValue(rule.CreatedAt),
		UpdatedAt:              types.StringValue(rule.UpdatedAt),
		ArchivedAt:             optionalString(rule.ArchivedAt),
	}

	workspaceIDs, listDiags := types.ListValueFrom(ctx, types.StringType, rule.WorkspaceIDs)
	diags.Append(listDiags...)
	model.WorkspaceIDs = workspaceIDs

	if rule.Match != nil {
		claims := types.MapNull(types.StringType)
		if len(rule.Match.Claims) > 0 {
			c, mapDiags := types.MapValueFrom(ctx, types.StringType, rule.Match.Claims)
			diags.Append(mapDiags...)
			claims = c
		}
		model.Match = &ruleMatchDataModel{
			SubjectPrefix: optionalString(rule.Match.SubjectPrefix),
			Audience:      optionalString(rule.Match.Audience),
			Claims:        claims,
			Condition:     optionalString(rule.Match.Condition),
		}
	}

	if rule.Target != nil {
		model.Target = &ruleTargetDataModel{
			ServiceAccountID:   types.StringValue(rule.Target.ServiceAccountID),
			Type:               optionalString(rule.Target.Type),
			ServiceAccountName: optionalString(rule.Target.ServiceAccountName),
		}
	}

	return model, diags
}
