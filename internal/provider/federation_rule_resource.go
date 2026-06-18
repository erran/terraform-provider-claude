// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/erran/terraform-provider-claude/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &federationRuleResource{}
	_ resource.ResourceWithConfigure   = &federationRuleResource{}
	_ resource.ResourceWithImportState = &federationRuleResource{}
)

// NewFederationRuleResource is the constructor registered with the provider.
func NewFederationRuleResource() resource.Resource {
	return &federationRuleResource{}
}

type federationRuleResource struct {
	client *client.Client
}

type ruleMatchModel struct {
	SubjectPrefix types.String `tfsdk:"subject_prefix"`
	Audience      types.String `tfsdk:"audience"`
	Claims        types.Map    `tfsdk:"claims"`
	Condition     types.String `tfsdk:"condition"`
}

type ruleTargetModel struct {
	ServiceAccountID   types.String `tfsdk:"service_account_id"`
	Type               types.String `tfsdk:"type"`
	ServiceAccountName types.String `tfsdk:"service_account_name"`
}

type federationRuleResourceModel struct {
	ID                     types.String     `tfsdk:"id"`
	Name                   types.String     `tfsdk:"name"`
	Description            types.String     `tfsdk:"description"`
	IssuerID               types.String     `tfsdk:"issuer_id"`
	IssuerName             types.String     `tfsdk:"issuer_name"`
	OAuthScope             types.String     `tfsdk:"oauth_scope"`
	TokenLifetimeSeconds   types.Int64      `tfsdk:"token_lifetime_seconds"`
	AppliesToAllWorkspaces types.Bool       `tfsdk:"applies_to_all_workspaces"`
	WorkspaceID            types.String     `tfsdk:"workspace_id"`
	WorkspaceIDs           types.List       `tfsdk:"workspace_ids"`
	Match                  *ruleMatchModel  `tfsdk:"match"`
	Target                 *ruleTargetModel `tfsdk:"target"`
	Type                   types.String     `tfsdk:"type"`
	CreatedAt              types.String     `tfsdk:"created_at"`
	UpdatedAt              types.String     `tfsdk:"updated_at"`
	ArchivedAt             types.String     `tfsdk:"archived_at"`
}

func (r *federationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule"
}

func (r *federationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "A Workload Identity Federation rule (`fdrl_...`): binds an issuer to a service " +
			"account so that JWTs satisfying the match conditions can mint tokens for the target.\n\n" +
			"OAuth-authenticated callers may only manage rules whose `oauth_scope` is `workspace:developer` " +
			"or `workspace:inference`; other scopes (such as `org:admin`) must be managed in the Console.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier of the federation rule (`fdrl_...`)."),
			"name": schema.StringAttribute{
				Description: "Slug identifier. Must match `^[a-z0-9-]+$`, be 1 to 255 characters, and be " +
					"unique within the organization.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.RegexMatches(nameRegex, "must contain only lowercase letters, digits, and hyphens"),
				},
			},
			"description": schema.StringAttribute{
				Description: "Optional free-text description.",
				Optional:    true,
			},
			"issuer_id": schema.StringAttribute{
				Description: "Federation issuer (`fdis_...`) whose tokens this rule accepts. Changing this " +
					"forces a new rule, as a rule's issuer cannot be changed.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"oauth_scope": schema.StringAttribute{
				Description: "Space-separated OAuth scopes granted on the minted token, e.g. " +
					"`workspace:developer` (the typical default) or `workspace:inference`.",
				Required: true,
			},
			"token_lifetime_seconds": schema.Int64Attribute{
				Description: "Lifetime in seconds of tokens minted via this rule (60-86400). Defaults to 3600.",
				Optional:    true,
				Computed:    true,
				Validators:  []validator.Int64{int64validator.Between(60, 86400)},
			},
			"applies_to_all_workspaces": schema.BoolAttribute{
				Description: "Enable this rule for every workspace in the org (including ones created later). " +
					"Either this or `workspace_id` must be set.",
				Optional: true,
				Computed: true,
			},
			"workspace_id": schema.StringAttribute{
				Description: "Workspace (`wrkspc_...`) to enable this rule for at creation. Required unless " +
					"`applies_to_all_workspaces` is true. Changing it forces a new rule.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_ids": schema.ListAttribute{
				Description: "Workspaces this rule is enabled for.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"match": schema.SingleNestedAttribute{
				Description: "Conditions the verified JWT must satisfy. At least one of `subject_prefix`, " +
					"`claims`, or `condition` is required.",
				Required: true,
				Attributes: map[string]schema.Attribute{
					"subject_prefix": schema.StringAttribute{
						Description: "Match against the JWT `sub` claim. Exact match unless it ends with `*`, " +
							"which makes it a prefix match.",
						Optional: true,
					},
					"audience": schema.StringAttribute{
						Description: "Exact match against the `aud` claim. Overrides the issuer's default audience.",
						Optional:    true,
					},
					"claims": schema.MapAttribute{
						Description: "Exact-match `{claim: value}` pairs against top-level string claims.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"condition": schema.StringAttribute{
						Description: "CEL expression over the `claims` variable for logic the structural fields " +
							"cannot express.",
						Optional: true,
					},
				},
			},
			"target": schema.SingleNestedAttribute{
				Description: "Identity that tokens minted via this rule act as.",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"service_account_id": schema.StringAttribute{
						Description: "Service account (`svac_...`) to mint tokens for.",
						Required:    true,
					},
					"type": schema.StringAttribute{
						Description: "Target type, always `service_account`.",
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("service_account"),
					},
					"service_account_name": schema.StringAttribute{
						Description: "Service account's display name at read time.",
						Computed:    true,
					},
				},
			},
			"issuer_name": computedString("Issuer's display name at read time."),
			"type":        computedString("Object type, always `federation_rule`."),
			"created_at":  computedString("RFC 3339 timestamp of when the rule was created."),
			"updated_at":  computedString("RFC 3339 timestamp of when the rule was last updated."),
			"archived_at": computedString("RFC 3339 timestamp of when the rule was archived, if archived."),
		},
	}
}

