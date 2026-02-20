// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ScimDataSource{}

func NewScimDataSource() datasource.DataSource {
	return &ScimDataSource{}
}

// ScimDataSource defines the data source implementation.
type ScimDataSource struct {
	client *netbird.Client
}

// ScimDataSourceModel describes the data source data model.
type ScimDataSourceModel struct {
	Id                types.String `tfsdk:"id"`
	ProviderName      types.String `tfsdk:"provider_name"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	GroupPrefixes     types.List   `tfsdk:"group_prefixes"`
	UserGroupPrefixes types.List   `tfsdk:"user_group_prefixes"`
	LastSyncedAt      types.String `tfsdk:"last_synced_at"`
}

func (d *ScimDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim"
}

func (d *ScimDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read SCIM integration settings",
		MarkdownDescription: "Read SCIM integration settings, see [NetBird Docs](https://docs.netbird.io/manage/team/idp-sync) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "SCIM integration ID",
				Optional:            true,
				Computed:            true,
			},
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "Name of the SCIM identity provider",
				Optional:            true,
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether the integration is enabled",
				Computed:            true,
			},
			"group_prefixes": schema.ListAttribute{
				MarkdownDescription: "List of start_with string patterns for groups to sync",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"user_group_prefixes": schema.ListAttribute{
				MarkdownDescription: "List of start_with string patterns for groups which users to sync",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"last_synced_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of when the integration was last synced",
				Computed:            true,
			},
		},
	}
}

func (d *ScimDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ScimDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ScimDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.ProviderName) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, provider_name)")
		return
	}

	scims, err := d.client.SCIM.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing SCIM integrations", err.Error())
		return
	}

	var scim *api.ScimIntegration
	for _, s := range scims {
		match := matchString(strconv.FormatInt(s.Id, 10), data.Id)
		match += matchString(s.Provider, data.ProviderName)
		if match > 0 {
			if scim != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple SCIM integrations")
			}
			scim = &s
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if scim == nil {
		resp.Diagnostics.AddError("No match", "SCIM integration matching parameters not found")
		return
	}

	data.Id = types.StringValue(strconv.FormatInt(scim.Id, 10))
	data.ProviderName = types.StringValue(scim.Provider)
	data.Enabled = types.BoolValue(scim.Enabled)
	data.LastSyncedAt = types.StringValue(scim.LastSyncedAt.Format("2006-01-02T15:04:05Z"))
	gp, diags := types.ListValueFrom(ctx, types.StringType, scim.GroupPrefixes)
	resp.Diagnostics.Append(diags...)
	data.GroupPrefixes = gp
	ugp, diags := types.ListValueFrom(ctx, types.StringType, scim.UserGroupPrefixes)
	resp.Diagnostics.Append(diags...)
	data.UserGroupPrefixes = ugp
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
