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

var (
	_ datasource.DataSource              = &organizationRateLimitsDataSource{}
	_ datasource.DataSourceWithConfigure = &organizationRateLimitsDataSource{}
)

// NewOrganizationRateLimitsDataSource is the constructor registered with the
// provider.
func NewOrganizationRateLimitsDataSource() datasource.DataSource {
	return &organizationRateLimitsDataSource{}
}

type organizationRateLimitsDataSource struct{ client *client.Client }

// organizationRateLimitsDataSourceModel is the top-level state model.
type organizationRateLimitsDataSourceModel struct {
	Model      types.String          `tfsdk:"model"`
	GroupType  types.String          `tfsdk:"group_type"`
	RateLimits []rateLimitGroupModel `tfsdk:"rate_limits"`
}

// rateLimitGroupModel maps a single rate limit group to Terraform state.
type rateLimitGroupModel struct {
	Type      types.String          `tfsdk:"type"`
	GroupType types.String          `tfsdk:"group_type"`
	Models    []types.String        `tfsdk:"models"`
	Limits    []rateLimitValueModel `tfsdk:"limits"`
}

// rateLimitValueModel maps a single organization limiter to Terraform state.
type rateLimitValueModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.Int64  `tfsdk:"value"`
}

func (d *organizationRateLimitsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_rate_limits"
}

func (d *organizationRateLimitsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the rate limits configured at the organization level for the Messages API and its " +
			"supporting resources. This is the same information shown on the Limits page in the Claude Console. " +
			"The Rate Limits API is read-only; limits cannot be changed through Terraform.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "When set, returns only the group containing this model ID or alias (e.g. " +
					"`claude-opus-4-8`). The lookup fails if no group matches the model string.",
				Optional: true,
			},
			"group_type": schema.StringAttribute{
				Description: "When set, restricts the response to a single category. One of `model_group`, " +
					"`batch`, `token_count`, `files`, `skills`, or `web_search`.",
				Optional: true,
			},
			"rate_limits": schema.ListNestedAttribute{
				Description: "The rate limit groups returned by the API.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Description: "Object type, always `rate_limit`.",
							Computed:    true,
						},
						"group_type": schema.StringAttribute{
							Description: "Category of limits covered by the group: `model_group`, `batch`, " +
								"`token_count`, `files`, `skills`, or `web_search`.",
							Computed: true,
						},
						"models": schema.ListAttribute{
							Description: "For `model_group` entries, every model ID and alias that counts " +
								"against the group's limits. Null for all other group types.",
							Computed:    true,
							ElementType: types.StringType,
						},
						"limits": schema.ListNestedAttribute{
							Description: "The configured limiters for the group.",
							Computed:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										Description: "The limiter, such as `requests_per_minute`, " +
											"`input_tokens_per_minute`, `output_tokens_per_minute`, or " +
											"`enqueued_batch_requests`.",
										Computed: true,
									},
									"value": schema.Int64Attribute{
										Description: "The configured limit.",
										Computed:    true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *organizationRateLimitsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *organizationRateLimitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config organizationRateLimitsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rateLimits, err := d.client.ListOrganizationRateLimits(ctx, config.Model.ValueString(), config.GroupType.ValueString())
	if err != nil {
		if client.NotFound(err) {
			resp.Diagnostics.AddError(
				"No rate limit group matches the requested model",
				fmt.Sprintf("The model %q does not fall under any organization rate limit group.", config.Model.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError("Unable to read organization rate limits", err.Error())
		return
	}

	groups := make([]rateLimitGroupModel, len(rateLimits))
	for i, rl := range rateLimits {
		groups[i] = rateLimitGroupModel{
			Type:      types.StringValue(rl.Type),
			GroupType: types.StringValue(rl.GroupType),
			Models:    stringSliceToList(rl.Models),
			Limits:    organizationLimitValues(rl.Limits),
		}
	}

	state := organizationRateLimitsDataSourceModel{
		Model:      config.Model,
		GroupType:  config.GroupType,
		RateLimits: groups,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// stringSliceToList maps an API string slice to Terraform values, preserving a
// nil slice as a null list (so non-model groups report null `models`).
func stringSliceToList(values []string) []types.String {
	if values == nil {
		return nil
	}
	out := make([]types.String, len(values))
	for i, v := range values {
		out[i] = types.StringValue(v)
	}
	return out
}

// organizationLimitValues maps API limiters to organization state values.
func organizationLimitValues(limits []client.RateLimitValue) []rateLimitValueModel {
	out := make([]rateLimitValueModel, len(limits))
	for i, l := range limits {
		out[i] = rateLimitValueModel{
			Type:  types.StringValue(l.Type),
			Value: types.Int64Value(l.Value),
		}
	}
	return out
}
