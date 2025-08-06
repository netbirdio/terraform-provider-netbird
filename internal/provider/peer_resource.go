// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Peer{}
var _ resource.ResourceWithImportState = &Peer{}

func NewPeer() resource.Resource {
	return &Peer{}
}

// Peer defines the resource implementation.
type Peer struct {
	client *netbird.Client
}

// PeerModel describes the resource data model.
type PeerModel struct {
	Id                          types.String `tfsdk:"id"`
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

func (r *Peer) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_peer"
}

func (r *Peer) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manage NetBird Peers",
		MarkdownDescription: "NetBird peer is a machine that runs the NetBird agent and is connected to the network. This resource doesn't create the Peer itself but manages it.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Peer ID",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Peer Name",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
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
				Computed:            true,
			},
			"ssh_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable SSH to Peer",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"inactivity_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable inactivity expiration for peer",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"approval_required": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether peer needs approval",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
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
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
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

func (r *Peer) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func peerAPIToTerraform(ctx context.Context, peer *api.Peer, data *PeerModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(peer.Id)
	data.Name = types.StringValue(peer.Name)
	data.Ip = types.StringValue(peer.Ip)
	data.ConnectionIp = types.StringValue(peer.ConnectionIp)
	data.Connected = types.BoolValue(peer.Connected)
	data.LastSeen = types.StringValue(peer.LastSeen.Format(time.RFC3339))
	data.Os = types.StringValue(peer.Os)
	data.KernelVersion = types.StringValue(peer.KernelVersion)
	data.GeonameId = types.Int32Value(int32(peer.GeonameId))
	data.Version = types.StringValue(peer.Version)
	data.SshEnabled = types.BoolValue(peer.SshEnabled)
	data.InactivityExpirationEnabled = types.BoolValue(peer.InactivityExpirationEnabled)
	data.ApprovalRequired = types.BoolValue(peer.ApprovalRequired)
	data.DnsLabel = types.StringValue(peer.DnsLabel)
	data.UserId = types.StringValue(peer.UserId)
	data.Hostname = types.StringValue(peer.Hostname)
	data.UiVersion = types.StringValue(peer.UiVersion)
	data.LoginExpirationEnabled = types.BoolValue(peer.LoginExpirationEnabled)
	data.LoginExpired = types.BoolValue(peer.LoginExpired)
	data.LastLogin = types.StringValue(peer.LastLogin.Format(time.RFC3339))
	data.CountryCode = types.StringValue(peer.CountryCode)
	data.CityName = types.StringValue(peer.CityName)
	data.SerialNumber = types.StringValue(peer.SerialNumber)
	groupIDs := make([]string, len(peer.Groups))
	for i, g := range peer.Groups {
		groupIDs[i] = g.Id
	}
	l, diag := types.ListValueFrom(ctx, types.StringType, groupIDs)
	ret.Append(diag...)
	data.Groups = l
	l, diag = types.ListValueFrom(ctx, types.StringType, peer.ExtraDnsLabels)
	ret.Append(diag...)
	data.ExtraDnsLabels = l
	return ret
}

func (r *Peer) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PeerModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	peer, err := r.client.Peers.Get(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error getting Peer", err.Error())
		return
	}

	updateRequest := api.PeerRequest{
		ApprovalRequired:            &peer.ApprovalRequired,
		InactivityExpirationEnabled: peer.InactivityExpirationEnabled,
		LoginExpirationEnabled:      peer.LoginExpirationEnabled,
		Name:                        peer.Name,
		SshEnabled:                  peer.SshEnabled,
	}
	updateRequired := false
	if !data.ApprovalRequired.IsUnknown() && data.ApprovalRequired.ValueBool() != peer.ApprovalRequired {
		updateRequest.ApprovalRequired = data.ApprovalRequired.ValueBoolPointer()
		updateRequired = true
	}
	if !data.InactivityExpirationEnabled.IsUnknown() && data.InactivityExpirationEnabled.ValueBool() != peer.InactivityExpirationEnabled {
		updateRequest.InactivityExpirationEnabled = data.InactivityExpirationEnabled.ValueBool()
		updateRequired = true
	}
	if !data.LoginExpirationEnabled.IsUnknown() && data.LoginExpirationEnabled.ValueBool() != peer.LoginExpirationEnabled {
		updateRequest.LoginExpirationEnabled = data.LoginExpirationEnabled.ValueBool()
		updateRequired = true
	}
	if !data.Name.IsUnknown() && data.Name.ValueString() != peer.Name {
		updateRequest.Name = data.Name.ValueString()
		updateRequired = true
	}
	if !data.SshEnabled.IsUnknown() && data.SshEnabled.ValueBool() != peer.SshEnabled {
		updateRequest.SshEnabled = data.SshEnabled.ValueBool()
		updateRequired = true
	}

	if updateRequired {
		peer, err = r.client.Peers.Update(ctx, peer.Id, updateRequest)
		if err != nil {
			resp.Diagnostics.AddError("Error updating Peer", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(peerAPIToTerraform(ctx, peer, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Peer) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PeerModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	peer, err := r.client.Peers.Get(ctx, data.Id.ValueString())

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting Peer", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(peerAPIToTerraform(ctx, peer, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Peer) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PeerModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	peer, err := r.client.Peers.Update(ctx, data.Id.ValueString(), api.PeerRequest{
		ApprovalRequired:            data.ApprovalRequired.ValueBoolPointer(),
		InactivityExpirationEnabled: data.InactivityExpirationEnabled.ValueBool(),
		LoginExpirationEnabled:      data.LoginExpirationEnabled.ValueBool(),
		Name:                        data.Name.ValueString(),
		SshEnabled:                  data.SshEnabled.ValueBool(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Error updating Peer", err.Error())
		return
	}

	resp.Diagnostics.Append(peerAPIToTerraform(ctx, peer, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update peer, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Peer) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PeerModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do not delete actual peers in acceptance tests to make running locally easier
	if _, ok := os.LookupEnv("TF_ACC"); !ok {
		err := r.client.Peers.Delete(ctx, data.Id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error deleting Peer", err.Error())
		}
	}
}

func (r *Peer) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
