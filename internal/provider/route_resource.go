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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Route{}
var _ resource.ResourceWithImportState = &Route{}

func NewRoute() resource.Resource {
	return &Route{}
}

// Route defines the resource implementation.
type Route struct {
	client *netbird.Client
}

// RouteModel describes the resource data model.
type RouteModel struct {
	Id                  types.String `tfsdk:"id"`
	NetworkType         types.String `tfsdk:"network_type"`
	Description         types.String `tfsdk:"description"`
	NetworkId           types.String `tfsdk:"network_id"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	Peer                types.String `tfsdk:"peer"`
	PeerGroups          types.List   `tfsdk:"peer_groups"`
	Network             types.String `tfsdk:"network"`
	Domains             types.List   `tfsdk:"domains"`
	Metric              types.Int32  `tfsdk:"metric"`
	Masquerade          types.Bool   `tfsdk:"masquerade"`
	Groups              types.List   `tfsdk:"groups"`
	KeepRoute           types.Bool   `tfsdk:"keep_route"`
	AccessControlGroups types.List   `tfsdk:"access_control_groups"`
}

func (r *Route) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route"
}

func (r *Route) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage Routes",
		MarkdownDescription: "Create and manage Routes, see [NetBird Docs](https://docs.netbird.io/how-to/routing-traffic-to-private-networks#network-routes) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Route ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_type": schema.StringAttribute{
				MarkdownDescription: "Domain or IPv4",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Route description",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "Route network identifier, to group HA routes",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 40)},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Route status",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"peer": schema.StringAttribute{
				MarkdownDescription: "Peer Identifier associated with route. This property can not be set together with peer_groups",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("peer_groups"))},
			},
			"peer_groups": schema.ListAttribute{
				MarkdownDescription: "Peers Group Identifier associated with route. This property can not be set together with peer",
				ElementType:         types.StringType,
				Optional:            true,
				Validators:          []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("peer"))},
			},
			"network": schema.StringAttribute{
				MarkdownDescription: "Network range in CIDR format, Conflicts with domains",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("domains"))},
			},
			"domains": schema.ListAttribute{
				MarkdownDescription: "Domain list to be dynamically resolved. Max of 32 domains can be added per route configuration. Conflicts with network",
				ElementType:         types.StringType,
				Optional:            true,
				Validators:          []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("network")), listvalidator.SizeAtMost(32)},
			},
			"metric": schema.Int32Attribute{
				MarkdownDescription: "Route metric number. Lowest number has higher priority",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(9999),
				Validators:          []validator.Int32{int32validator.Between(1, 9999)},
			},
			"masquerade": schema.BoolAttribute{
				MarkdownDescription: "Indicate if peer should masquerade traffic to this route's prefix",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Group IDs containing routing peers",
				ElementType:         types.StringType,
				Required:            true,
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"keep_route": schema.BoolAttribute{
				MarkdownDescription: "Indicate if the route should be kept after a domain doesn't resolve that IP anymore",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"access_control_groups": schema.ListAttribute{
				MarkdownDescription: "Access control group identifier associated with route.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (r *Route) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func routeAPIToTerraform(ctx context.Context, route *api.Route, data *RouteModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(route.Id)
	data.NetworkType = types.StringValue(route.NetworkType)
	data.Description = types.StringValue(route.Description)
	data.NetworkId = types.StringValue(route.NetworkId)
	data.Enabled = types.BoolValue(route.Enabled)
	if route.Peer != nil && *route.Peer != "" {
		data.Peer = types.StringPointerValue(route.Peer)
	} else {
		data.Peer = types.StringNull()
	}
	if route.Domains != nil && len(*route.Domains) > 0 {
		data.Network = types.StringNull()
	} else {
		data.Network = types.StringPointerValue(route.Network)
	}
	data.Metric = types.Int32Value(int32(route.Metric))
	data.Masquerade = types.BoolValue(route.Masquerade)
	data.KeepRoute = types.BoolValue(route.KeepRoute)
	if route.PeerGroups == nil {
		data.PeerGroups = types.ListNull(types.StringType)
	} else {
		data.PeerGroups, d = types.ListValueFrom(ctx, types.StringType, route.PeerGroups)
		ret.Append(d...)
	}
	if route.Domains == nil {
		data.Domains = types.ListNull(types.StringType)
	} else {
		data.Domains, d = types.ListValueFrom(ctx, types.StringType, route.Domains)
		ret.Append(d...)
	}
	data.Groups, d = types.ListValueFrom(ctx, types.StringType, route.Groups)
	ret.Append(d...)
	if route.AccessControlGroups == nil {
		data.AccessControlGroups = types.ListNull(types.StringType)
	} else {
		data.AccessControlGroups, d = types.ListValueFrom(ctx, types.StringType, route.AccessControlGroups)
		ret.Append(d...)
	}
	return ret
}

func (r *Route) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RouteModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	routeReq := api.RouteRequest{
		AccessControlGroups: stringListDefaultPointer(ctx, data.AccessControlGroups, nil),
		Description:         data.Description.ValueString(),
		Domains:             stringListDefaultPointer(ctx, data.Domains, nil),
		Enabled:             data.Enabled.ValueBool(),
		Groups:              stringListDefault(ctx, data.Groups, []string{}),
		KeepRoute:           data.KeepRoute.ValueBool(),
		Masquerade:          data.Masquerade.ValueBool(),
		Metric:              int(data.Metric.ValueInt32()),
		Network:             data.Network.ValueStringPointer(),
		NetworkId:           data.NetworkId.ValueString(),
		Peer:                data.Peer.ValueStringPointer(),
		PeerGroups:          stringListDefaultPointer(ctx, data.PeerGroups, nil),
	}

	route, err := r.client.Routes.Create(ctx, routeReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating route", err.Error())
		return
	}

	resp.Diagnostics.Append(routeAPIToTerraform(ctx, route, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Route) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RouteModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	route, err := r.client.Routes.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting Route", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(routeAPIToTerraform(ctx, route, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Route) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RouteModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	routeReq := api.RouteRequest{
		AccessControlGroups: stringListDefaultPointer(ctx, data.AccessControlGroups, nil),
		Description:         data.Description.ValueString(),
		Domains:             stringListDefaultPointer(ctx, data.Domains, nil),
		Enabled:             data.Enabled.ValueBool(),
		Groups:              stringListDefault(ctx, data.Groups, []string{}),
		KeepRoute:           data.KeepRoute.ValueBool(),
		Masquerade:          data.Masquerade.ValueBool(),
		Metric:              int(data.Metric.ValueInt32()),
		Network:             data.Network.ValueStringPointer(),
		NetworkId:           data.NetworkId.ValueString(),
		Peer:                data.Peer.ValueStringPointer(),
		PeerGroups:          stringListDefaultPointer(ctx, data.PeerGroups, nil),
	}

	route, err := r.client.Routes.Update(ctx, data.Id.ValueString(), routeReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Route", err.Error())
		return
	}

	resp.Diagnostics.Append(routeAPIToTerraform(ctx, route, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Route) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RouteModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Routes.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Route", err.Error())
	}
}

func (r *Route) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
