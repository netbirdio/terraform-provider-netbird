// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/management/client/rest"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PeerDataSource{}

func NewPeerDataSource() datasource.DataSource {
	return &PeerDataSource{}
}

// PeerDataSource defines the data source implementation.
type PeerDataSource struct {
	client *netbird.Client
}

func (d *PeerDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_peer"
}

func (d *PeerDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read Peer information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Peer ID",
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Peer Name",
				Optional:            true,
				Computed:            true,
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "Peer  IP",
				Computed:            true,
			},
			"connection_ip": schema.StringAttribute{
				MarkdownDescription: "Peer Public IP",
				Computed:            true,
			},
			"connected": schema.BoolAttribute{
				MarkdownDescription: "Peer Connection Status",
				Computed:            true,
			},
			"last_seen": schema.StringAttribute{
				MarkdownDescription: "Peer Last Seen timedate",
				Computed:            true,
			},
			"os": schema.StringAttribute{
				MarkdownDescription: "Peer OS",
				Computed:            true,
			},
			"kernel_version": schema.StringAttribute{
				MarkdownDescription: "Peer Kernel Version",
				Computed:            true,
			},
			"geoname_id": schema.Int32Attribute{
				MarkdownDescription: "Peer Location ID",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Peer  Version",
				Computed:            true,
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Peer groups",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"ssh_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable SSH to Peer",
				Optional:            true,
			},
			"inactivity_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable inactivity expiration for peer",
				Optional:            true,
			},
			"approval_required": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether peer needs approval",
				Optional:            true,
			},
			"dns_label": schema.StringAttribute{
				MarkdownDescription: "Peer DNS Label",
				Computed:            true,
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User ID of peer",
				Computed:            true,
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "Peer's HOSTNAME",
				Computed:            true,
			},
			"ui_version": schema.StringAttribute{
				MarkdownDescription: "Peer  UI Version",
				Computed:            true,
			},
			"login_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether login expiration is enabled for peer",
				Optional:            true,
			},
			"login_expired": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether peer login is expired",
				Computed:            true,
			},
			"last_login": schema.StringAttribute{
				MarkdownDescription: "Time of peer last login",
				Computed:            true,
			},
			"country_code": schema.StringAttribute{
				MarkdownDescription: "Peer country code",
				Computed:            true,
			},
			"city_name": schema.StringAttribute{
				MarkdownDescription: "Peer city name",
				Computed:            true,
			},
			"serial_number": schema.StringAttribute{
				MarkdownDescription: "Peer device serial number",
				Computed:            true,
			},
			"extra_dns_labels": schema.ListAttribute{
				MarkdownDescription: "Peer extra DNS Labels",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *PeerDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PeerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PeerModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	peer, err := d.client.Peers.Get(ctx, data.Id.ValueString())

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			data.Id = types.StringNull()
		} else {
			resp.Diagnostics.AddError("Error getting Peer", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(peerAPIToTerraform(ctx, peer, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
