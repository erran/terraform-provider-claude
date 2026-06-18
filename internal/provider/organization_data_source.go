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
	_ datasource.DataSource              = &organizationDataSource{}
	_ datasource.DataSourceWithConfigure = &organizationDataSource{}
)

// NewOrganizationDataSource is the constructor registered with the provider.
func NewOrganizationDataSource() datasource.DataSource { return &organizationDataSource{} }

type organizationDataSource struct{ client *client.Client }

type organizationDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Type types.String `tfsdk:"type"`
	Name types.String `tfsdk:"name"`
}

func (d *organizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *organizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Information about the current Claude organization.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Description: "Organization UUID.", Computed: true},
			"type": schema.StringAttribute{Description: "Object type, always `organization`.", Computed: true},
			"name": schema.StringAttribute{Description: "Organization name.", Computed: true},
		},
	}
}

func (d *organizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = c
}

func (d *organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	org, err := d.client.GetOrganization(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read organization", err.Error())
		return
	}
	state := organizationDataSourceModel{
		ID:   types.StringValue(org.ID),
		Type: types.StringValue(org.Type),
		Name: types.StringValue(org.Name),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
