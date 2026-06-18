// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

var (
	_ resource.Resource                = &federationIssuerResource{}
	_ resource.ResourceWithConfigure   = &federationIssuerResource{}
	_ resource.ResourceWithImportState = &federationIssuerResource{}
)

// NewFederationIssuerResource is the constructor registered with the provider.
func NewFederationIssuerResource() resource.Resource {
	return &federationIssuerResource{}
}

type federationIssuerResource struct {
	client *client.Client
}

type federationIssuerResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	IssuerURL             types.String `tfsdk:"issuer_url"`
	CheckJTI              types.Bool   `tfsdk:"check_jti"`
	MaxJWTLifetimeSeconds types.Int64  `tfsdk:"max_jwt_lifetime_seconds"`

	// jwks is flattened into prefixed attributes to keep the discriminated
	// union easy to express in HCL.
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

func (r *federationIssuerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_issuer"
}

func (r *federationIssuerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "A Workload Identity Federation issuer (`fdis_...`): an external OIDC identity " +
			"provider that Anthropic trusts for the RFC 7523 jwt-bearer grant.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier of the federation issuer (`fdis_...`)."),
			"name": schema.StringAttribute{
				Description: "Slug identifier. Must match `^[a-z0-9-]+$`, be 1 to 255 characters, and be " +
					"unique within the organization.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.RegexMatches(nameRegex, "must contain only lowercase letters, digits, and hyphens"),
				},
			},
			"issuer_url": schema.StringAttribute{
				Description: "The `iss` claim value incoming JWTs must match exactly, e.g. " +
					"`https://token.actions.githubusercontent.com`.",
				Required: true,
			},
			"check_jti": schema.BoolAttribute{
				Description: "Whether the jwt-bearer exchange enforces JTI single-use (replay protection) for " +
					"tokens from this issuer. Defaults to `true`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"max_jwt_lifetime_seconds": schema.Int64Attribute{
				Description: "Maximum allowed iat→exp spread for assertions from this issuer (1-176400 seconds). " +
					"Defaults to 3600.",
				Optional: true,
				Computed: true,
			},
			"jwks_type": schema.StringAttribute{
				Description: "How signing keys are obtained: `discovery` (OIDC discovery, the default), " +
					"`explicit_url` (a fixed JWKS endpoint), or `inline` (a static key set).",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("discovery"),
				Validators: []validator.String{
					stringvalidator.OneOf("discovery", "explicit_url", "inline"),
				},
			},
			"jwks_url": schema.StringAttribute{
				Description: "JWKS endpoint URL. Required when `jwks_type` is `explicit_url`.",
				Optional:    true,
			},
			"jwks_discovery_base": schema.StringAttribute{
				Description: "Discovery base URL, set when it differs from `issuer_url`. Only for `jwks_type` `discovery`.",
				Optional:    true,
			},
			"jwks_ca_cert_pem": schema.StringAttribute{
				Description: "Optional custom CA (PEM) for TLS verification of the JWKS fetch. Only for " +
					"`discovery` and `explicit_url`.",
				Optional: true,
			},
			"jwks_inline_keys": schema.StringAttribute{
				Description: "A JSON array of JWK objects. Required when `jwks_type` is `inline`.",
				Optional:    true,
			},
			"type":        computedString("Object type, always `federation_issuer`."),
			"created_at":  computedString("RFC 3339 timestamp of when the issuer was created."),
			"updated_at":  computedString("RFC 3339 timestamp of when the issuer was last updated."),
			"archived_at": computedString("RFC 3339 timestamp of when the issuer was archived, if archived."),
		},
	}
}

func (r *federationIssuerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *federationIssuerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan federationIssuerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jwks := r.buildJWKS(plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateFederationIssuer(ctx, client.CreateFederationIssuerRequest{
		Name:                  plan.Name.ValueString(),
		IssuerURL:             plan.IssuerURL.ValueString(),
		CheckJTI:              plan.CheckJTI.ValueBoolPointer(),
		MaxJWTLifetimeSeconds: int64PointerFromPlan(plan.MaxJWTLifetimeSeconds),
		JWKS:                  jwks,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create federation issuer", err.Error())
		return
	}

	tflog.Trace(ctx, "created a federation issuer", map[string]any{"id": created.ID})

	state, diags := r.modelFromIssuer(created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *federationIssuerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state federationIssuerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	issuer, err := r.client.GetFederationIssuer(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read federation issuer", err.Error())
		return
	}

	model, diags := r.modelFromIssuer(issuer)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *federationIssuerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan federationIssuerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jwks := r.buildJWKS(plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateFederationIssuer(ctx, plan.ID.ValueString(), client.UpdateFederationIssuerRequest{
		Name:                  plan.Name.ValueString(),
		IssuerURL:             plan.IssuerURL.ValueString(),
		CheckJTI:              plan.CheckJTI.ValueBoolPointer(),
		MaxJWTLifetimeSeconds: int64PointerFromPlan(plan.MaxJWTLifetimeSeconds),
		JWKS:                  jwks,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update federation issuer", err.Error())
		return
	}

	model, diags := r.modelFromIssuer(updated)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *federationIssuerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state federationIssuerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.ArchiveFederationIssuer(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to archive federation issuer", err.Error())
	}
}

func (r *federationIssuerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildJWKS assembles the JWKS union from the flattened plan attributes,
// validating the fields required for the selected type.
func (r *federationIssuerResource) buildJWKS(plan federationIssuerResourceModel, diags *diag.Diagnostics) *client.JWKS {
	jwksType := plan.JWKSType.ValueString()
	if jwksType == "" {
		return nil
	}

	jwks := &client.JWKS{
		Type:          jwksType,
		URL:           plan.JWKSURL.ValueString(),
		DiscoveryBase: plan.JWKSDiscoveryBase.ValueString(),
		CACertPEM:     plan.JWKSCACertPEM.ValueString(),
	}

	switch jwksType {
	case "explicit_url":
		if jwks.URL == "" {
			diags.AddAttributeError(path.Root("jwks_url"), "Missing jwks_url",
				"jwks_url is required when jwks_type is \"explicit_url\".")
			return nil
		}
	case "inline":
		raw := plan.JWKSInlineKeys.ValueString()
		if raw == "" {
			diags.AddAttributeError(path.Root("jwks_inline_keys"), "Missing jwks_inline_keys",
				"jwks_inline_keys is required when jwks_type is \"inline\".")
			return nil
		}
		var keys []map[string]any
		if err := json.Unmarshal([]byte(raw), &keys); err != nil {
			diags.AddAttributeError(path.Root("jwks_inline_keys"), "Invalid jwks_inline_keys",
				fmt.Sprintf("jwks_inline_keys must be a JSON array of JWK objects: %s", err))
			return nil
		}
		jwks.Keys = keys
	}

	return jwks
}

func (r *federationIssuerResource) modelFromIssuer(issuer *client.FederationIssuer) (federationIssuerResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := federationIssuerResourceModel{
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
