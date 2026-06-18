// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/erran/terraform-provider-claude/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Environment variables the provider reads, mirroring the names the Anthropic
// SDKs and `ant` CLI use.
const (
	envOAuthToken        = "ANTHROPIC_OAUTH_TOKEN"
	envIdentityToken     = "ANTHROPIC_IDENTITY_TOKEN"
	envIdentityTokenFile = "ANTHROPIC_IDENTITY_TOKEN_FILE"
	envFederationRuleID  = "ANTHROPIC_FEDERATION_RULE_ID"
	envOrganizationID    = "ANTHROPIC_ORGANIZATION_ID"
	envServiceAccountID  = "ANTHROPIC_SERVICE_ACCOUNT_ID"
	envWorkspaceID       = "ANTHROPIC_WORKSPACE_ID"
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
	Endpoint   types.String `tfsdk:"endpoint"`
	OAuthToken types.String `tfsdk:"oauth_token"`

	// Workload Identity Federation: exchange an OIDC token for a short-lived
	// org:admin bearer token.
	IdentityToken     types.String `tfsdk:"identity_token"`
	IdentityTokenFile types.String `tfsdk:"identity_token_file"`
	FederationRuleID  types.String `tfsdk:"federation_rule_id"`
	OrganizationID    types.String `tfsdk:"organization_id"`
	ServiceAccountID  types.String `tfsdk:"service_account_id"`
	WorkspaceID       types.String `tfsdk:"workspace_id"`
}

func (p *ClaudeProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "claude"
	resp.Version = p.version
}

func (p *ClaudeProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Claude Admin API resources, such as Workload Identity Federation service accounts.\n\n" +
			"Authenticate either with a static `org:admin` OAuth token (`oauth_token`), or, for CI and " +
			"automation, with Workload Identity Federation: the provider exchanges an OIDC identity token " +
			"(such as a GitLab CI `id_token`) for a short-lived bearer token. The granted scope is " +
			"determined by the federation rule, which must have `oauth_scope: org:admin`.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "Base URL of the Claude Admin API. Defaults to `https://api.anthropic.com`.",
				Optional:    true,
			},
			"oauth_token": schema.StringAttribute{
				Description: "OAuth bearer token carrying the `org:admin` scope. May also be set with the " +
					"`ANTHROPIC_OAUTH_TOKEN` environment variable. Obtain one interactively with the `ant` CLI:\n\n" +
					"```shell\n" +
					"ant auth login --profile admin --scope \"org:admin\"\n" +
					"export ANTHROPIC_OAUTH_TOKEN=$(ant auth print-credentials --profile admin --access-token)\n" +
					"```\n\n" +
					"Takes precedence over Workload Identity Federation when both are configured. " +
					"Admin API keys (`x-api-key`) are not accepted on these endpoints.",
				Optional:  true,
				Sensitive: true,
			},
			"identity_token": schema.StringAttribute{
				Description: "OIDC identity token (JWT) to exchange for a bearer token via Workload Identity " +
					"Federation. In GitLab CI, populate this from an `id_tokens` entry, e.g. the " +
					"`ANTHROPIC_IDENTITY_TOKEN` environment variable. Mutually exclusive with " +
					"`identity_token_file`.",
				Optional:  true,
				Sensitive: true,
			},
			"identity_token_file": schema.StringAttribute{
				Description: "Path to a file containing the OIDC identity token (JWT). May also be set with the " +
					"`ANTHROPIC_IDENTITY_TOKEN_FILE` environment variable.",
				Optional: true,
			},
			"federation_rule_id": schema.StringAttribute{
				Description: "Federation rule (`fdrl_...`) to evaluate the identity token against. Its " +
					"`oauth_scope` must be `org:admin`. May also be set with `ANTHROPIC_FEDERATION_RULE_ID`.",
				Optional: true,
			},
			"organization_id": schema.StringAttribute{
				Description: "Anthropic organization UUID for the token exchange. May also be set with " +
					"`ANTHROPIC_ORGANIZATION_ID`.",
				Optional: true,
			},
			"service_account_id": schema.StringAttribute{
				Description: "Target service account (`svac_...`) for the token exchange; must have the admin " +
					"organization role. May also be set with `ANTHROPIC_SERVICE_ACCOUNT_ID`.",
				Optional: true,
			},
			"workspace_id": schema.StringAttribute{
				Description: "Workspace (`wrkspc_...`) for the token exchange. Required only when the federation " +
					"rule covers more than one workspace. May also be set with `ANTHROPIC_WORKSPACE_ID`.",
				Optional: true,
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

	for _, attr := range []struct {
		name  string
		value types.String
	}{
		{"oauth_token", data.OAuthToken},
		{"identity_token", data.IdentityToken},
	} {
		if attr.value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root(attr.name),
				"Unknown provider configuration value",
				fmt.Sprintf("The %s value is unknown at configuration time. Set it statically or via its environment variable.", attr.name),
			)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := data.Endpoint.ValueString()

	token := p.resolveBearerToken(ctx, data, endpoint, resp)
	if resp.Diagnostics.HasError() || token == "" {
		return
	}

	c := client.New(token, endpoint, p.userAgent(), nil)

	// Make the client available to resources and data sources.
	resp.DataSourceData = c
	resp.ResourceData = c
}

