// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PeersDataSource{}

func NewPeersDataSource() datasource.DataSource {
	return &PeersDataSource{}
}

// PeersModel describes the resource data model.
type PeersModel struct {
	Ids                         types.List   `tfsdk:"ids"`
	Name                        types.String `tfsdk:"name"`
	Ip                          types.String `tfsdk:"ip"`
	ConnectionIp                types.String `tfsdk:"connection_ip"`
	Connected                   types.Bool   `tfsdk:"connected"`
	LastSeen                    types.String `tfsdk:"last_seen"`
	Os                          types.String `tfsdk:"os"`
	KernelVersion               types.String `tfsdk:"kernel_version"`
	GeonameId                   types.Int32  `tfsdk:"geoname_id"`
	Version                     types.String `tfsdk:"version"`
	Groups                      types.List   `tfsdk:"groups"`
	SshEnabled                  types.Bool   `tfsdk:"ssh_enabled"`
	InactivityExpirationEnabled types.Bool   `tfsdk:"inactivity_expiration_enabled"`
	ApprovalRequired            types.Bool   `tfsdk:"approval_required"`
	DnsLabel                    types.String `tfsdk:"dns_label"`
	UserId                      types.String `tfsdk:"user_id"`
	Hostname                    types.String `tfsdk:"hostname"`
	UiVersion                   types.String `tfsdk:"ui_version"`
	LoginExpirationEnabled      types.Bool   `tfsdk:"login_expiration_enabled"`
	LoginExpired                types.Bool   `tfsdk:"login_expired"`
	LastLogin                   types.String `tfsdk:"last_login"`
	CountryCode                 types.String `tfsdk:"country_code"`
	CityName                    types.String `tfsdk:"city_name"`
	SerialNumber                types.String `tfsdk:"serial_number"`
	ExtraDnsLabels              types.List   `tfsdk:"extra_dns_labels"`
}

// PeersDataSource defines the data source implementation.
type PeersDataSource struct {
	client *netbird.Client
}

func (d *PeersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_peers"
}

