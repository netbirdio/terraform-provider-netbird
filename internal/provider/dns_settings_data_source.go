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
var _ datasource.DataSource = &DNSSettingsDataSource{}

func NewDNSSettingsDataSource() datasource.DataSource {
	return &DNSSettingsDataSource{}
}

// DNSSettingsDataSource defines the data source implementation.
type DNSSettingsDataSource struct {
	client *netbird.Client
}

func (d *DNSSettingsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_settings"
}

func (d *DNSSettingsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read DNS Management Settings",
		Attributes: map[string]schema.Attribute{
			"disabled_management_groups": schema.ListAttribute{
				MarkdownDescription: "Groups whose DNS management is disabled",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *DNSSettingsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DNSSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DNSSettingsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dnsSettings, err := d.client.DNS.GetSettings(ctx)

	if err != nil {
		resp.Diagnostics.AddError("Error getting DNSSettings", err.Error())
		return
	}

	resp.Diagnostics.Append(dnsSettingsAPIToTerraform(ctx, dnsSettings, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
