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
	_ resource.Resource                = &apiKeyResource{}
	_ resource.ResourceWithConfigure   = &apiKeyResource{}
	_ resource.ResourceWithImportState = &apiKeyResource{}
)

// NewAPIKeyResource is the constructor registered with the provider.
func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

type apiKeyResource struct {
	client *client.Client
}

// apiKeyResourceModel maps the resource schema to Go types.
// created_by is stored as a flattened "<type>:<id>" string to keep the schema
// simple and avoid a nested object attribute.
type apiKeyResourceModel struct {
	ID             types.String `tfsdk:"id"`
	APIKeyID       types.String `tfsdk:"api_key_id"`
	Name           types.String `tfsdk:"name"`
	Status         types.String `tfsdk:"status"`
	Type           types.String `tfsdk:"type"`
	WorkspaceID    types.String `tfsdk:"workspace_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	CreatedByID    types.String `tfsdk:"created_by_id"`
	CreatedByType  types.String `tfsdk:"created_by_type"`
	ExpiresAt      types.String `tfsdk:"expires_at"`
	PartialKeyHint types.String `tfsdk:"partial_key_hint"`
}

func (r *apiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "Adopts an existing Claude organization API key and manages its `name` and `status`.\n\n" +
			"API keys **cannot** be created or deleted via the Admin API; they must be created and " +
			"deleted in the [Anthropic Console](https://console.anthropic.com). This resource imports " +
			"an existing key by `api_key_id` and allows you to update its name and status (active, " +
			"inactive, or archived) through Terraform.\n\n" +
			"Destroying this resource removes it from Terraform state only — the API key continues " +
			"to exist in the Console.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Identifier of the API key (`apikey_...`). Equal to `api_key_id`."),
			"api_key_id": schema.StringAttribute{
				Description: "ID of the existing API key to manage (`apikey_...`). Changing this value " +
					"forces a new resource (the old key is dropped from state; no API delete is issued).",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Human-readable name of the API key. Updatable in place.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Status of the API key. One of `active`, `inactive`, or `archived`. " +
					"Updatable in place. Note: `expired` may appear as a read value when a key has " +
					"passed its expiry date, but it cannot be set via the API.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("active", "inactive", "archived"),
				},
			},
			"type":             computedString("Object type, always `api_key`."),
			"workspace_id":     computedString("ID of the workspace the API key belongs to, or empty for the default workspace."),
			"created_at":       computedString("RFC 3339 timestamp of when the API key was created."),
			"created_by_id":    computedString("ID of the actor (user or service account) that created the API key."),
			"created_by_type":  computedString("Type of the actor that created the API key (e.g. `user`)."),
			"expires_at":       computedString("RFC 3339 timestamp of when the API key expires, or empty if it never expires."),
			"partial_key_hint": computedString("Partially redacted hint showing the beginning and end of the raw API key value."),
		},
	}
}

func (r *apiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create adopts an existing API key. There is no create endpoint in the Admin
// API: keys are created only in the Anthropic Console. This method:
//  1. GETs the key to confirm it exists (404 → clear error message).
//  2. If name or status are explicitly configured, issues one UpdateAPIKey call.
//  3. Writes the resulting state.
func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Confirm the key exists.
	key, err := r.client.GetAPIKey(ctx, plan.APIKeyID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError(
				"API key not found",
				fmt.Sprintf(
					"No API key with id %q exists in this organization. "+
						"API keys must be created in the Anthropic Console "+
						"(https://console.anthropic.com) before they can be managed by Terraform.",
					plan.APIKeyID.ValueString(),
				),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to read API key", err.Error())
		return
	}

	tflog.Trace(ctx, "adopted an existing API key", map[string]any{"id": key.ID})

	// If the user explicitly set name or status, apply them immediately.
	updateReq := client.UpdateAPIKeyRequest{
		Name:   stringFromPlanIfSet(plan.Name),
		Status: stringFromPlanIfSet(plan.Status),
	}
	if updateReq.Name != "" || updateReq.Status != "" {
		key, err = r.client.UpdateAPIKey(ctx, key.ID, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Unable to update API key during adoption", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromAPIKey(key))...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.GetAPIKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The key no longer exists; drop it from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromAPIKey(key))...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateAPIKey(ctx, plan.ID.ValueString(), client.UpdateAPIKeyRequest{
		Name:   plan.Name.ValueString(),
		Status: plan.Status.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromAPIKey(updated))...)
}

// Delete is a no-op: the Admin API has no delete endpoint for API keys. Keys
// must be archived or deleted in the Anthropic Console. Terraform will remove
// this resource from state, but the key continues to exist in the Console.
func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "claude_api_key destroy is a no-op: the Admin API does not support deleting "+
		"API keys. The key still exists in the Anthropic Console and must be archived or removed there.",
		map[string]any{"id": state.ID.ValueString()},
	)

	resp.Diagnostics.AddWarning(
		"API key not deleted from Anthropic",
		fmt.Sprintf(
			"Terraform has removed API key %q from its state, but the key still exists in the "+
				"Anthropic Console. To fully remove it, archive or delete it at "+
				"https://console.anthropic.com.",
			state.ID.ValueString(),
		),
	)
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// modelFromAPIKey converts a client.APIKey to the Terraform state model.
func modelFromAPIKey(key *client.APIKey) apiKeyResourceModel {
	return apiKeyResourceModel{
		ID:             types.StringValue(key.ID),
		APIKeyID:       types.StringValue(key.ID),
		Name:           types.StringValue(key.Name),
		Status:         types.StringValue(key.Status),
		Type:           types.StringValue(key.Type),
		WorkspaceID:    optionalString(key.WorkspaceID),
		CreatedAt:      types.StringValue(key.CreatedAt),
		CreatedByID:    types.StringValue(key.CreatedBy.ID),
		CreatedByType:  types.StringValue(key.CreatedBy.Type),
		ExpiresAt:      optionalString(key.ExpiresAt),
		PartialKeyHint: types.StringValue(key.PartialKeyHint),
	}
}

// stringFromPlanIfSet returns the string value only when it is explicitly
// configured (non-null, non-unknown), so unset Optional+Computed attributes
// do not overwrite the server-side value during adoption.
func stringFromPlanIfSet(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}
