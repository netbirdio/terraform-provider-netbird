// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccountSettings{}
var _ resource.ResourceWithImportState = &AccountSettings{}

func NewAccountSettings() resource.Resource {
	return &AccountSettings{}
}

// AccountSettings defines the resource implementation.
type AccountSettings struct {
	client *netbird.Client
}

// AccountSettingsModel describes the resource data model.
type AccountSettingsModel struct {
	Id                                 types.String `tfsdk:"id"`
	JwtAllowGroups                     types.List   `tfsdk:"jwt_allow_groups"`
	JwtGroupsClaimName                 types.String `tfsdk:"jwt_groups_claim_name"`
	PeerLoginExpiration                types.Int32  `tfsdk:"peer_login_expiration"`
	PeerInactivityExpiration           types.Int32  `tfsdk:"peer_inactivity_expiration"`
	PeerLoginExpirationEnabled         types.Bool   `tfsdk:"peer_login_expiration_enabled"`
	PeerInactivityExpirationEnabled    types.Bool   `tfsdk:"peer_inactivity_expiration_enabled"`
	RegularUsersViewBlocked            types.Bool   `tfsdk:"regular_users_view_blocked"`
	GroupsPropagationEnabled           types.Bool   `tfsdk:"groups_propagation_enabled"`
	JwtGroupsEnabled                   types.Bool   `tfsdk:"jwt_groups_enabled"`
	RoutingPeerDnsResolutionEnabled    types.Bool   `tfsdk:"routing_peer_dns_resolution_enabled"`
	PeerApprovalEnabled                types.Bool   `tfsdk:"peer_approval_enabled"`
	NetworkTrafficLogsEnabled          types.Bool   `tfsdk:"network_traffic_logs_enabled"`
	NetworkTrafficPacketCounterEnabled types.Bool   `tfsdk:"network_traffic_packet_counter_enabled"`
}

func (r *AccountSettings) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_settings"
}