// resolveBearerToken returns an org:admin bearer token, preferring an explicit
// static token and otherwise performing a Workload Identity Federation token
// exchange. It appends diagnostics and returns "" on failure.
func (p *ClaudeProvider) resolveBearerToken(ctx context.Context, data ClaudeProviderModel, endpoint string, resp *provider.ConfigureResponse) string {
	// 1. Static OAuth token (interactive / `ant` CLI) takes precedence.
	if token := stringOrEnv(data.OAuthToken, envOAuthToken); token != "" {
		return token
	}

	// 2. Workload Identity Federation: exchange an OIDC token.
	exchange := client.TokenExchangeRequest{
		FederationRuleID: stringOrEnv(data.FederationRuleID, envFederationRuleID),
		OrganizationID:   stringOrEnv(data.OrganizationID, envOrganizationID),
		ServiceAccountID: stringOrEnv(data.ServiceAccountID, envServiceAccountID),
		WorkspaceID:      stringOrEnv(data.WorkspaceID, envWorkspaceID),
	}
	exchange.Assertion = p.resolveIdentityToken(data, resp)
	if resp.Diagnostics.HasError() {
		return ""
	}

	if !exchange.Valid() {
		resp.Diagnostics.AddError(
			"Missing Claude credentials",
			"The provider requires an org:admin credential to authenticate against the Claude Admin API. "+
				"Provide one of:\n\n"+
				"  - A static OAuth token via the oauth_token attribute or "+envOAuthToken+" "+
				"(obtain it with `ant auth login --profile admin --scope \"org:admin\"`), or\n"+
				"  - Workload Identity Federation: an identity token (identity_token / identity_token_file / "+
				envIdentityToken+" / "+envIdentityTokenFile+") together with federation_rule_id, "+
				"organization_id, and service_account_id (or their ANTHROPIC_* environment variables). "+
				"The federation rule must have oauth_scope: org:admin.",
		)
		return ""
	}

	tflog.Debug(ctx, "exchanging identity token for an org:admin bearer token", map[string]any{
		"federation_rule_id": exchange.FederationRuleID,
		"service_account_id": exchange.ServiceAccountID,
	})

	token, err := client.ExchangeToken(ctx, nil, endpoint, p.userAgent(), exchange)
	if err != nil {
		resp.Diagnostics.AddError("Workload Identity Federation token exchange failed", err.Error())
		return ""
	}
	return token
}

// resolveIdentityToken reads the OIDC identity token from the inline attribute,
// the file attribute, or their environment variables, in that order.
func (p *ClaudeProvider) resolveIdentityToken(data ClaudeProviderModel, resp *provider.ConfigureResponse) string {
	if token := stringOrEnv(data.IdentityToken, envIdentityToken); token != "" {
		return token
	}

	if file := stringOrEnv(data.IdentityTokenFile, envIdentityTokenFile); file != "" {
		contents, err := os.ReadFile(file)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("identity_token_file"),
				"Unable to read identity token file",
				fmt.Sprintf("Reading %q: %s", file, err),
			)
			return ""
		}
		return strings.TrimSpace(string(contents))
	}

	return ""
}

// stringOrEnv returns the attribute value if set, otherwise the environment
// variable named by env.
func stringOrEnv(v types.String, env string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	return os.Getenv(env)
}

func (p *ClaudeProvider) userAgent() string {
	return fmt.Sprintf("terraform-provider-claude/%s", p.version)
}

func (p *ClaudeProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewServiceAccountResource,
		NewFederationIssuerResource,
		NewFederationRuleResource,
		NewWorkspaceResource,
		NewWorkspaceMemberResource,
		NewOrganizationInviteResource,
		NewOrganizationMemberResource,
		NewAPIKeyResource,
	}
}

func (p *ClaudeProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewWorkspaceDataSource,
		NewWorkspacesDataSource,
		NewServiceAccountDataSource,
		NewFederationIssuerDataSource,
		NewFederationRuleDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ClaudeProvider{
			version: version,
		}
	}
}
