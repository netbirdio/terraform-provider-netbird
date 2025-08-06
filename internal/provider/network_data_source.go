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
var _ datasource.DataSource = &NetworkDataSource{}

func NewNetworkDataSource() datasource.DataSource {
	return &NetworkDataSource{}
}

// NetworkDataSource defines the data source implementation.
type NetworkDataSource struct {
	client *netbird.Client
}

func (d *NetworkDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (d *NetworkDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Network Settings and Metadata",
		MarkdownDescription: "Read Network Settings and Metadata, see [NetBird Docs](https://docs.netbird.io/how-to/networks) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Network ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Network Name",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Network Description",
				Computed:            true,
			},
			"routers": schema.ListAttribute{
				MarkdownDescription: "Network Router IDs",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"resources": schema.ListAttribute{
				MarkdownDescription: "Network Resource IDs",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"policies": schema.ListAttribute{
				MarkdownDescription: "Policy IDs associated with resources inside this Network",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *NetworkDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NetworkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NetworkModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
		return
	}

	networks, err := d.client.Networks.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Networks", err.Error())
		return
	}

	var network *api.Network
	for _, n := range networks {
		match := 0
		match += matchString(n.Id, data.Id)
		match += matchString(n.Name, data.Name)
		if match > 0 {
			if network != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple networks")
			}
			network = &n
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if network == nil {
		resp.Diagnostics.AddError("No match", "Network matching parameters not found")
		return
	}

	resp.Diagnostics.Append(networkAPIToTerraform(ctx, network, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
