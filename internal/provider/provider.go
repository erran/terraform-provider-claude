// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gitlab.com/gitlab-org/ai/terraform-provider-claude/internal/client"
)

// envOAuthToken is the environment variable holding the org:admin OAuth bearer
// token. It mirrors the variable the `ant` CLI populates.
const envOAuthToken = "ANTHROPIC_OAUTH_TOKEN"

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
	Endpoint   types.String `tfsdk:"endpoint"`
	OAuthToken types.String `tfsdk:"oauth_token"`
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
			"oauth_token": schema.StringAttribute{
				Description: "OAuth bearer token carrying the `org:admin` scope. May also be set with the " +
					"`ANTHROPIC_OAUTH_TOKEN` environment variable, which is the recommended way to supply it. " +
					"Obtain one with the `ant` CLI:\n\n" +
					"```shell\n" +
					"ant auth login --profile admin --scope \"org:admin\"\n" +
					"export ANTHROPIC_OAUTH_TOKEN=$(ant auth print-credentials --profile admin --access-token)\n" +
					"```\n\n" +
					"Interactive tokens are short-lived; re-run the export command if requests start returning 401. " +
					"Admin API keys (`x-api-key`) are not accepted on these endpoints.",
				Optional:  true,
				Sensitive: true,
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

	if data.OAuthToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("oauth_token"),
			"Unknown Claude OAuth token",
			"The provider cannot create the Claude Admin API client because the oauth_token value is unknown. "+
				"Either set the value statically, or use the "+envOAuthToken+" environment variable.",
		)
		return
	}

	// Resolve the token: explicit configuration wins, otherwise fall back to
	// the ANTHROPIC_OAUTH_TOKEN environment variable.
	token := os.Getenv(envOAuthToken)
	if !data.OAuthToken.IsNull() {
		token = data.OAuthToken.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("oauth_token"),
			"Missing Claude OAuth token",
			"The provider requires an org:admin OAuth bearer token to authenticate against the Claude Admin API.\n\n"+
				"Set the "+envOAuthToken+" environment variable (recommended) or the provider's oauth_token attribute.\n\n"+
				"Obtain a token with the ant CLI:\n"+
				"  ant auth login --profile admin --scope \"org:admin\"\n"+
				"  export "+envOAuthToken+"=$(ant auth print-credentials --profile admin --access-token)",
		)
		return
	}

	endpoint := data.Endpoint.ValueString()

	c := client.New(token, endpoint, p.userAgent(), nil)

	// Make the client available to resources and data sources.
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *ClaudeProvider) userAgent() string {
	return fmt.Sprintf("terraform-provider-claude/%s", p.version)
}

func (p *ClaudeProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewServiceAccountResource,
	}
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
