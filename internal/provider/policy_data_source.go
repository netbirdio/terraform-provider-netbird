// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/management/client/rest"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PolicyDataSource{}

func NewPolicyDataSource() datasource.DataSource {
	return &PolicyDataSource{}
}

// PolicyDataSource defines the data source implementation.
type PolicyDataSource struct {
	client *netbird.Client
}

func (d *PolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *PolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read Policy Settings, See [NetBird Docs](https://docs.netbird.io/how-to/manage-network-access#policies) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy ID",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Policy Name",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Policy description",
				Computed:            true,
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Policy Rule Action (accept|drop)",
				Computed:            true,
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Policy Rule Protocol (tcp|udp|icmp|all)",
				Computed:            true,
			},
			"ports": schema.ListAttribute{
				MarkdownDescription: "Policy Rule Ports (mutually exclusive with port_ranges)",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"port_ranges": schema.ListNestedAttribute{
				MarkdownDescription: "Policy Rule Port Ranges (mutually exclusive with ports)",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"start": schema.Int32Attribute{
							Computed: true,
						},
						"end": schema.Int32Attribute{
							Computed: true,
						},
					},
				},
				Computed: true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Policy Rule Enabled",
				Computed:            true,
			},
			"bidirectional": schema.BoolAttribute{
				MarkdownDescription: "Policy Rule Bidirectional",
				Computed:            true,
			},
			"sources": schema.ListAttribute{
				MarkdownDescription: "Policy Rule Source Groups (mutually exclusive with source_resource)",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"source_resource": schema.ObjectAttribute{
				MarkdownDescription: "Policy Rule Source Resource (mutually exclusive with sources)",
				AttributeTypes: map[string]attr.Type{
					"id":   types.StringType,
					"type": types.StringType,
				},
				Computed: true,
			},
			"destinations": schema.ListAttribute{
				MarkdownDescription: "Policy Rule Destination Groups (mutually exclusive with destination_resource)",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"destination_resource": schema.ObjectAttribute{
				MarkdownDescription: "Policy Rule Destination Resource (mutually exclusive with destinations)",
				AttributeTypes: map[string]attr.Type{
					"id":   types.StringType,
					"type": types.StringType,
				},
				Computed: true,
			},
		},
	}
}

func (d *PolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := d.client.Policies.Get(ctx, data.Id.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Error getting Policy", err.Error())
		return
	}

	resp.Diagnostics.Append(policyAPIToTerraform(ctx, policy, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
