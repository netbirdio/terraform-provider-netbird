// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &DNSRecordDataSource{}

func NewDNSRecordDataSource() datasource.DataSource {
	return &DNSRecordDataSource{}
}

// DNSRecordDataSource defines the data source implementation.
type DNSRecordDataSource struct {
	client *netbird.Client
}

func (d *DNSRecordDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (d *DNSRecordDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read DNS Record metadata.",
		MarkdownDescription: "Read DNS Record metadata, see [NetBird Docs](https://docs.netbird.io/manage/dns/custom-zones) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "DNS Record ID",
				Optional:            true,
				Computed:            true,
			},
			"zone_id": schema.StringAttribute{
				MarkdownDescription: "DNS Zone ID that this record belongs to",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "DNS record name as a full FQDN",
				Optional:            true,
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "DNS record type (A, AAAA, or CNAME)",
				Optional:            true,
				Computed:            true,
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "DNS record content (IP address for A/AAAA, domain for CNAME)",
				Computed:            true,
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "Time to live in seconds",
				Computed:            true,
			},
		},
	}
}

func (d *DNSRecordDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DNSRecordDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DNSRecordModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var record *api.DNSRecord
	records, err := d.client.DNSZones.ListRecords(ctx, data.ZoneId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS Records", err.Error())
		return
	}

	for _, r := range records {
		match := 0
		match += matchString(r.Id, data.Id)
		match += matchString(r.Name, data.Name)
		match += matchString(string(r.Type), data.Type)
		if match > 0 {
			if record != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple DNS records")
			}
			record = &r
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if record == nil {
		resp.Diagnostics.AddError("Not Found", "DNS Record not found")
		return
	}

	resp.Diagnostics.Append(dnsRecordAPIToTerraform(ctx, record, data.ZoneId.ValueString(), &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
