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
	_ resource.Resource                = &serviceAccountWorkspaceResource{}
	_ resource.ResourceWithConfigure   = &serviceAccountWorkspaceResource{}
	_ resource.ResourceWithImportState = &serviceAccountWorkspaceResource{}
)

// NewServiceAccountWorkspaceResource is the constructor registered with the
// provider.
func NewServiceAccountWorkspaceResource() resource.Resource {
	return &serviceAccountWorkspaceResource{}
}

type serviceAccountWorkspaceResource struct {
	client *client.Client
}

// serviceAccountWorkspaceResourceModel maps the resource schema to Go types.
type serviceAccountWorkspaceResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	WorkspaceRole    types.String `tfsdk:"workspace_role"`
	Implicit         types.Bool   `tfsdk:"implicit"`
	Type             types.String `tfsdk:"type"`
}

func (r *serviceAccountWorkspaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account_workspace"
}

func (r *serviceAccountWorkspaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An explicit membership of a Workload Identity Federation service account in a workspace, " +
			"with a workspace role. A service account must be a member of a workspace before federated tokens " +
			"can act in it. Every service account is implicitly a member of the organization's default " +
			"workspace, so manage this resource only for non-default workspaces.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier in the form `<service_account_id>/<workspace_id>`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_account_id": schema.StringAttribute{
				Description: "Identifier of the service account (`svac_...`). Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Description: "Identifier of the workspace (`wrkspc_...`) to add the service account to. Changing " +
					"this forces a new resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_role": schema.StringAttribute{
				Description: "Role granted to the service account within the workspace. One of `workspace_user`, " +
					"`workspace_developer`, `workspace_restricted_developer`, or `workspace_admin`. Service " +
					"accounts cannot hold the `workspace_billing` role.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"workspace_user",
						"workspace_developer",
						"workspace_restricted_developer",
						"workspace_admin",
					),
				},
			},
			"implicit": schema.BoolAttribute{
				Description: "Whether this is the implicit default-workspace membership every service account " +
					"has. Implicit memberships always have role `workspace_user` and cannot be removed.",
				Computed: true,
			},
			"type": schema.StringAttribute{
				Description:   "Object type, always `service_account_workspace_member`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *serviceAccountWorkspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serviceAccountWorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountWorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	membership, err := r.client.AddServiceAccountWorkspace(ctx, plan.ServiceAccountID.ValueString(), client.AddServiceAccountWorkspaceRequest{
		WorkspaceID:   plan.WorkspaceID.ValueString(),
		WorkspaceRole: plan.WorkspaceRole.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to add service account to workspace", err.Error())
		return
	}

	tflog.Trace(ctx, "added a service account to a workspace", map[string]any{
		"service_account_id": membership.ServiceAccountID,
		"workspace_id":       membership.WorkspaceID,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromServiceAccountWorkspace(membership))...)
}

func (r *serviceAccountWorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountWorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	membership, err := r.client.GetServiceAccountWorkspace(ctx, state.ServiceAccountID.ValueString(), state.WorkspaceID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The membership no longer exists; drop it from state so it is
			// recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read service account workspace membership", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromServiceAccountWorkspace(membership))...)
}

func (r *serviceAccountWorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountWorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-adding the service account with a new role replaces the existing
	// membership in place.
	membership, err := r.client.AddServiceAccountWorkspace(ctx, plan.ServiceAccountID.ValueString(), client.AddServiceAccountWorkspaceRequest{
		WorkspaceID:   plan.WorkspaceID.ValueString(),
		WorkspaceRole: plan.WorkspaceRole.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update service account workspace membership", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromServiceAccountWorkspace(membership))...)
}

func (r *serviceAccountWorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountWorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveServiceAccountWorkspace(ctx, state.ServiceAccountID.ValueString(), state.WorkspaceID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to remove service account from workspace", err.Error())
	}
}

func (r *serviceAccountWorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected import ID in the form \"service_account_id/workspace_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[1])...)
}

func modelFromServiceAccountWorkspace(m *client.ServiceAccountWorkspace) serviceAccountWorkspaceResourceModel {
	return serviceAccountWorkspaceResourceModel{
		ID:               types.StringValue(m.ServiceAccountID + "/" + m.WorkspaceID),
		ServiceAccountID: types.StringValue(m.ServiceAccountID),
		WorkspaceID:      types.StringValue(m.WorkspaceID),
		WorkspaceRole:    types.StringValue(m.WorkspaceRole),
		Implicit:         types.BoolValue(m.Implicit),
		Type:             types.StringValue(m.Type),
	}
}
