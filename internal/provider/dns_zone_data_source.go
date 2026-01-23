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
var _ datasource.DataSource = &DNSZoneDataSource{}

func NewDNSZoneDataSource() datasource.DataSource {
	return &DNSZoneDataSource{}
}

// DNSZoneDataSource defines the data source implementation.
type DNSZoneDataSource struct {
	client *netbird.Client
}

func (d *DNSZoneDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (d *DNSZoneDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read DNS Zone metadata.",
		MarkdownDescription: "Read DNS Zone metadata, see [NetBird Docs](https://docs.netbird.io/manage/dns/custom-zones) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "DNS Zone ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "DNS Zone name identifier",
				Optional:            true,
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Zone domain (FQDN)",
				Optional:            true,
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "DNS Zone status",
				Computed:            true,
			},
			"enable_search_domain": schema.BoolAttribute{
				MarkdownDescription: "Enable this zone as a search domain",
				Computed:            true,
			},
			"distribution_groups": schema.SetAttribute{
				MarkdownDescription: "Group IDs that define groups of peers that will resolve this zone",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (d *DNSZoneDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DNSZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DNSZoneModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var zone *api.Zone
	zones, err := d.client.DNSZones.ListZones(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS Zones", err.Error())
		return
	}

	for _, z := range zones {
		match := 0
		match += matchString(z.Id, data.Id)
		match += matchString(z.Name, data.Name)
		match += matchString(z.Domain, data.Domain)
		if match > 0 {
			if zone != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple DNS zones")
			}
			zone = &z
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if zone == nil {
		resp.Diagnostics.AddError("Not Found", "DNS Zone not found")
		return
	}

	resp.Diagnostics.Append(dnsZoneAPIToTerraform(ctx, zone, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
