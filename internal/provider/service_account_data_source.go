// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/erran/terraform-provider-claude/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the data source satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &serviceAccountDataSource{}
	_ datasource.DataSourceWithConfigure = &serviceAccountDataSource{}
)

// NewServiceAccountDataSource is the constructor registered with the provider.
func NewServiceAccountDataSource() datasource.DataSource {
	return &serviceAccountDataSource{}
}

type serviceAccountDataSource struct {
	client *client.Client
}

// serviceAccountDataSourceModel maps the data source schema to Go types.
type serviceAccountDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	OrganizationRole types.String `tfsdk:"organization_role"`
	Type             types.String `tfsdk:"type"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (d *serviceAccountDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (d *serviceAccountDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a single Workload Identity Federation service account by its id (`svac_...`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier of the service account (`svac_...`).",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Name of the service account.",
				Computed:    true,
			},
			"organization_role": schema.StringAttribute{
				Description: "Organization role granted to the service account.",
				Computed:    true,
			},
			"type": schema.StringAttribute{
				Description: "Object type, always `service_account`.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "RFC 3339 timestamp of when the service account was created.",
				Computed:    true,
			},
		},
	}
}

func (d *serviceAccountDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config serviceAccountDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := d.client.GetServiceAccount(ctx, config.ID.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError(
				"Service Account Not Found",
				fmt.Sprintf("No service account with id %q was found.", config.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to read service account", err.Error())
		return
	}

	state := serviceAccountDataSourceModel{
		ID:               types.StringValue(sa.ID),
		Name:             types.StringValue(sa.Name),
		OrganizationRole: types.StringValue(sa.OrganizationRole),
		Type:             types.StringValue(sa.Type),
		CreatedAt:        types.StringValue(sa.CreatedAt),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
