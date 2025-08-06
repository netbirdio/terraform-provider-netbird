// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Group{}
var _ resource.ResourceWithImportState = &Group{}

func NewGroup() resource.Resource {
	return &Group{}
}

// Group defines the resource implementation.
type Group struct {
	client *netbird.Client
}

// GroupModel describes the resource data model.
type GroupModel struct {
	Id        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Peers     types.List   `tfsdk:"peers"`
	Resources types.List   `tfsdk:"resources"`
	Issued    types.String `tfsdk:"issued"`
}

func (r *Group) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *Group) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and assign Groups",
		MarkdownDescription: "Create and assign Groups, see [NetBird Docs](https://docs.netbird.io/how-to/manage-network-access#groups) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Group ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Group name identifier",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"issued": schema.StringAttribute{
				MarkdownDescription: "Group issued by",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"peers": schema.ListAttribute{
				MarkdownDescription: "List of peers ids",
				ElementType:         types.StringType,
				Computed:            true,
				Optional:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
			"resources": schema.ListAttribute{
				MarkdownDescription: "List of network resource ids",
				ElementType:         types.StringType,
				Computed:            true,
				Optional:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
		},
	}
}

func (r *Group) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func groupAPIToTerraform(ctx context.Context, group *api.Group, data *GroupModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(group.Id)
	data.Name = types.StringValue(group.Name)
	data.Issued = types.StringPointerValue((*string)(group.Issued))
	peers := make([]string, len(group.Peers))
	for i, v := range group.Peers {
		peers[i] = v.Id
	}
	l, diag := types.ListValueFrom(ctx, types.StringType, peers)
	ret.Append(diag...)
	data.Peers = l
	resources := make([]string, len(group.Resources))
	for i, j := range group.Resources {
		resources[i] = j.Id
	}
	l, diag = types.ListValueFrom(ctx, types.StringType, resources)
	ret.Append(diag...)
	data.Resources = l
	return ret
}

func (r *Group) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var resources *[]api.Resource
	if len(data.Resources.Elements()) > 0 {
		var tfVal []map[string]string
		var resourcesVal []api.Resource
		resp.Diagnostics.Append(data.Resources.ElementsAs(ctx, &tfVal, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, v := range tfVal {
			resourcesVal = append(resourcesVal, api.Resource{
				Id:   v["id"],
				Type: api.ResourceType(v["type"]),
			})
		}
		resources = &resourcesVal
	}

	groupReq := api.GroupRequest{
		Name:      data.Name.ValueString(),
		Peers:     stringListDefaultPointer(ctx, data.Peers, nil),
		Resources: resources,
	}

	group, err := r.client.Groups.Create(ctx, groupReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating group", err.Error())
		return
	}

	resp.Diagnostics.Append(groupAPIToTerraform(ctx, group, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Group) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GroupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.Groups.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting Group", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(groupAPIToTerraform(ctx, group, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Group) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	var resources *[]api.Resource
	if len(data.Resources.Elements()) > 0 {
		var tfVal []map[string]string
		var resourcesVal []api.Resource
		resp.Diagnostics.Append(data.Resources.ElementsAs(ctx, &tfVal, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, v := range tfVal {
			resourcesVal = append(resourcesVal, api.Resource{
				Id:   v["id"],
				Type: api.ResourceType(v["type"]),
			})
		}
		resources = &resourcesVal
	}

	groupReq := api.GroupRequest{
		Name:      data.Name.ValueString(),
		Peers:     stringListDefaultPointer(ctx, data.Peers, nil),
		Resources: resources,
	}

	group, err := r.client.Groups.Update(ctx, data.Id.ValueString(), groupReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Group", err.Error())
		return
	}

	resp.Diagnostics.Append(groupAPIToTerraform(ctx, group, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Group) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GroupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Groups.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Group", err.Error())
	}
}

func (r *Group) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
