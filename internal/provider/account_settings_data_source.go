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
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &AccountSettingsDataSource{}

func NewAccountSettingsDataSource() datasource.DataSource {
	return &AccountSettingsDataSource{}
}

// AccountSettingsDataSource defines the data source implementation.
type AccountSettingsDataSource struct {
	client *netbird.Client
}

func (d *AccountSettingsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_settings"
}

func (d *AccountSettingsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Read Account-wide Settings`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of an account",
				Computed:            true,
			},
			"jwt_allow_groups": schema.ListAttribute{
				MarkdownDescription: "List of groups to which users are allowed access",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"jwt_groups_claim_name": schema.StringAttribute{
				MarkdownDescription: "Name of the claim from which we extract groups names to add it to account groups.",
				Computed:            true,
			},
			"peer_login_expiration": schema.Int32Attribute{
				MarkdownDescription: "Period of time after which peer login expires (seconds).",
				Computed:            true,
			},
			"peer_inactivity_expiration": schema.Int32Attribute{
				MarkdownDescription: "Period of time of inactivity after which peer session expires (seconds).",
				Computed:            true,
			},
			"peer_login_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables peer login expiration globally. After peer's login has expired the user has to log in (authenticate). Applies only to peers that were added by a user (interactive SSO login).",
				Computed:            true,
			},
			"peer_inactivity_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables peer inactivity expiration globally. After peer's session has expired the user has to log in (authenticate). Applies only to peers that were added by a user (interactive SSO login).",
				Computed:            true,
			},
			"regular_users_view_blocked": schema.BoolAttribute{
				MarkdownDescription: "Allows blocking regular users from viewing parts of the system.",
				Computed:            true,
			},
			"groups_propagation_enabled": schema.BoolAttribute{
				MarkdownDescription: "Allows propagate the new user auto groups to peers that belongs to the user",
				Computed:            true,
			},
			"jwt_groups_enabled": schema.BoolAttribute{
				MarkdownDescription: "Allows extract groups from JWT claim and add it to account groups.",
				Computed:            true,
			},
			"routing_peer_dns_resolution_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables DNS resolution on the routing peers",
				Computed:            true,
			},
			"peer_approval_enabled": schema.BoolAttribute{
				MarkdownDescription: "(Cloud only) Enables or disables peer approval globally. If enabled, all peers added will be in pending state until approved by an admin.",
				Computed:            true,
			},
			"network_traffic_logs_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables network traffic logging. If enabled, all network traffic events from peers will be stored.",
				Computed:            true,
			},
			"network_traffic_packet_counter_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables network traffic packet counter. If enabled, network packets and their size will be counted and reported. (This can have an slight impact on performance)",
				Computed:            true,
			},
		},
	}
}

func (d *AccountSettingsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AccountSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AccountSettingsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	accounts, err := d.client.Accounts.List(ctx)
	account := accounts[0]

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting AccountSettings", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(accountAPIToTerraform(ctx, &account, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
