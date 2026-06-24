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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the resource satisfies the expected interfaces.
var (
	_ resource.Resource                = &skillVersionResource{}
	_ resource.ResourceWithConfigure   = &skillVersionResource{}
	_ resource.ResourceWithImportState = &skillVersionResource{}
)

// NewSkillVersionResource is the constructor registered with the provider.
func NewSkillVersionResource() resource.Resource {
	return &skillVersionResource{}
}

type skillVersionResource struct {
	client *client.Client
}

// skillVersionResourceModel maps the resource schema to Go types.
type skillVersionResourceModel struct {
	ID          types.String `tfsdk:"id"`
	SkillID     types.String `tfsdk:"skill_id"`
	Files       types.Map    `tfsdk:"files"`
	Version     types.String `tfsdk:"version"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Directory   types.String `tfsdk:"directory"`
	Type        types.String `tfsdk:"type"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (r *skillVersionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill_version"
}

func (r *skillVersionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "A version of an Agent Skill (`skillver_...`): an immutable snapshot of a skill's files. " +
			"Each version is created by uploading a new set of files to an existing `claude_skill`. Versions " +
			"are immutable, so changing the `skill_id` or `files` forces a new resource. The Skills API is in " +
			"beta.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the skill version (`skillver_...`).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"skill_id": schema.StringAttribute{
				Description: "Identifier of the skill (`skill_...`) this version belongs to. Changing it forces a " +
					"new resource.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"files": schema.MapAttribute{
				Description: "Files to upload as this version, keyed by path. All paths must share one top-level " +
					"directory and include a `SKILL.md` at its root (e.g. `my-skill/SKILL.md`). Values are the " +
					"file contents, typically read with the `file()` function. Write-only: the API does not " +
					"return file contents, so they are not restored on import. Changing the files forces a new " +
					"resource.",
				ElementType: types.StringType,
				Required:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"version":     computedString("Version identifier, a Unix epoch timestamp (e.g. `1759178010641129`)."),
			"name":        computedString("Human-readable name of the version, extracted from the uploaded `SKILL.md`."),
			"description": computedString("Description of the version, extracted from the uploaded `SKILL.md`."),
			"directory":   computedString("Top-level directory name extracted from the uploaded files."),
			"type":        computedString("Object type, always `skill_version`."),
			"created_at":  computedString("ISO 8601 timestamp of when the version was created."),
		},
	}
}

func (r *skillVersionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *skillVersionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan skillVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	files, diags := skillFilesFromPlan(ctx, plan.Files)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateSkillVersion(ctx, plan.SkillID.ValueString(), files)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create skill version", err.Error())
		return
	}

	tflog.Trace(ctx, "created a skill version", map[string]any{"id": created.ID, "skill_id": created.SkillID})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromSkillVersion(created, plan.Files))...)
}

func (r *skillVersionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state skillVersionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	version, err := r.client.GetSkillVersion(ctx, state.SkillID.ValueString(), state.Version.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The version no longer exists; drop it from state so it is
			// recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read skill version", err.Error())
		return
	}

	// Files are write-only; the API never returns them, so carry the prior
	// state value forward unchanged.
	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromSkillVersion(version, state.Files))...)
}

// Update is a no-op: every configurable attribute forces replacement, so the
// framework never calls this with a changed plan. It re-persists state to keep
// the framework satisfied.
func (r *skillVersionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan skillVersionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *skillVersionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state skillVersionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSkillVersion(ctx, state.SkillID.ValueString(), state.Version.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete skill version", err.Error())
	}
}

func (r *skillVersionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			fmt.Sprintf("Expected import ID in the form \"skill_id/version\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("skill_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("version"), parts[1])...)
}

func modelFromSkillVersion(v *client.SkillVersion, files types.Map) skillVersionResourceModel {
	return skillVersionResourceModel{
		ID:          types.StringValue(v.ID),
		SkillID:     types.StringValue(v.SkillID),
		Files:       files,
		Version:     types.StringValue(v.Version),
		Name:        types.StringValue(v.Name),
		Description: types.StringValue(v.Description),
		Directory:   types.StringValue(v.Directory),
		Type:        types.StringValue(v.Type),
		CreatedAt:   types.StringValue(v.CreatedAt),
	}
}
