// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

// nameRegex enforces the Admin API resource name constraint: lowercase
// alphanumerics and hyphens, 1 to 255 characters.
var nameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

// Ensure the resource satisfies the expected interfaces.
var (
	_ resource.Resource                = &serviceAccountResource{}
	_ resource.ResourceWithConfigure   = &serviceAccountResource{}
	_ resource.ResourceWithImportState = &serviceAccountResource{}
)

// NewServiceAccountResource is the constructor registered with the provider.
func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

type serviceAccountResource struct {
	client *client.Client
}

// serviceAccountResourceModel maps the resource schema to Go types.
type serviceAccountResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	OrganizationRole types.String `tfsdk:"organization_role"`
	Type             types.String `tfsdk:"type"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *serviceAccountResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Workload Identity Federation service account: the non-human identity " +
			"(`svac_...`) that a federated token acts as.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the service account (`svac_...`).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the service account. Must match `^[a-z0-9-]+$`, be 1 to 255 " +
					"characters, and be unique within the organization.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
					stringvalidator.RegexMatches(nameRegex, "must contain only lowercase letters, digits, and hyphens"),
				},
			},
			"organization_role": schema.StringAttribute{
				Description: "Organization role granted to the service account. One of `developer` " +
					"or `admin`. Defaults to `developer`. A rule with `oauth_scope: org:admin` must " +
					"target a service account whose role is `admin`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("developer"),
				Validators: []validator.String{
					stringvalidator.OneOf("developer", "admin"),
				},
			},
			"type": schema.StringAttribute{
				Description: "Object type, always `service_account`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the service account was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *serviceAccountResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateServiceAccount(ctx, client.CreateServiceAccountRequest{
		Name:             plan.Name.ValueString(),
		OrganizationRole: plan.OrganizationRole.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create service account", err.Error())
		return
	}

	tflog.Trace(ctx, "created a service account", map[string]any{"id": created.ID})

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromServiceAccount(created))...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := r.client.GetServiceAccount(ctx, state.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			// The service account no longer exists; drop it from state so it
			// is recreated on the next apply.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read service account", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromServiceAccount(sa))...)
}

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan serviceAccountResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateServiceAccount(ctx, plan.ID.ValueString(), client.UpdateServiceAccountRequest{
		Name:             plan.Name.ValueString(),
		OrganizationRole: plan.OrganizationRole.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update service account", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, modelFromServiceAccount(updated))...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Deletion is a soft delete (archive). Archiving an already-archived
	// account is idempotent, but archiving fails while a live federation rule
	// still references the account; archive the rule first.
	if err := r.client.ArchiveServiceAccount(ctx, state.ID.ValueString()); err != nil {
		if client.NotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to archive service account", err.Error())
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func modelFromServiceAccount(sa *client.ServiceAccount) serviceAccountResourceModel {
	return serviceAccountResourceModel{
		ID:               types.StringValue(sa.ID),
		Name:             types.StringValue(sa.Name),
		OrganizationRole: types.StringValue(sa.OrganizationRole),
		Type:             types.StringValue(sa.Type),
		CreatedAt:        types.StringValue(sa.CreatedAt),
	}
}
