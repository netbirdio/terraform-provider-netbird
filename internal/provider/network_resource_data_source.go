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
var _ datasource.DataSource = &NetworkResourceDataSource{}

func NewNetworkResourceDataSource() datasource.DataSource {
	return &NetworkResourceDataSource{}
}

// NetworkResourceDataSource defines the data source implementation.
type NetworkResourceDataSource struct {
	client *netbird.Client
}

func (d *NetworkResourceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_resource"
}

func (d *NetworkResourceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Network Resource settings and metadata",
		MarkdownDescription: "Read Network Resource settings and metadata, see [NetBird Docs](https://docs.netbird.io/how-to/networks#resources) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a resource",
				Optional:            true,
				Computed:            true,
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a network",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "NetworkResource Name",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "NetworkResource Description",
				Computed:            true,
			},
			"address": schema.StringAttribute{
				MarkdownDescription: "Network resource address (either a direct host like 1.1.1.1 or 1.1.1.1/32, or a subnet like 192.168.178.0/24, or domains like example.com and *.example.com)",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "NetworkResource status",
				Computed:            true,
			},
			"groups": schema.SetAttribute{
				MarkdownDescription: "Group IDs containing the resource",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *NetworkResourceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NetworkResourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NetworkResourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
		return
	}

	networks, err := d.client.Networks.Resources(data.NetworkId.ValueString()).List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Network Resources", err.Error())
		return
	}

	var networkResource *api.NetworkResource
	for _, n := range networks {
		match := 0
		match += matchString(n.Id, data.Id)
		match += matchString(n.Name, data.Name)
		if match > 0 {
			if networkResource != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple network resources")
			}
			networkResource = &n
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if networkResource == nil {
		resp.Diagnostics.AddError("No match", "Network Resource matching parameters not found")
		return
	}

	resp.Diagnostics.Append(networkResourceAPIToTerraform(ctx, networkResource, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
