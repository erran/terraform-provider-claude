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

// Ensure the data source satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &workspaceDataSource{}
	_ datasource.DataSourceWithConfigure = &workspaceDataSource{}
)

// NewWorkspaceDataSource is the constructor registered with the provider.
func NewWorkspaceDataSource() datasource.DataSource {
	return &workspaceDataSource{}
}

type workspaceDataSource struct {
	client *client.Client
}

// workspaceDataSourceModel maps the data source schema to Go types.
type workspaceDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	DisplayColor  types.String `tfsdk:"display_color"`
	CompartmentID types.String `tfsdk:"compartment_id"`
	ExternalKeyID types.String `tfsdk:"external_key_id"`
	Type          types.String `tfsdk:"type"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ArchivedAt    types.String `tfsdk:"archived_at"`
}

func (d *workspaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Claude organization workspace (`wrkspc_...`) by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the workspace (`wrkspc_...`) to look up.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the workspace.",
				Computed:    true,
			},
			"display_color": schema.StringAttribute{
				Description: "Hex color code representing the workspace in the Anthropic Console (e.g. `#6C5BB9`).",
				Computed:    true,
			},
			"compartment_id": schema.StringAttribute{
				Description: "Identifier for this workspace's encryption compartment, used when configuring a customer-managed encryption key (CMEK).",
				Computed:    true,
			},
			"external_key_id": schema.StringAttribute{
				Description: "ID of the customer-managed encryption key (CMEK) configuration attached to this workspace, or null if none.",
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "Object type, always `workspace`.",
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
	}
}

func (d *workspaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workspaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ws, err := d.client.GetWorkspace(ctx, config.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError("Workspace not found", err.Error())
			return
		}
		resp.Diagnostics.AddError("Unable to read workspace", err.Error())
		return
	}

	state := workspaceDataSourceModel{
		ID:            types.StringValue(ws.ID),
		Name:          types.StringValue(ws.Name),
		DisplayColor:  types.StringValue(ws.DisplayColor),
		CompartmentID: types.StringValue(ws.CompartmentID),
		ExternalKeyID: optionalString(ws.ExternalKeyID),
		Type:          types.StringValue(ws.Type),
		CreatedAt:     types.StringValue(ws.CreatedAt),
		ArchivedAt:    optionalString(ws.ArchivedAt),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
