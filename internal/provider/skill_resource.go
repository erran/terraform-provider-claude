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
			"Claude can load on demand. Uploading `files` here seeds the skill's first version; changing " +
			"them uploads a new version (you can also add versions with `claude_skill_version`). The Skills " +
			"API is in beta. `display_title` has no update endpoint, so it is immutable once set.",
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
					"The Skills API has no update endpoint for it, so it is immutable once set: changing it " +
					"is rejected at plan time. Recreate the skill to change it.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					requireImmutableString(),
				},
			},
			"files": schema.MapAttribute{
				Description: "Files to upload as the skill's first version, keyed by path. All paths must share " +
					"one top-level directory and include a `SKILL.md` at its root (e.g. " +
					"`my-skill/SKILL.md`). Values are the file contents, typically read with the `file()` " +
					"function. Write-only: the API does not return file contents, so they are not restored on " +
					"import. Changing the files uploads a new version of the skill.",
				ElementType: types.StringType,
				Optional:    true,
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

	// Files are write-only; the API never returns them, so carry the prior
	// state value forward unchanged.
	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromSkill(skill, state.Files))...)
}

// Update uploads changed files as a new version of the skill rather than
// replacing it. display_title is immutable (enforced at plan time by
// requireImmutableString), so the only configurable change reaching here is to
// files.
func (r *skillResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state skillResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.Files.Equal(state.Files) {
		files, diags := skillFilesFromPlan(ctx, plan.Files)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if len(files) == 0 {
			resp.Diagnostics.AddError(
				"No files provided",
				"At least one file must be provided to upload a new version of the skill. "+
					"Include a SKILL.md at the root of a top-level directory (e.g. my-skill/SKILL.md).",
			)
			return
		}

		version, err := r.client.CreateSkillVersion(ctx, state.ID.ValueString(), files)
		if err != nil {
			resp.Diagnostics.AddError("Unable to create skill version", err.Error())
			return
		}

		tflog.Trace(ctx, "created a skill version", map[string]any{"id": version.ID, "skill_id": state.ID.ValueString()})
	}

	// Re-fetch the skill to pick up the new latest_version and updated_at.
	skill, err := r.client.GetSkill(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read skill", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromSkill(skill, plan.Files))...)
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