func (r *AccountSettings) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Account-wide Settings",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of an account",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"jwt_allow_groups": schema.ListAttribute{
				MarkdownDescription: "List of groups to which users are allowed access",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"jwt_groups_claim_name": schema.StringAttribute{
				MarkdownDescription: "Name of the claim from which we extract groups names to add it to account groups.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"peer_login_expiration": schema.Int32Attribute{
				MarkdownDescription: "Period of time after which peer login expires (seconds).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
				// https://github.com/netbirdio/netbird/blob/main/management/server/account.go#L281
				Validators: []validator.Int32{int32validator.Between(3600, 180*24*3600)},
			},
			"peer_inactivity_expiration": schema.Int32Attribute{
				MarkdownDescription: "Period of time of inactivity after which peer session expires (seconds).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
			},
			"peer_login_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables peer login expiration globally. After peer's login has expired the user has to log in (authenticate). Applies only to peers that were added by a user (interactive SSO login).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"peer_inactivity_expiration_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables peer inactivity expiration globally. After peer's session has expired the user has to log in (authenticate). Applies only to peers that were added by a user (interactive SSO login).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"regular_users_view_blocked": schema.BoolAttribute{
				MarkdownDescription: "Allows blocking regular users from viewing parts of the system.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"groups_propagation_enabled": schema.BoolAttribute{
				MarkdownDescription: "Allows propagate the new user auto groups to peers that belongs to the user",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"jwt_groups_enabled": schema.BoolAttribute{
				MarkdownDescription: "Allows extract groups from JWT claim and add it to account groups.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"routing_peer_dns_resolution_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables DNS resolution on the routing peers",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"peer_approval_enabled": schema.BoolAttribute{
				MarkdownDescription: "(Cloud only) Enables or disables peer approval globally. If enabled, all peers added will be in pending state until approved by an admin.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"network_traffic_logs_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables network traffic logging. If enabled, all network traffic events from peers will be stored.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"network_traffic_packet_counter_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables or disables network traffic packet counter. If enabled, network packets and their size will be counted and reported. (This can have an slight impact on performance)",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *AccountSettings) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func accountAPIToTerraform(ctx context.Context, account *api.Account, data *AccountSettingsModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(account.Id)
	data.JwtAllowGroups, ret = types.ListValueFrom(ctx, types.StringType, account.Settings.JwtAllowGroups)
	data.JwtGroupsClaimName = types.StringPointerValue(account.Settings.JwtGroupsClaimName)
	data.PeerLoginExpiration = types.Int32Value(int32(account.Settings.PeerLoginExpiration))
	data.PeerInactivityExpiration = types.Int32Value(int32(account.Settings.PeerInactivityExpiration))
	data.PeerLoginExpirationEnabled = types.BoolValue(account.Settings.PeerLoginExpirationEnabled)
	data.PeerInactivityExpirationEnabled = types.BoolValue(account.Settings.PeerInactivityExpirationEnabled)
	data.RegularUsersViewBlocked = types.BoolValue(account.Settings.RegularUsersViewBlocked)
	data.GroupsPropagationEnabled = types.BoolPointerValue(account.Settings.GroupsPropagationEnabled)
	data.JwtGroupsEnabled = types.BoolPointerValue(account.Settings.JwtGroupsEnabled)
	data.RoutingPeerDnsResolutionEnabled = types.BoolPointerValue(account.Settings.RoutingPeerDnsResolutionEnabled)
	data.PeerApprovalEnabled = types.BoolValue(account.Settings.Extra.PeerApprovalEnabled)
	data.NetworkTrafficLogsEnabled = types.BoolValue(account.Settings.Extra.NetworkTrafficLogsEnabled)
	data.NetworkTrafficPacketCounterEnabled = types.BoolValue(account.Settings.Extra.NetworkTrafficPacketCounterEnabled)
	return ret
}

func accountTerraformToAPI(ctx context.Context, account *api.Account, data AccountSettingsModel) api.AccountRequest {
	return api.AccountRequest{
		Settings: api.AccountSettings{
			Extra: &api.AccountExtraSettings{
				NetworkTrafficLogsEnabled:          boolDefault(data.NetworkTrafficLogsEnabled, account.Settings.Extra.NetworkTrafficLogsEnabled),
				NetworkTrafficPacketCounterEnabled: boolDefault(data.NetworkTrafficPacketCounterEnabled, account.Settings.Extra.NetworkTrafficPacketCounterEnabled),
				PeerApprovalEnabled:                boolDefault(data.PeerApprovalEnabled, account.Settings.Extra.PeerApprovalEnabled),
			},
			GroupsPropagationEnabled:        boolDefaultPointer(data.GroupsPropagationEnabled, account.Settings.GroupsPropagationEnabled),
			JwtAllowGroups:                  stringListDefaultPointer(ctx, data.JwtAllowGroups, account.Settings.JwtAllowGroups),
			JwtGroupsClaimName:              stringDefaultPointer(data.JwtGroupsClaimName, account.Settings.JwtGroupsClaimName),
			JwtGroupsEnabled:                boolDefaultPointer(data.JwtGroupsEnabled, account.Settings.JwtGroupsEnabled),
			PeerInactivityExpiration:        int(int32Default(data.PeerInactivityExpiration, int32(account.Settings.PeerInactivityExpiration))),
			PeerInactivityExpirationEnabled: boolDefault(data.PeerInactivityExpirationEnabled, account.Settings.PeerInactivityExpirationEnabled),
			PeerLoginExpiration:             int(int32Default(data.PeerLoginExpiration, int32(account.Settings.PeerLoginExpiration))),
			PeerLoginExpirationEnabled:      boolDefault(data.PeerLoginExpirationEnabled, account.Settings.PeerLoginExpirationEnabled),
			RegularUsersViewBlocked:         boolDefault(data.RegularUsersViewBlocked, account.Settings.RegularUsersViewBlocked),
			RoutingPeerDnsResolutionEnabled: boolDefaultPointer(data.RoutingPeerDnsResolutionEnabled, account.Settings.RoutingPeerDnsResolutionEnabled),
		},
	}
}

func (r *AccountSettings) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccountSettingsModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	accounts, err := r.client.Accounts.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error getting AccountSettings", err.Error())
		return
	}

	account := &accounts[0]

	updateRequest := accountTerraformToAPI(ctx, account, data)

	account, err = r.client.Accounts.Update(ctx, account.Id, updateRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error updating AccountSettings", err.Error())
		return
	}

	resp.Diagnostics.Append(accountAPIToTerraform(ctx, account, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountSettings) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccountSettingsModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	accounts, err := r.client.Accounts.List(ctx)
	account := &accounts[0]

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting AccountSettings", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(accountAPIToTerraform(ctx, account, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountSettings) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AccountSettingsModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	accounts, err := r.client.Accounts.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error getting AccountSettings", err.Error())
		return
	}
	account := &accounts[0]

	updateRequest := accountTerraformToAPI(ctx, account, data)

	account, err = r.client.Accounts.Update(ctx, data.Id.ValueString(), updateRequest)

	if err != nil {
		resp.Diagnostics.AddError("Error updating AccountSettings", err.Error())
		return
	}

	resp.Diagnostics.Append(accountAPIToTerraform(ctx, account, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccountSettings) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing
}

func (r *AccountSettings) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