func (r *federationRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *federationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan federationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.WorkspaceID.IsNull() && !plan.AppliesToAllWorkspaces.ValueBool() {
		resp.Diagnostics.AddError(
			"Missing workspace binding",
			"Either workspace_id or applies_to_all_workspaces must be set when creating a federation rule.",
		)
		return
	}

	match := r.buildMatch(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateFederationRule(ctx, client.CreateFederationRuleRequest{
		Name:                   plan.Name.ValueString(),
		IssuerID:               plan.IssuerID.ValueString(),
		Match:                  match,
		Target:                 r.buildTarget(plan),
		OAuthScope:             plan.OAuthScope.ValueString(),
		Description:            plan.Description.ValueString(),
		TokenLifetimeSeconds:   int64PointerFromPlan(plan.TokenLifetimeSeconds),
		AppliesToAllWorkspaces: boolPointerFromPlan(plan.AppliesToAllWorkspaces),
		WorkspaceID:            plan.WorkspaceID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create federation rule", err.Error())
		return
	}

	tflog.Trace(ctx, "created a federation rule", map[string]any{"id": created.ID})

	model, diags := r.modelFromRule(ctx, created)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *federationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state federationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.GetFederationRule(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read federation rule", err.Error())
		return
	}

	model, diags := r.modelFromRule(ctx, rule)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *federationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan federationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	match := r.buildMatch(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	description := plan.Description.ValueString()
	updated, err := r.client.UpdateFederationRule(ctx, plan.ID.ValueString(), client.UpdateFederationRuleRequest{
		Name:                   plan.Name.ValueString(),
		Match:                  match,
		Target:                 r.buildTarget(plan),
		OAuthScope:             plan.OAuthScope.ValueString(),
		Description:            &description,
		TokenLifetimeSeconds:   int64PointerFromPlan(plan.TokenLifetimeSeconds),
		AppliesToAllWorkspaces: boolPointerFromPlan(plan.AppliesToAllWorkspaces),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update federation rule", err.Error())
		return
	}

	model, diags := r.modelFromRule(ctx, updated)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *federationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state federationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.ArchiveFederationRule(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to archive federation rule", err.Error())
	}
}

func (r *federationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *federationRuleResource) buildMatch(ctx context.Context, plan federationRuleResourceModel, diags *diag.Diagnostics) *client.RuleMatch {
	if plan.Match == nil {
		return nil
	}

	match := &client.RuleMatch{
		SubjectPrefix: plan.Match.SubjectPrefix.ValueString(),
		Audience:      plan.Match.Audience.ValueString(),
		Condition:     plan.Match.Condition.ValueString(),
	}

	if !plan.Match.Claims.IsNull() && !plan.Match.Claims.IsUnknown() {
		claims := map[string]string{}
		diags.Append(plan.Match.Claims.ElementsAs(ctx, &claims, false)...)
		if len(claims) > 0 {
			match.Claims = claims
		}
	}

	if match.SubjectPrefix == "" && match.Condition == "" && len(match.Claims) == 0 {
		diags.AddAttributeError(path.Root("match"), "Empty match",
			"At least one of match.subject_prefix, match.claims, or match.condition is required.")
		return nil
	}

	return match
}

func (r *federationRuleResource) buildTarget(plan federationRuleResourceModel) *client.RuleTarget {
	if plan.Target == nil {
		return nil
	}
	return &client.RuleTarget{
		Type:             plan.Target.Type.ValueString(),
		ServiceAccountID: plan.Target.ServiceAccountID.ValueString(),
	}
}

func (r *federationRuleResource) modelFromRule(ctx context.Context, rule *client.FederationRule) (federationRuleResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	model := federationRuleResourceModel{
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
		model.Match = &ruleMatchModel{
			SubjectPrefix: optionalString(rule.Match.SubjectPrefix),
			Audience:      optionalString(rule.Match.Audience),
			Claims:        claims,
			Condition:     optionalString(rule.Match.Condition),
		}
	}

	if rule.Target != nil {
		model.Target = &ruleTargetModel{
			ServiceAccountID:   types.StringValue(rule.Target.ServiceAccountID),
			Type:               optionalString(rule.Target.Type),
			ServiceAccountName: optionalString(rule.Target.ServiceAccountName),
		}
	}

	return model, diags
}
