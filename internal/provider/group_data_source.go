// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/management/client/rest"
	"github.com/netbirdio/netbird/management/server/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &GroupDataSource{}

func NewGroupDataSource() datasource.DataSource {
	return &GroupDataSource{}
}

// GroupDataSource defines the data source implementation.
type GroupDataSource struct {
	client *netbird.Client
}

func (d *GroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *GroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Group metadata and associated resources and peers",
		MarkdownDescription: "Read Group metadata and associated resources and peers, see [NetBird Docs](https://docs.netbird.io/how-to/manage-network-access#groups) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Group ID",
				Computed:            true,
				Optional:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Group name identifier",
				Computed:            true,
				Optional:            true,
			},
			"issued": schema.StringAttribute{
				MarkdownDescription: "Group issued by",
				Computed:            true,
			},
			"peers": schema.ListAttribute{
				MarkdownDescription: "List of peers ids",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"resources": schema.ListAttribute{
				MarkdownDescription: "List of network resource ids",
				ElementType:         types.StringType,
				Computed:            true,
			}},
	}
}

func (d *GroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *GroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GroupModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var group *api.Group
	var err error
	if !data.Id.IsUnknown() && !data.Id.IsNull() {
		group, err = d.client.Groups.Get(ctx, data.Id.ValueString())
		if !data.Name.IsNull() && !data.Name.IsUnknown() && data.Name.ValueString() != group.Name {
			group = nil
		}
	} else if !data.Name.IsUnknown() && !data.Name.IsNull() {
		var groups []api.Group
		groups, err = d.client.Groups.List(ctx)
		if err == nil {
			for _, g := range groups {
				if g.Name == data.Name.ValueString() {
					if group != nil {
						resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple groups")
					}
					group = &g
				}
			}

		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Error getting Group", err.Error())
		return
	}

	if group == nil {
		resp.Diagnostics.AddError("No match", "Group matching parameters not found")
	}

	resp.Diagnostics.Append(groupAPIToTerraform(ctx, group, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