func (d *PeersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read Peer information.",
		Attributes: map[string]schema.Attribute{
			"ids": schema.ListAttribute{
				Computed:            true,
				MarkdownDescription: "Peers IDs",
				ElementType:         types.StringType,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Peer Name",
				Optional:            true,
				Computed:            true,
			},
			"ip": schema.StringAttribute{
				MarkdownDescription: "Peer  IP",
				Optional:            true,
				Computed:            true,
			},
			"connection_ip": schema.StringAttribute{
				MarkdownDescription: "Peer Public IP",
				Optional:            true,
				Computed:            true,
			},
			"connected": schema.BoolAttribute{
				MarkdownDescription: "Peer Connection Status",
				Optional:            true,
				Computed:            true,
			},
			"last_seen": schema.StringAttribute{
				MarkdownDescription: "Peer Last Seen timedate",
				Computed:            true,
			},
			"os": schema.StringAttribute{
				MarkdownDescription: "Peer OS",
				Optional:            true,
				Computed:            true,
			},
			"kernel_version": schema.StringAttribute{
				MarkdownDescription: "Peer Kernel Version",
				Computed:            true,
			},
			"geoname_id": schema.Int32Attribute{
				MarkdownDescription: "Peer Location ID",
				Optional:            true,
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Peer Version",
				Computed:            true,
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Peer groups",
				Optional:            true,
				ElementType:         types.StringType,
				Computed:            true,
			},
			"ssh_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable SSH to Peer",
				Optional:            true,
				Computed:            true,
			},
			"inactivity_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable inactivity expiration for peer",
				Optional:            true,
				Computed:            true,
			},
			"approval_required": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether peer needs approval",
				Optional:            true,
				Computed:            true,
			},
			"dns_label": schema.StringAttribute{
				MarkdownDescription: "Peer DNS Label",
				Optional:            true,
				Computed:            true,
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User ID of peer",
				Optional:            true,
				Computed:            true,
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "Peer's HOSTNAME",
				Optional:            true,
				Computed:            true,
			},
			"ui_version": schema.StringAttribute{
				MarkdownDescription: "Peer  UI Version",
				Computed:            true,
			},
			"login_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether login expiration is enabled for peer",
				Optional:            true,
				Computed:            true,
			},
			"login_expired": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether peer login is expired",
				Optional:            true,
				Computed:            true,
			},
			"last_login": schema.StringAttribute{
				MarkdownDescription: "Time of peer last login",
				Computed:            true,
			},
			"country_code": schema.StringAttribute{
				MarkdownDescription: "Peer country code",
				Optional:            true,
				Computed:            true,
			},
			"city_name": schema.StringAttribute{
				MarkdownDescription: "Peer city name",
				Optional:            true,
				Computed:            true,
			},
			"serial_number": schema.StringAttribute{
				MarkdownDescription: "Peer device serial number",
				Computed:            true,
			},
			"extra_dns_labels": schema.ListAttribute{
				MarkdownDescription: "Peer extra DNS Labels",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *PeersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func filterPeers(ctx context.Context, peers []api.Peer, data PeersModel) ([]string, diag.Diagnostics) {
	var d diag.Diagnostics
	var filteredPeers []string
	for _, p := range peers {
		match := 0
		match += matchString(p.Name, data.Name)
		match += matchString(p.Ip, data.Ip)
		match += matchString(p.ConnectionIp, data.ConnectionIp)
		match += matchString(p.DnsLabel, data.DnsLabel)
		match += matchString(p.UserId, data.UserId)
		match += matchString(p.Hostname, data.Hostname)
		match += matchString(p.CountryCode, data.CountryCode)
		match += matchString(p.CityName, data.CityName)
		match += matchString(p.Os, data.Os)
		match += matchBool(p.Connected, data.Connected)
		match += matchBool(p.SshEnabled, data.SshEnabled)
		match += matchBool(p.InactivityExpirationEnabled, data.InactivityExpirationEnabled)
		match += matchBool(p.ApprovalRequired, data.ApprovalRequired)
		match += matchBool(p.LoginExpirationEnabled, data.LoginExpirationEnabled)
		match += matchBool(p.LoginExpired, data.LoginExpired)
		match += matchInt32(int32(p.GeonameId), data.GeonameId)
		m, di := matchListString(ctx, p.ExtraDnsLabels, data.ExtraDnsLabels)
		d.Append(di...)
		if d.HasError() {
			return filteredPeers, d
		}
		match += m
		groups := make([]string, len(p.Groups))
		for i, j := range p.Groups {
			groups[i] = j.Id
		}
		m, di = matchListString(ctx, groups, data.Groups)
		d.Append(di...)
		if d.HasError() {
			return filteredPeers, d
		}
		match += m

		if match > 0 {
			filteredPeers = append(filteredPeers, p.Id)
		}
	}

	return filteredPeers, d
}

func (d *PeersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PeersModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(
		data.Name,
		data.Ip,
		data.ConnectionIp,
		data.DnsLabel,
		data.UserId,
		data.Hostname,
		data.CountryCode,
		data.CityName,
		data.Os,
		data.Connected,
		data.SshEnabled,
		data.InactivityExpirationEnabled,
		data.ApprovalRequired,
		data.LoginExpirationEnabled,
		data.LoginExpired,
		data.GeonameId,
		data.Groups,
	) == 0 {
		resp.Diagnostics.AddError(
			"No selector",
			`Must add at least one of (name, ip, connection_ip, dns_label, user_id, hostname, country_code, city_name, os,`+
				` connected, ssh_enabled, inactivity_expiration_enabled, approval_required, login_expiration_enabled,`+
				` login_expired, geoname_id, groups)`,
		)
		return
	}

	var err error
	var peers []api.Peer
	peers, err = d.client.Peers.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Peers", err.Error())
		return
	}

	filteredPeers, di := filterPeers(ctx, peers, data)
	resp.Diagnostics.Append(di...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Ids, di = types.ListValueFrom(ctx, types.StringType, filteredPeers)
	resp.Diagnostics.Append(di...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
