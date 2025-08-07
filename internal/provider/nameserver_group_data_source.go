// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &NameserverGroupDataSource{}

func NewNameserverGroupDataSource() datasource.DataSource {
	return &NameserverGroupDataSource{}
}

// NameserverGroupDataSource defines the data source implementation.
type NameserverGroupDataSource struct {
	client *netbird.Client
}

func (d *NameserverGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nameserver_group"
}

func (d *NameserverGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read Nameserver Group settings",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "NameserverGroup ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of nameserver group",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the nameserver group",
				Computed:            true,
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Distribution group IDs that defines group of peers that will use this nameserver group",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"domains": schema.ListAttribute{
				MarkdownDescription: "Match domain list. It should be empty only if primary is true.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"nameservers": schema.ListNestedAttribute{
				MarkdownDescription: "Nameserver list",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip": schema.StringAttribute{
							MarkdownDescription: "Nameserver IP",
							Computed:            true,
						},
						"ns_type": schema.StringAttribute{
							MarkdownDescription: "Nameserver Type",
							Computed:            true,
						},
						"port": schema.Int32Attribute{
							MarkdownDescription: "Nameserver Port",
							Computed:            true,
						},
					},
				},
				Computed: true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Nameserver group status",
				Computed:            true,
			},
			"primary": schema.BoolAttribute{
				MarkdownDescription: "Defines if a nameserver group is primary that resolves all domains. It should be true only if domains list is empty.",
				Computed:            true,
			},
			"search_domains_enabled": schema.BoolAttribute{
				MarkdownDescription: "Search domain status for match domains. It should be true only if domains list is not empty.",
				Computed:            true,
			},
		},
	}
}

func (d *NameserverGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NameserverGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NameserverGroupModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
		return
	}

	nsGroups, err := d.client.DNS.ListNameserverGroups(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing NameserverGroups", err.Error())
		return
	}

	var nameserverGroup *api.NameserverGroup
	for _, nsg := range nsGroups {
		match := 0
		match += matchString(nsg.Id, data.Id)
		match += matchString(nsg.Name, data.Name)
		if match > 0 {
			if nameserverGroup != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple nameserver groups")
			}
			nameserverGroup = &nsg
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if nameserverGroup == nil {
		resp.Diagnostics.AddError("No match", "NameServerGroup matching parameters not found")
		return
	}

	resp.Diagnostics.Append(nameserverGroupAPIToTerraform(ctx, nameserverGroup, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
