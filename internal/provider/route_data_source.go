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
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &RouteDataSource{}

func NewRouteDataSource() datasource.DataSource {
	return &RouteDataSource{}
}

// RouteDataSource defines the data source implementation.
type RouteDataSource struct {
	client *netbird.Client
}

func (d *RouteDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route"
}

func (d *RouteDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Route metadata and associated resources and peers",
		MarkdownDescription: "Read Route metadata and associated resources and peers, see [NetBird Docs](https://docs.netbird.io/how-to/manage-network-access#routes) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Route ID",
				Optional:            true,
				Computed:            true,
			},
			"network_type": schema.StringAttribute{
				MarkdownDescription: "",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Route description",
				Computed:            true,
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "Route network identifier, to group HA routes",
				Optional:            true,
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Route status",
				Computed:            true,
			},
			"peer": schema.StringAttribute{
				MarkdownDescription: "Peer Identifier associated with route. This property can not be set together with peer_groups",
				Computed:            true,
			},
			"peer_groups": schema.ListAttribute{
				MarkdownDescription: "Peers Group Identifier associated with route. This property can not be set together with peer",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"network": schema.StringAttribute{
				MarkdownDescription: "Network range in CIDR format, Conflicts with domains",
				Computed:            true,
			},
			"domains": schema.ListAttribute{
				MarkdownDescription: "Domain list to be dynamically resolved. Max of 32 domains can be added per route configuration. Conflicts with network",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"metric": schema.Int32Attribute{
				MarkdownDescription: "Route metric number. Lowest number has higher priority",
				Computed:            true,
			},
			"masquerade": schema.BoolAttribute{
				MarkdownDescription: "Indicate if peer should masquerade traffic to this route's prefix",
				Computed:            true,
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Group IDs containing routing peers",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"keep_route": schema.BoolAttribute{
				MarkdownDescription: "Indicate if the route should be kept after a domain doesn't resolve that IP anymore",
				Computed:            true,
			},
			"access_control_groups": schema.ListAttribute{
				MarkdownDescription: "Access control group identifier associated with route.",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *RouteDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RouteDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RouteModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.NetworkId) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, network_id)")
		return
	}

	routes, err := d.client.Routes.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Routes", err.Error())
		return
	}

	var route *api.Route
	for _, r := range routes {
		match := 0
		match += matchString(r.Id, data.Id)
		match += matchString(r.NetworkId, data.NetworkId)
		if match > 0 {
			if route != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple routes")
			}
			route = &r
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if route == nil {
		resp.Diagnostics.AddError("No match", "Route matching parameters not found")
		return
	}

	resp.Diagnostics.Append(routeAPIToTerraform(ctx, route, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
