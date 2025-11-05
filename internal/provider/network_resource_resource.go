// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NetworkResource{}
var _ resource.ResourceWithImportState = &NetworkResource{}

func NewNetworkResource() resource.Resource {
	return &NetworkResource{}
}

// NetworkResource defines the resource implementation.
type NetworkResource struct {
	client *netbird.Client
}

// NetworkResourceModel describes the resource data model.
type NetworkResourceModel struct {
	Id          types.String `tfsdk:"id"`
	NetworkId   types.String `tfsdk:"network_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Address     types.String `tfsdk:"address"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Groups      types.Set    `tfsdk:"groups"`
}

func (r *NetworkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_resource"
}

func (r *NetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage Network Resources",
		MarkdownDescription: "Create and manage Network Resources, see [NetBird Docs](https://docs.netbird.io/how-to/networks#resources) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a resource",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a network",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "NetworkResource Name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "NetworkResource Description",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Default:             stringdefault.StaticString(""),
			},
			"address": schema.StringAttribute{
				MarkdownDescription: "Network resource address (either a direct host like 1.1.1.1 or 1.1.1.1/32, or a subnet like 192.168.178.0/24, or domains like example.com and *.example.com)",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "NetworkResource status",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"groups": schema.SetAttribute{
				MarkdownDescription: "Group IDs containing the resource",
				Required:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{setvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)), setvalidator.SizeAtLeast(1)},
			},
		},
	}
}

func (r *NetworkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func networkResourceAPIToTerraform(ctx context.Context, networkResource *api.NetworkResource, data *NetworkResourceModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(networkResource.Id)
	data.Name = types.StringValue(networkResource.Name)
	data.Description = types.StringPointerValue(networkResource.Description)
	data.Address = types.StringValue(networkResource.Address)
	data.Enabled = types.BoolValue(networkResource.Enabled)
	groups := make([]string, len(networkResource.Groups))
	for i, k := range networkResource.Groups {
		groups[i] = k.Id
	}
	data.Groups, d = types.SetValueFrom(ctx, types.StringType, groups)
	ret.Append(d...)
	return ret
}

func (r *NetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NetworkResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkResourceReq := api.NetworkResourceRequest{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
		Address:     data.Address.ValueString(),
		Enabled:     data.Enabled.ValueBool(),
		Groups:      stringSetDefault(ctx, data.Groups, []string{}),
	}

	networkResource, err := r.client.Networks.Resources(data.NetworkId.ValueString()).Create(ctx, networkResourceReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating networkResource", err.Error())
		return
	}

	resp.Diagnostics.Append(networkResourceAPIToTerraform(ctx, networkResource, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NetworkResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkResource, err := r.client.Networks.Resources(data.NetworkId.ValueString()).Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting NetworkResource", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(networkResourceAPIToTerraform(ctx, networkResource, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NetworkResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	networkResourceReq := api.NetworkResourceRequest{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueStringPointer(),
		Address:     data.Address.ValueString(),
		Enabled:     data.Enabled.ValueBool(),
		Groups:      stringSetDefault(ctx, data.Groups, []string{}),
	}

	networkResource, err := r.client.Networks.Resources(data.NetworkId.ValueString()).Update(ctx, data.Id.ValueString(), networkResourceReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NetworkResource", err.Error())
		return
	}

	resp.Diagnostics.Append(networkResourceAPIToTerraform(ctx, networkResource, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NetworkResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Networks.Resources(data.NetworkId.ValueString()).Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting NetworkResource", err.Error())
	}
}

func (r *NetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	splitID := strings.Split(req.ID, "/")
	if len(splitID) != 2 {
		resp.Diagnostics.AddError("Error importing NetworkResource", "Invalid import ID, must be in format `networkID/networkResourceID`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), splitID[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), splitID[1])...)
}
