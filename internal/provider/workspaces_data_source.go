// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

var (
	_ datasource.DataSource              = &workspacesDataSource{}
	_ datasource.DataSourceWithConfigure = &workspacesDataSource{}
)

// NewWorkspacesDataSource is the constructor registered with the provider.
func NewWorkspacesDataSource() datasource.DataSource { return &workspacesDataSource{} }

type workspacesDataSource struct{ client *client.Client }

// workspacesDataSourceModel is the top-level state model for the data source.
type workspacesDataSourceModel struct {
	IncludeArchived types.Bool       `tfsdk:"include_archived"`
	Workspaces      []workspaceModel `tfsdk:"workspaces"`
}

// workspaceModel maps a single Workspace API object to Terraform state.
type workspaceModel struct {
	ID            types.String `tfsdk:"id"`
	Type          types.String `tfsdk:"type"`
	Name          types.String `tfsdk:"name"`
	DisplayColor  types.String `tfsdk:"display_color"`
	CompartmentID types.String `tfsdk:"compartment_id"`
	ExternalKeyID types.String `tfsdk:"external_key_id"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ArchivedAt    types.String `tfsdk:"archived_at"`
}

func (d *workspacesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspaces"
}

func (d *workspacesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all workspaces in the Claude organization.",
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{
				Description: "When true, archived workspaces are included in the results. Defaults to false.",
				Optional:    true,
			},
			"workspaces": schema.ListNestedAttribute{
				Description: "The list of workspaces returned by the API.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Identifier of the workspace (`wrkspc_...`).",
							Computed:    true,
						},
						"type": schema.StringAttribute{
							Description: "Object type, always `workspace`.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Human-readable name of the workspace.",
							Computed:    true,
						},
						"display_color": schema.StringAttribute{
							Description: "Hex color string used to display the workspace in the UI.",
							Computed:    true,
						},
						"compartment_id": schema.StringAttribute{
							Description: "Compartment identifier associated with the workspace.",
							Computed:    true,
						},
						"external_key_id": schema.StringAttribute{
							Description: "External API key identifier, if one is set.",
							Computed:    true,
						},
						"created_at": schema.StringAttribute{
							Description: "RFC 3339 timestamp of when the workspace was created.",
							Computed:    true,
						},
						"archived_at": schema.StringAttribute{
							Description: "RFC 3339 timestamp of when the workspace was archived, or null if not archived.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *workspacesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *workspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workspacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	includeArchived := false
	if !config.IncludeArchived.IsNull() && !config.IncludeArchived.IsUnknown() {
		includeArchived = config.IncludeArchived.ValueBool()
	}

	workspaces, err := d.client.ListWorkspaces(ctx, includeArchived)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list workspaces", err.Error())
		return
	}

	models := make([]workspaceModel, len(workspaces))
	for i, w := range workspaces {
		models[i] = workspaceModel{
			ID:            types.StringValue(w.ID),
			Type:          types.StringValue(w.Type),
			Name:          types.StringValue(w.Name),
			DisplayColor:  types.StringValue(w.DisplayColor),
			CompartmentID: types.StringValue(w.CompartmentID),
			ExternalKeyID: optionalString(w.ExternalKeyID),
			CreatedAt:     types.StringValue(w.CreatedAt),
			ArchivedAt:    optionalString(w.ArchivedAt),
		}
	}

	state := workspacesDataSourceModel{
		IncludeArchived: config.IncludeArchived,
		Workspaces:      models,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
