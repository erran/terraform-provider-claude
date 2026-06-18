// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/erran/terraform-provider-claude/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the resource satisfies the expected interfaces.
var (
	_ resource.Resource                = &organizationInviteResource{}
	_ resource.ResourceWithConfigure   = &organizationInviteResource{}
	_ resource.ResourceWithImportState = &organizationInviteResource{}
)

// NewOrganizationInviteResource is the constructor registered with the provider.
func NewOrganizationInviteResource() resource.Resource {
	return &organizationInviteResource{}
}

type organizationInviteResource struct {
	client *client.Client
}

// organizationInviteResourceModel maps the resource schema to Go types.
type organizationInviteResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Type      types.String `tfsdk:"type"`
	Email     types.String `tfsdk:"email"`
	Role      types.String `tfsdk:"role"`
	Status    types.String `tfsdk:"status"`
	InvitedAt types.String `tfsdk:"invited_at"`
	ExpiresAt types.String `tfsdk:"expires_at"`
}

func (r *organizationInviteResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_invite"
}

func (r *organizationInviteResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "An organization invite (`invite_...`): an email invitation for a user to join " +
			"the organization with a given role. Invites cannot be updated; any change to " +
			"`email` or `role` will destroy and recreate the resource.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier of the invite (`invite_...`)."),
			"email": schema.StringAttribute{
				Description: "Email address of the user being invited.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Description: "Organization role to grant to the invited user. One of `user`, " +
					"`developer`, `billing`, or `claude_code_user`. Cannot be `admin` at " +
					"invite time.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("user", "developer", "billing", "claude_code_user"),
				},
			},
			"type": computedString("Object type, always `invite`."),
			"status": schema.StringAttribute{
				Description: "Current status of the invite. One of `pending`, `accepted`, `expired`, or `deleted`.",
				Computed:    true,
			},
			"invited_at": computedString("RFC 3339 timestamp of when the invite was created."),
			"expires_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the invite expires.",
				Computed:    true,
			},
		},
	}
}

func (r *organizationInviteResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *organizationInviteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationInviteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateOrganizationInvite(ctx, client.CreateOrganizationInviteRequest{
		Email: plan.Email.ValueString(),
		Role:  plan.Role.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create organization invite", err.Error())
		return
	}

	tflog.Trace(ctx, "created an organization invite", map[string]any{"id": created.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromOrganizationInvite(created))...)
}

func (r *organizationInviteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	invite, err := r.client.GetOrganizationInvite(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The invite no longer exists; drop it from state so it is
			// recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read organization invite", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromOrganizationInvite(invite))...)
}

// Update is not reachable in practice because both writable attributes
// (email and role) carry RequiresReplace, so the framework will always
// destroy and recreate the resource. It is implemented as a no-op that
// copies the plan to state to satisfy the resource.Resource interface.
func (r *organizationInviteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationInviteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationInviteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationInviteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteOrganizationInvite(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete organization invite", err.Error())
	}
}

func (r *organizationInviteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func modelFromOrganizationInvite(invite *client.OrganizationInvite) organizationInviteResourceModel {
	return organizationInviteResourceModel{
		ID:        types.StringValue(invite.ID),
		Type:      types.StringValue(invite.Type),
		Email:     types.StringValue(invite.Email),
		Role:      types.StringValue(invite.Role),
		Status:    types.StringValue(invite.Status),
		InvitedAt: types.StringValue(invite.InvitedAt),
		ExpiresAt: types.StringValue(invite.ExpiresAt),
	}
}
