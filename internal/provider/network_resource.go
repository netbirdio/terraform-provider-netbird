// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Network{}
var _ resource.ResourceWithImportState = &Network{}

func NewNetwork() resource.Resource {
	return &Network{}
}

// Network defines the resource implementation.
type Network struct {
	client *netbird.Client
}

// NetworkModel describes the resource data model.
type NetworkModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Resources   types.List   `tfsdk:"resources"`
	Routers     types.List   `tfsdk:"routers"`
	Policies    types.List   `tfsdk:"policies"`
}

func (r *Network) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *Network) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage Networks",
		MarkdownDescription: "Create and manage Networks, see [NetBird Docs](https://docs.netbird.io/how-to/networks) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Network ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Network Name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Network Description",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"routers": schema.ListAttribute{
				MarkdownDescription: "Network Router IDs",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"resources": schema.ListAttribute{
				MarkdownDescription: "Network Resource IDs",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"policies": schema.ListAttribute{
				MarkdownDescription: "Policy IDs associated with resources inside this Network",
				ElementType:         types.StringType,
				Computed:            true,
			},
		},
	}
}

func (r *Network) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func networkAPIToTerraform(ctx context.Context, network *api.Network, data *NetworkModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(network.Id)
	data.Name = types.StringValue(network.Name)
	data.Description = types.StringPointerValue(network.Description)
	data.Resources, d = types.ListValueFrom(ctx, types.StringType, network.Resources)
	ret.Append(d...)
	data.Routers, d = types.ListValueFrom(ctx, types.StringType, network.Routers)
	ret.Append(d...)
	data.Policies, d = types.ListValueFrom(ctx, types.StringType, network.Policies)
	ret.Append(d...)
	return ret
}

func (r *Network) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NetworkModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkReq := api.NetworkRequest{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
	}

	network, err := r.client.Networks.Create(ctx, networkReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network", err.Error())
		return
	}

	resp.Diagnostics.Append(networkAPIToTerraform(ctx, network, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Network) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NetworkModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	network, err := r.client.Networks.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting Network", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(networkAPIToTerraform(ctx, network, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Network) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NetworkModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	networkReq := api.NetworkRequest{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
	}

	network, err := r.client.Networks.Update(ctx, data.Id.ValueString(), networkReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Network", err.Error())
		return
	}

	resp.Diagnostics.Append(networkAPIToTerraform(ctx, network, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Network) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NetworkModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Networks.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Network", err.Error())
	}
}

func (r *Network) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
