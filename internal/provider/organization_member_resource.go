// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

// Ensure the resource satisfies the expected interfaces.
var (
	_ resource.Resource                = &organizationMemberResource{}
	_ resource.ResourceWithConfigure   = &organizationMemberResource{}
	_ resource.ResourceWithImportState = &organizationMemberResource{}
)

// NewOrganizationMemberResource is the constructor registered with the provider.
func NewOrganizationMemberResource() resource.Resource {
	return &organizationMemberResource{}
}

type organizationMemberResource struct {
	client *client.Client
}

// organizationMemberResourceModel maps the resource schema to Go types.
type organizationMemberResourceModel struct {
	ID      types.String `tfsdk:"id"`
	UserID  types.String `tfsdk:"user_id"`
	Role    types.String `tfsdk:"role"`
	Email   types.String `tfsdk:"email"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	AddedAt types.String `tfsdk:"added_at"`
}

func (r *organizationMemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_member"
}

func (r *organizationMemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "Manages the organization role of an EXISTING organization member (`user_...`).\n\n" +
			"Users cannot be created via the API — they join an organization by accepting an invite. " +
			"This resource ADOPTS a user who is already a member and manages their role going forward. " +
			"Destroying this resource removes the user from the organization. " +
			"Note: admin users cannot be removed via the API.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier of the organization member (`user_...`). Equal to `user_id`."),
			"user_id": schema.StringAttribute{
				Description: "ID of the existing organization member to manage (`user_...`). " +
					"The user must already be a member of the organization (accepted an invite). " +
					"Changing this value forces a new resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Description: "Organization role of the member. One of `user`, `claude_code_user`, " +
					"`developer`, `billing`, or `admin`. Note: the role cannot be updated to or from " +
					"`admin` via the API.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("user", "claude_code_user", "developer", "billing", "admin"),
				},
			},
			"email":    computedString("Email address of the organization member."),
			"name":     computedString("Display name of the organization member."),
			"type":     computedString("Object type, always `user`."),
			"added_at": computedString("RFC 3339 timestamp of when the user joined the organization."),
		},
	}
}

func (r *organizationMemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create adopts an existing organization member by setting their role to the
// desired value. There is no create endpoint in the API — users join
// organizations by accepting an invite — so Create is implemented as an Update
// followed by a Read. If the user does not exist (404), an actionable error is
// returned explaining that the user must already be a member.
func (r *organizationMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := plan.UserID.ValueString()

	member, err := r.client.UpdateOrganizationMember(ctx, userID, client.UpdateOrganizationMemberRequest{
		Role: plan.Role.ValueString(),
	})
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError(
				"Organization member not found",
				fmt.Sprintf(
					"User %q is not a member of the organization. Users cannot be created via the API — "+
						"they must first accept an organization invite. Once the user has joined, "+
						"re-run `terraform apply` to adopt them under Terraform management.",
					userID,
				),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to set organization member role", err.Error())
		return
	}

	tflog.Trace(ctx, "adopted organization member", map[string]any{"id": member.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromOrganizationMember(member))...)
}

func (r *organizationMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.GetOrganizationMember(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The member has left or been removed; drop from state so the next
			// apply reports the drift.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read organization member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromOrganizationMember(member))...)
}

func (r *organizationMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.UpdateOrganizationMember(ctx, plan.UserID.ValueString(), client.UpdateOrganizationMemberRequest{
		Role: plan.Role.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update organization member role", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromOrganizationMember(member))...)
}

func (r *organizationMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteOrganizationMember(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			// Already removed; treat as success.
			return
		}
		resp.Diagnostics.AddError("Unable to remove organization member", err.Error())
	}
}

func (r *organizationMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// id == user_id, so passing through the id also populates user_id via Read.
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func modelFromOrganizationMember(m *client.OrganizationMember) organizationMemberResourceModel {
	return organizationMemberResourceModel{
		ID:      types.StringValue(m.ID),
		UserID:  types.StringValue(m.ID),
		Role:    types.StringValue(m.Role),
		Email:   types.StringValue(m.Email),
		Name:    types.StringValue(m.Name),
		Type:    types.StringValue(m.Type),
		AddedAt: types.StringValue(m.AddedAt),
	}
}
