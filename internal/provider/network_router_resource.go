// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NetworkRouter{}
var _ resource.ResourceWithImportState = &NetworkRouter{}

func NewNetworkRouter() resource.Resource {
	return &NetworkRouter{}
}

// NetworkRouter defines the resource implementation.
type NetworkRouter struct {
	client *netbird.Client
}

// NetworkRouterModel describes the resource data model.
type NetworkRouterModel struct {
	Id         types.String `tfsdk:"id"`
	NetworkId  types.String `tfsdk:"network_id"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	PeerGroups types.List   `tfsdk:"peer_groups"`
	Peer       types.String `tfsdk:"peer"`
	Metric     types.Int32  `tfsdk:"metric"`
	Masquerade types.Bool   `tfsdk:"masquerade"`
}

func (r *NetworkRouter) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_router"
}

func (r *NetworkRouter) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage Network Routers",
		MarkdownDescription: "Create and manage Network Routers, see [NetBird Docs](https://docs.netbird.io/how-to/networks#routing-peers) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a router",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a network",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Network router status",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"masquerade": schema.BoolAttribute{
				MarkdownDescription: "Indicate if peer should masquerade traffic to this route's prefix",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"peer": schema.StringAttribute{
				MarkdownDescription: "Peer Identifier associated with route. This property can not be set together with peer_groups",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("peer_groups"))},
			},
			"metric": schema.Int32Attribute{
				MarkdownDescription: "Route metric number. Lowest number has higher priority",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(9999),
				PlanModifiers:       []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
				Validators:          []validator.Int32{int32validator.Between(1, 9999)},
			},
			"peer_groups": schema.ListAttribute{
				MarkdownDescription: "Peers Group Identifier associated with route. This property can not be set together with peer",
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("peer"))},
			},
		},
	}
}

func (r *NetworkRouter) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func networkRouterAPIToTerraform(ctx context.Context, networkRouter *api.NetworkRouter, data *NetworkRouterModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(networkRouter.Id)
	if networkRouter.Peer != nil && *networkRouter.Peer == "" {
		data.Peer = types.StringNull()
	} else {
		data.Peer = types.StringPointerValue(networkRouter.Peer)
	}
	data.Enabled = types.BoolValue(networkRouter.Enabled)
	data.Masquerade = types.BoolValue(networkRouter.Masquerade)
	data.Metric = types.Int32Value(int32(networkRouter.Metric))
	data.PeerGroups, d = types.ListValueFrom(ctx, types.StringType, networkRouter.PeerGroups)
	ret.Append(d...)
	return ret
}

func (r *NetworkRouter) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NetworkRouterModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkRouterReq := api.NetworkRouterRequest{
		Enabled:    data.Enabled.ValueBool(),
		Masquerade: data.Masquerade.ValueBool(),
		Metric:     int(data.Metric.ValueInt32()),
		Peer:       data.Peer.ValueStringPointer(),
		PeerGroups: stringListDefaultPointer(ctx, data.PeerGroups, nil),
	}

	networkRouter, err := r.client.Networks.Routers(data.NetworkId.ValueString()).Create(ctx, networkRouterReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating networkRouter", err.Error())
		return
	}

	resp.Diagnostics.Append(networkRouterAPIToTerraform(ctx, networkRouter, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkRouter) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NetworkRouterModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkRouter, err := r.client.Networks.Routers(data.NetworkId.ValueString()).Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting NetworkRouter", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(networkRouterAPIToTerraform(ctx, networkRouter, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkRouter) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NetworkRouterModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	networkRouterReq := api.NetworkRouterRequest{
		Enabled:    data.Enabled.ValueBool(),
		Masquerade: data.Masquerade.ValueBool(),
		Metric:     int(data.Metric.ValueInt32()),
		Peer:       data.Peer.ValueStringPointer(),
		PeerGroups: stringListDefaultPointer(ctx, data.PeerGroups, nil),
	}

	networkRouter, err := r.client.Networks.Routers(data.NetworkId.ValueString()).Update(ctx, data.Id.ValueString(), networkRouterReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NetworkRouter", err.Error())
		return
	}

	resp.Diagnostics.Append(networkRouterAPIToTerraform(ctx, networkRouter, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkRouter) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NetworkRouterModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Networks.Routers(data.NetworkId.ValueString()).Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting NetworkRouter", err.Error())
	}
}

func (r *NetworkRouter) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	splitID := strings.Split(req.ID, "/")
	if len(splitID) != 2 {
		resp.Diagnostics.AddError("Error importing NetworkRouter", "Invalid import ID, must be in format `networkID/networkRouterID`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), splitID[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), splitID[1])...)
}
