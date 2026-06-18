// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ClaudeProvider satisfies various provider interfaces.
var _ provider.Provider = &ClaudeProvider{}

// ClaudeProvider defines the provider implementation.
type ClaudeProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" in acceptance tests.
	version string
}

// ClaudeProviderModel describes the provider data model.
type ClaudeProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
}

func (p *ClaudeProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "claude"
	resp.Version = p.version
}

func (p *ClaudeProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Claude Admin API resources, such as Workload Identity Federation service accounts.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "Base URL of the Claude Admin API. Defaults to `https://api.anthropic.com`.",
				Optional:    true,
			},
		},
	}
}

func (p *ClaudeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ClaudeProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Authentication and client construction are added in a later change.
}

func (p *ClaudeProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *ClaudeProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ClaudeProvider{
			version: version,
		}
	}
}
