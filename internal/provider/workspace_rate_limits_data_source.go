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
	_ datasource.DataSource              = &workspaceRateLimitsDataSource{}
	_ datasource.DataSourceWithConfigure = &workspaceRateLimitsDataSource{}
)

// NewWorkspaceRateLimitsDataSource is the constructor registered with the
// provider.
func NewWorkspaceRateLimitsDataSource() datasource.DataSource {
	return &workspaceRateLimitsDataSource{}
}

type workspaceRateLimitsDataSource struct{ client *client.Client }

// workspaceRateLimitsDataSourceModel is the top-level state model.
type workspaceRateLimitsDataSourceModel struct {
	WorkspaceID types.String                   `tfsdk:"workspace_id"`
	GroupType   types.String                   `tfsdk:"group_type"`
	RateLimits  []workspaceRateLimitGroupModel `tfsdk:"rate_limits"`
}

// workspaceRateLimitGroupModel maps a single workspace override group to state.
type workspaceRateLimitGroupModel struct {
	Type      types.String                   `tfsdk:"type"`
	GroupType types.String                   `tfsdk:"group_type"`
	Models    []types.String                 `tfsdk:"models"`
	Limits    []workspaceRateLimitValueModel `tfsdk:"limits"`
}

// workspaceRateLimitValueModel maps a single workspace limiter override to
// state, including the organization-level value it overrides.
type workspaceRateLimitValueModel struct {
	Type     types.String `tfsdk:"type"`
	Value    types.Int64  `tfsdk:"value"`
	OrgLimit types.Int64  `tfsdk:"org_limit"`
}

func (d *workspaceRateLimitsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_rate_limits"
}

func (d *workspaceRateLimitsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the rate limit overrides configured for a single workspace. The response contains " +
			"only overrides: any group or limiter absent from it is inherited from the organization (query " +
			"`claude_organization_rate_limits` for inherited values). The default workspace cannot have " +
			"overrides and has no entry here. The Rate Limits API is read-only; limits cannot be changed " +
			"through Terraform.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{
				Description: "Identifier of the workspace (`wrkspc_...`) whose overrides to read.",
				Required:    true,
			},
			"group_type": schema.StringAttribute{
				Description: "When set, restricts the response to a single category. One of `model_group`, " +
					"`batch`, `token_count`, `files`, `skills`, or `web_search`.",
				Optional: true,
			},
			"rate_limits": schema.ListNestedAttribute{
				Description: "The workspace override groups returned by the API.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Description: "Object type, always `workspace_rate_limit`.",
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
							Description: "The limiter overrides for the group.",
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
										Description: "The workspace override value for the limiter.",
										Computed:    true,
									},
									"org_limit": schema.Int64Attribute{
										Description: "The organization-level value for the same limiter, or " +
											"null if the organization has no configured limit for it.",
										Computed: true,
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

func (d *workspaceRateLimitsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workspaceRateLimitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config workspaceRateLimitsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rateLimits, err := d.client.ListWorkspaceRateLimits(ctx, config.WorkspaceID.ValueString(), config.GroupType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read workspace rate limits", err.Error())
		return
	}

	groups := make([]workspaceRateLimitGroupModel, len(rateLimits))
	for i, rl := range rateLimits {
		limits := make([]workspaceRateLimitValueModel, len(rl.Limits))
		for j, l := range rl.Limits {
			limits[j] = workspaceRateLimitValueModel{
				Type:     types.StringValue(l.Type),
				Value:    types.Int64Value(l.Value),
				OrgLimit: optionalInt64(l.OrgLimit),
			}
		}
		groups[i] = workspaceRateLimitGroupModel{
			Type:      types.StringValue(rl.Type),
			GroupType: types.StringValue(rl.GroupType),
			Models:    stringSliceToList(rl.Models),
			Limits:    limits,
		}
	}

	state := workspaceRateLimitsDataSourceModel{
		WorkspaceID: config.WorkspaceID,
		GroupType:   config.GroupType,
		RateLimits:  groups,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
