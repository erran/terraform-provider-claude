// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

var (
	_ datasource.DataSource              = &federationIssuerDataSource{}
	_ datasource.DataSourceWithConfigure = &federationIssuerDataSource{}
)

// NewFederationIssuerDataSource is the constructor registered with the provider.
func NewFederationIssuerDataSource() datasource.DataSource {
	return &federationIssuerDataSource{}
}

type federationIssuerDataSource struct {
	client *client.Client
}

type federationIssuerDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	IssuerURL             types.String `tfsdk:"issuer_url"`
	CheckJTI              types.Bool   `tfsdk:"check_jti"`
	MaxJWTLifetimeSeconds types.Int64  `tfsdk:"max_jwt_lifetime_seconds"`

	// jwks is flattened into prefixed attributes.
	JWKSType          types.String `tfsdk:"jwks_type"`
	JWKSURL           types.String `tfsdk:"jwks_url"`
	JWKSDiscoveryBase types.String `tfsdk:"jwks_discovery_base"`
	JWKSCACertPEM     types.String `tfsdk:"jwks_ca_cert_pem"`
	JWKSInlineKeys    types.String `tfsdk:"jwks_inline_keys"`

	Type       types.String `tfsdk:"type"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
	ArchivedAt types.String `tfsdk:"archived_at"`
}

func (d *federationIssuerDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_issuer"
}

func (d *federationIssuerDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Workload Identity Federation issuer (`fdis_...`) by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the federation issuer (`fdis_...`).",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Slug identifier of the federation issuer.",
				Computed:    true,
			},
			"issuer_url": schema.StringAttribute{
				Description: "The `iss` claim value incoming JWTs must match exactly.",
				Computed:    true,
			},
			"check_jti": schema.BoolAttribute{
				Description: "Whether the jwt-bearer exchange enforces JTI single-use (replay protection).",
				Computed:    true,
			},
			"max_jwt_lifetime_seconds": schema.Int64Attribute{
				Description: "Maximum allowed iat→exp spread for assertions from this issuer.",
				Computed:    true,
			},
			"jwks_type": schema.StringAttribute{
				Description: "How signing keys are obtained: `discovery`, `explicit_url`, or `inline`.",
				Computed:    true,
			},
			"jwks_url": schema.StringAttribute{
				Description: "JWKS endpoint URL, set when `jwks_type` is `explicit_url`.",
				Computed:    true,
			},
			"jwks_discovery_base": schema.StringAttribute{
				Description: "Discovery base URL, set when it differs from `issuer_url`.",
				Computed:    true,
			},
			"jwks_ca_cert_pem": schema.StringAttribute{
				Description: "Optional custom CA (PEM) for TLS verification of the JWKS fetch.",
				Computed:    true,
			},
			"jwks_inline_keys": schema.StringAttribute{
				Description: "A JSON array of JWK objects, set when `jwks_type` is `inline`.",
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "Object type, always `federation_issuer`.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the issuer was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the issuer was last updated.",
				Computed:    true,
			},
			"archived_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the issuer was archived, if archived.",
				Computed:    true,
			},
		},
	}
}

func (d *federationIssuerDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *federationIssuerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config federationIssuerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	issuer, err := d.client.GetFederationIssuer(ctx, config.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError(
				"Federation Issuer Not Found",
				fmt.Sprintf("No federation issuer found with id %q.", config.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to read federation issuer", err.Error())
		return
	}

	model, diags := modelFromFederationIssuer(issuer)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func modelFromFederationIssuer(issuer *client.FederationIssuer) (federationIssuerDataSourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := federationIssuerDataSourceModel{
		ID:                    types.StringValue(issuer.ID),
		Name:                  types.StringValue(issuer.Name),
		IssuerURL:             types.StringValue(issuer.IssuerURL),
		CheckJTI:              types.BoolValue(issuer.CheckJTI),
		MaxJWTLifetimeSeconds: types.Int64Value(issuer.MaxJWTLifetimeSeconds),
		Type:                  types.StringValue(issuer.Type),
		CreatedAt:             types.StringValue(issuer.CreatedAt),
		UpdatedAt:             types.StringValue(issuer.UpdatedAt),
		ArchivedAt:            optionalString(issuer.ArchivedAt),
		// Default the flattened jwks attributes; overwritten below if present.
		JWKSType:          types.StringNull(),
		JWKSURL:           types.StringNull(),
		JWKSDiscoveryBase: types.StringNull(),
		JWKSCACertPEM:     types.StringNull(),
		JWKSInlineKeys:    types.StringNull(),
	}

	if issuer.JWKS != nil {
		model.JWKSType = types.StringValue(issuer.JWKS.Type)
		model.JWKSURL = optionalString(issuer.JWKS.URL)
		model.JWKSDiscoveryBase = optionalString(issuer.JWKS.DiscoveryBase)
		model.JWKSCACertPEM = optionalString(issuer.JWKS.CACertPEM)
		if len(issuer.JWKS.Keys) > 0 {
			encoded, err := json.Marshal(issuer.JWKS.Keys)
			if err != nil {
				diags.AddError("Unable to encode inline JWKS keys", err.Error())
				return model, diags
			}
			model.JWKSInlineKeys = types.StringValue(string(encoded))
		}
	}

	return model, diags
}
