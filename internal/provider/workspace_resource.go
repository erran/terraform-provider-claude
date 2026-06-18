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
	_ resource.Resource                = &workspaceResource{}
	_ resource.ResourceWithConfigure   = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

// NewWorkspaceResource is the constructor registered with the provider.
func NewWorkspaceResource() resource.Resource {
	return &workspaceResource{}
}

type workspaceResource struct {
	client *client.Client
}

// workspaceResourceModel maps the resource schema to Go types.
type workspaceResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	DisplayColor  types.String `tfsdk:"display_color"`
	CompartmentID types.String `tfsdk:"compartment_id"`
	ExternalKeyID types.String `tfsdk:"external_key_id"`
	Type          types.String `tfsdk:"type"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ArchivedAt    types.String `tfsdk:"archived_at"`
}

func (r *workspaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *workspaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "A Claude organization workspace (`wrkspc_...`): an isolated environment " +
			"for API keys, usage limits, and members within an organization.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier of the workspace (`wrkspc_...`)."),
			"name": schema.StringAttribute{
				Description: "Name of the workspace. Must match `^[a-z0-9-]+$`, be 1 to 255 " +
					"characters, and be unique within the organization.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.RegexMatches(nameRegex, "must contain only lowercase letters, digits, and hyphens"),
				},
			},
			"display_color": schema.StringAttribute{
				Description: "Hex color code representing the workspace in the Anthropic Console " +
					"(e.g. `#6C5BB9`). Set by the API; not configurable.",
				Computed: true,
			},
			"compartment_id": computedString("Identifier for this workspace's encryption compartment, " +
				"used when configuring a customer-managed encryption key (CMEK)."),
			"external_key_id": schema.StringAttribute{
				Description: "ID of the customer-managed encryption key (CMEK) configuration to use " +
					"for this workspace. Requires CMEK to be enabled for the organization. This " +
					"field is write-once: once attached it cannot be detached or replaced.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type":       computedString("Object type, always `workspace`."),
			"created_at": computedString("RFC 3339 timestamp of when the workspace was created."),
			"archived_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the workspace was archived, or null if not archived.",
				Computed:    true,
			},
		},
	}
}

func (r *workspaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateWorkspace(ctx, client.CreateWorkspaceRequest{
		Name:          plan.Name.ValueString(),
		ExternalKeyID: plan.ExternalKeyID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create workspace", err.Error())
		return
	}

	tflog.Trace(ctx, "created a workspace", map[string]any{"id": created.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromWorkspace(created))...)
}

func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ws, err := r.client.GetWorkspace(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The workspace no longer exists; drop it from state so it is
			// recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read workspace", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromWorkspace(ws))...)
}

func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateWorkspace(ctx, plan.ID.ValueString(), client.UpdateWorkspaceRequest{
		Name:          plan.Name.ValueString(),
		ExternalKeyID: plan.ExternalKeyID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update workspace", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromWorkspace(updated))...)
}

func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deletion is a soft delete (archive). The default workspace cannot be
	// archived; ensure any dependent members are removed first.
	if err := r.client.ArchiveWorkspace(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to archive workspace", err.Error())
	}
}

func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func modelFromWorkspace(ws *client.Workspace) workspaceResourceModel {
	return workspaceResourceModel{
		ID:            types.StringValue(ws.ID),
		Name:          types.StringValue(ws.Name),
		DisplayColor:  types.StringValue(ws.DisplayColor),
		CompartmentID: types.StringValue(ws.CompartmentID),
		ExternalKeyID: optionalString(ws.ExternalKeyID),
		Type:          types.StringValue(ws.Type),
		CreatedAt:     types.StringValue(ws.CreatedAt),
		ArchivedAt:    optionalString(ws.ArchivedAt),
	}
}
