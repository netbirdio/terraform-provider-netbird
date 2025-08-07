// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &NetworkRouterDataSource{}

func NewNetworkRouterDataSource() datasource.DataSource {
	return &NetworkRouterDataSource{}
}

// NetworkRouterDataSource defines the data source implementation.
type NetworkRouterDataSource struct {
	client *netbird.Client
}

func (d *NetworkRouterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_router"
}

func (d *NetworkRouterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Network Router settings and metadata",
		MarkdownDescription: "Read Network Router settings and metadata, see [NetBird Docs](https://docs.netbird.io/how-to/networks#routing-peers) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a router",
				Required:            true,
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a network",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Network router status",
				Computed:            true,
			},
			"masquerade": schema.BoolAttribute{
				MarkdownDescription: "Indicate if peer should masquerade traffic to this route's prefix",
				Computed:            true,
			},
			"peer": schema.StringAttribute{
				MarkdownDescription: "Peer Identifier associated with route. This property can not be set together with peer_groups",
				Computed:            true,
			},
			"metric": schema.Int32Attribute{
				MarkdownDescription: "Route metric number. Lowest number has higher priority",
				Computed:            true,
			},
			"peer_groups": schema.ListAttribute{
				MarkdownDescription: "Peers Group Identifier associated with route. This property can not be set together with peer",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *NetworkRouterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

func (d *NetworkRouterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NetworkRouterModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkRouter, err := d.client.Networks.Routers(data.NetworkId.ValueString()).Get(ctx, data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Error getting NetworkRouter", err.Error())
		return
	}

	resp.Diagnostics.Append(networkRouterAPIToTerraform(ctx, networkRouter, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
