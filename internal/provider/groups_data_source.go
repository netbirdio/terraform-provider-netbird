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
var _ datasource.DataSource = &GroupsDataSource{}

// NewGroupsDataSource returns the netbird_groups data source.
func NewGroupsDataSource() datasource.DataSource {
	return &GroupsDataSource{}
}

// GroupsModel describes the data source data model.
type GroupsModel struct {
	Name   types.String `tfsdk:"name"`
	Issued types.String `tfsdk:"issued"`
	Ids    types.List   `tfsdk:"ids"`
	Groups []GroupModel `tfsdk:"groups"`
}

// GroupsDataSource defines the data source implementation.
type GroupsDataSource struct {
	client *netbird.Client
}

// Metadata sets the data source type name.
func (d *GroupsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_groups"
}

// Schema defines the netbird_groups filters and computed attributes.
func (d *GroupsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List Groups matching all the given filters",
		MarkdownDescription: "List Groups matching all the given filters (at least one). Unlike `netbird_group`, matching " +
			"nothing is not an error: an OIDC group that no member has logged in with yet simply is not returned. " +
			"See [NetBird Docs](https://docs.netbird.io/how-to/manage-network-access#groups) for more information.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Only return the group with this name",
				Optional:            true,
			},
			"issued": schema.StringAttribute{
				MarkdownDescription: "Only return groups issued by this source (`api`, `integration` or `jwt`)",
				Optional:            true,
			},
			"ids": schema.ListAttribute{
				MarkdownDescription: "IDs of the matching groups",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"groups": schema.ListNestedAttribute{
				MarkdownDescription: "Matching groups",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Group ID",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Group name identifier",
							Computed:            true,
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
						},
					},
				},
			},
		},
	}
}

// Configure stores the NetBird API client provided by the provider.
func (d *GroupsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// filterGroups keeps the groups matching every configured filter.
func filterGroups(groups []api.Group, data GroupsModel) []api.Group {
	filtered := make([]api.Group, 0, len(groups))
	for _, g := range groups {
		issued := ""
		if g.Issued != nil {
			issued = string(*g.Issued)
		}
		if matchString(g.Name, data.Name) < 0 || matchString(issued, data.Issued) < 0 {
			continue
		}
		filtered = append(filtered, g)
	}
	return filtered
}

// Read lists all groups, applies the configured filters, and sets the state.
func (d *GroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data GroupsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Name, data.Issued) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (name, issued)")
		return
	}

	groups, err := d.client.Groups.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Groups", err.Error())
		return
	}

	resp.Diagnostics.Append(groupsAPIToTerraform(ctx, filterGroups(groups, data), &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// groupsAPIToTerraform converts API groups into the Terraform groups list.
func groupsAPIToTerraform(ctx context.Context, groups []api.Group, data *GroupsModel) diag.Diagnostics {
	var ret diag.Diagnostics
	ids := make([]string, len(groups))
	data.Groups = make([]GroupModel, len(groups))
	for i := range groups {
		ids[i] = groups[i].Id
		ret.Append(groupAPIToTerraform(ctx, &groups[i], &data.Groups[i])...)
	}
	l, diag := types.ListValueFrom(ctx, types.StringType, ids)
	ret.Append(diag...)
	data.Ids = l
	return ret
}
