// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"strings"

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
	_ resource.Resource                = &workspaceMemberResource{}
	_ resource.ResourceWithConfigure   = &workspaceMemberResource{}
	_ resource.ResourceWithImportState = &workspaceMemberResource{}
)

// NewWorkspaceMemberResource is the constructor registered with the provider.
func NewWorkspaceMemberResource() resource.Resource {
	return &workspaceMemberResource{}
}

type workspaceMemberResource struct {
	client *client.Client
}

// workspaceMemberResourceModel maps the resource schema to Go types.
type workspaceMemberResourceModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	UserID        types.String `tfsdk:"user_id"`
	WorkspaceRole types.String `tfsdk:"workspace_role"`
	Type          types.String `tfsdk:"type"`
}

func (r *workspaceMemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_member"
}

func (r *workspaceMemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "A workspace member: a user assigned to a Claude workspace with a specific role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier of the workspace member in the form `<workspace_id>/<user_id>`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Description: "Identifier of the workspace (`wrkspc_...`). Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": schema.StringAttribute{
				Description: "Identifier of the user to add to the workspace. Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_role": schema.StringAttribute{
				Description: "Role granted to the user within the workspace. One of `workspace_user`, " +
					"`workspace_developer`, `workspace_admin`, or `workspace_billing`.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"workspace_user",
						"workspace_developer",
						"workspace_admin",
						"workspace_billing",
					),
				},
			},
			"type": computedString("Object type, always `workspace_member`."),
		},
	}
}

func (r *workspaceMemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workspaceMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.AddWorkspaceMember(ctx, plan.WorkspaceID.ValueString(), client.AddWorkspaceMemberRequest{
		UserID:        plan.UserID.ValueString(),
		WorkspaceRole: plan.WorkspaceRole.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to add workspace member", err.Error())
		return
	}

	tflog.Trace(ctx, "added a workspace member", map[string]any{
		"workspace_id": member.WorkspaceID,
		"user_id":      member.UserID,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromWorkspaceMember(member))...)
}

func (r *workspaceMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.GetWorkspaceMember(ctx, state.WorkspaceID.ValueString(), state.UserID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The workspace member no longer exists; drop it from state so it
			// is recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read workspace member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromWorkspaceMember(member))...)
}

func (r *workspaceMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.UpdateWorkspaceMember(ctx, plan.WorkspaceID.ValueString(), plan.UserID.ValueString(), client.UpdateWorkspaceMemberRequest{
		WorkspaceRole: plan.WorkspaceRole.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update workspace member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromWorkspaceMember(member))...)
}

func (r *workspaceMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWorkspaceMember(ctx, state.WorkspaceID.ValueString(), state.UserID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete workspace member", err.Error())
	}
}

func (r *workspaceMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected import ID in the form \"workspace_id/user_id\", got: %q", req.ID),
		)
		return
	}

	workspaceID := parts[0]
	userID := parts[1]
	compositeID := req.ID

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), compositeID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), workspaceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), userID)...)
}

func modelFromWorkspaceMember(m *client.WorkspaceMember) workspaceMemberResourceModel {
	return workspaceMemberResourceModel{
		ID:            types.StringValue(m.WorkspaceID + "/" + m.UserID),
		WorkspaceID:   types.StringValue(m.WorkspaceID),
		UserID:        types.StringValue(m.UserID),
		WorkspaceRole: types.StringValue(m.WorkspaceRole),
		Type:          types.StringValue(m.Type),
	}
}
