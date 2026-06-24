// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                = &skillResource{}
	_ resource.ResourceWithConfigure   = &skillResource{}
	_ resource.ResourceWithImportState = &skillResource{}
)

// NewSkillResource is the constructor registered with the provider.
func NewSkillResource() resource.Resource {
	return &skillResource{}
}

type skillResource struct {
	client *client.Client
}

// skillResourceModel maps the resource schema to Go types.
type skillResourceModel struct {
	ID            types.String `tfsdk:"id"`
	DisplayTitle  types.String `tfsdk:"display_title"`
	Files         types.Map    `tfsdk:"files"`
	LatestVersion types.String `tfsdk:"latest_version"`
	Source        types.String `tfsdk:"source"`
	Type          types.String `tfsdk:"type"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

func (r *skillResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (r *skillResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{
			Description:   desc,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}

	resp.Schema = schema.Schema{
		Description: "A custom Agent Skill (`skill_...`): a reusable bundle of instructions and files that " +
			"Claude can load on demand. Uploading `files` here seeds the skill's first version; add further " +
			"versions with `claude_skill_version`. The Skills API is in beta. Because there is no update " +
			"endpoint, changing `display_title` or `files` replaces the skill.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the skill (`skill_...`).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_title": schema.StringAttribute{
				Description: "Human-readable label for the skill. Not included in the prompt sent to the model. " +
					"Changing it forces a new resource.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"files": schema.MapAttribute{
				Description: "Files to upload as the skill's first version, keyed by path. All paths must share " +
					"one top-level directory and include a `SKILL.md` at its root (e.g. " +
					"`my-skill/SKILL.md`). Values are the file contents, typically read with the `file()` " +
					"function. The API does not return file contents on read, but on import they are recovered " +
					"by downloading the skill's latest version content. Changing the files forces a new resource.",
				ElementType: types.StringType,
				Optional:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"latest_version": computedString("Identifier of the most recent version of the skill, or null if none has been created."),
			"source":         computedString("Source of the skill: `custom` for user-created skills, `anthropic` for Anthropic-provided ones."),
			"type":           computedString("Object type, always `skill`."),
			"created_at":     computedString("ISO 8601 timestamp of when the skill was created."),
			"updated_at":     computedString("ISO 8601 timestamp of when the skill was last updated."),
		},
	}
}

func (r *skillResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *skillResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	files, diags := skillFilesFromPlan(ctx, plan.Files)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(files) == 0 {
		resp.Diagnostics.AddError(
			"No files provided",
			"At least one file must be provided when creating a skill. "+
				"Include a SKILL.md at the root of a top-level directory (e.g. my-skill/SKILL.md).",
		)
		return
	}

	created, err := r.client.CreateSkill(ctx, plan.DisplayTitle.ValueString(), files)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create skill", err.Error())
		return
	}

	tflog.Trace(ctx, "created a skill", map[string]any{"id": created.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromSkill(created, plan.Files))...)
}

func (r *skillResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	skill, err := r.client.GetSkill(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The skill no longer exists; drop it from state so it is recreated
			// on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read skill", err.Error())
		return
	}

	// Files are write-only on create and read, so the API never returns them
	// alongside the skill. Normally carry the prior state value forward
	// unchanged, but on import (no prior files) recover them by downloading the
	// skill's latest version content.
	files := state.Files
	if files.IsNull() && skill.LatestVersion != "" {
		downloaded, err := r.client.DownloadSkillVersion(ctx, skill.ID, skill.LatestVersion)
		if err != nil {
			resp.Diagnostics.AddError("Unable to download skill files", err.Error())
			return
		}
		m, diags := skillFilesToMap(downloaded)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		files = m
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromSkill(skill, files))...)
}

// Update is a no-op: every configurable attribute forces replacement, so the
// framework never calls this with a changed plan. It re-persists state to keep
// the framework satisfied.
func (r *skillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *skillResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state skillResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteSkill(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete skill", err.Error())
	}
}

func (r *skillResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func modelFromSkill(s *client.Skill, files types.Map) skillResourceModel {
	return skillResourceModel{
		ID:            types.StringValue(s.ID),
		DisplayTitle:  optionalString(s.DisplayTitle),
		Files:         files,
		LatestVersion: optionalString(s.LatestVersion),
		Source:        types.StringValue(s.Source),
		Type:          types.StringValue(s.Type),
		CreatedAt:     types.StringValue(s.CreatedAt),
		UpdatedAt:     types.StringValue(s.UpdatedAt),
	}
}
