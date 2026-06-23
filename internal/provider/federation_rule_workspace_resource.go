// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/erran/terraform-provider-claude/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the resource satisfies the expected interfaces.
var (
	_ resource.Resource                = &federationRuleWorkspaceResource{}
	_ resource.ResourceWithConfigure   = &federationRuleWorkspaceResource{}
	_ resource.ResourceWithImportState = &federationRuleWorkspaceResource{}
)

// NewFederationRuleWorkspaceResource is the constructor registered with the
// provider.
func NewFederationRuleWorkspaceResource() resource.Resource {
	return &federationRuleWorkspaceResource{}
}

type federationRuleWorkspaceResource struct {
	client *client.Client
}

// federationRuleWorkspaceResourceModel maps the resource schema to Go types.
type federationRuleWorkspaceResourceModel struct {
	ID               types.String `tfsdk:"id"`
	FederationRuleID types.String `tfsdk:"federation_rule_id"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
	CreatedAt        types.String `tfsdk:"created_at"`
	Type             types.String `tfsdk:"type"`
}

func (r *federationRuleWorkspaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule_workspace"
}

func (r *federationRuleWorkspaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "Enables a Workload Identity Federation rule in an additional workspace. The rule's " +
			"initial workspace is set on the `claude_federation_rule` resource (`workspace_id` or " +
			"`applies_to_all_workspaces`); use this resource to enable the rule in further workspaces. Do not " +
			"use it together with a rule that has `applies_to_all_workspaces = true`. OAuth-authenticated " +
			"callers may only manage workspaces of rules whose `oauth_scope` is `workspace:developer` or " +
			"`workspace:inference`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Composite identifier in the form `<federation_rule_id>/<workspace_id>`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"federation_rule_id": schema.StringAttribute{
				Description: "Identifier of the federation rule (`fdrl_...`). Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Description: "Identifier of the workspace (`wrkspc_...`) to enable the rule in. Changing this " +
					"forces a new resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_name": computedString("Display name of the workspace."),
			"created_at":     computedString("Timestamp when the rule was enabled for the workspace."),
			"type":           computedString("Object type, always `federation_rule_workspace`."),
		},
	}
}

func (r *federationRuleWorkspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *federationRuleWorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan federationRuleWorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enablement, err := r.client.AddFederationRuleWorkspace(ctx, plan.FederationRuleID.ValueString(), plan.WorkspaceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to enable federation rule in workspace", err.Error())
		return
	}

	tflog.Trace(ctx, "enabled a federation rule in a workspace", map[string]any{
		"federation_rule_id": enablement.FederationRuleID,
		"workspace_id":       enablement.WorkspaceID,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromFederationRuleWorkspace(enablement))...)
}

func (r *federationRuleWorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state federationRuleWorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enablement, err := r.client.GetFederationRuleWorkspace(ctx, state.FederationRuleID.ValueString(), state.WorkspaceID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The enablement no longer exists; drop it from state so it is
			// recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read federation rule workspace", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromFederationRuleWorkspace(enablement))...)
}

// Update is a no-op: every configurable attribute forces replacement, so the
// framework never calls this with a changed plan. It re-persists state to keep
// the framework satisfied.
func (r *federationRuleWorkspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan federationRuleWorkspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *federationRuleWorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state federationRuleWorkspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveFederationRuleWorkspace(ctx, state.FederationRuleID.ValueString(), state.WorkspaceID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to disable federation rule in workspace", err.Error())
	}
}

func (r *federationRuleWorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected import ID in the form \"federation_rule_id/workspace_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("federation_rule_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[1])...)
}

func modelFromFederationRuleWorkspace(w *client.FederationRuleWorkspace) federationRuleWorkspaceResourceModel {
	return federationRuleWorkspaceResourceModel{
		ID:               types.StringValue(w.FederationRuleID + "/" + w.WorkspaceID),
		FederationRuleID: types.StringValue(w.FederationRuleID),
		WorkspaceID:      types.StringValue(w.WorkspaceID),
		WorkspaceName:    optionalString(w.WorkspaceName),
		CreatedAt:        types.StringValue(w.CreatedAt),
		Type:             types.StringValue(w.Type),
	}
}
