// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/management/client/rest"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SetupKeyDataSource{}

func NewSetupKeyDataSource() datasource.DataSource {
	return &SetupKeyDataSource{}
}

// SetupKeyDataSource defines the data source implementation.
type SetupKeyDataSource struct {
	client *netbird.Client
}

func (d *SetupKeyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_setup_key"
}

func (d *SetupKeyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read SetupKey settings and metadata, see [NetBird Docs](https://docs.netbird.io/how-to/register-machines-using-setup-keys) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SetupKey ID",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SetupKey Name",
			},
			"expires": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SetupKey Expiration Date",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp",
			},
			"last_used": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last usage time",
			},
			"key": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Plaintext setup key",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"usage_limit": schema.Int32Attribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"used_times": schema.Int32Attribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"auto_groups": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "",
			},
			"ephemeral": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"allow_extra_dns_labels": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"valid": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "",
			},
			"revoked": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "",
			}},
	}
}

func (d *SetupKeyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SetupKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SetupKeyModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	setupKey, err := d.client.SetupKeys.Get(ctx, data.Id.ValueString())

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			data.Id = types.StringNull()
		} else {
			resp.Diagnostics.AddError("Error getting SetupKey", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(setupKeyAPIToTerraform(ctx, setupKey, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
