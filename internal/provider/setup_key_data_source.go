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
		Description: "Read SetupKey settings and metadata",
		MarkdownDescription: `Read SetupKey settings and metadata, see [NetBird Docs](https://docs.netbird.io/how-to/register-machines-using-setup-keys) for more information.

This can **not** read the plain SetupKey.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "SetupKey ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "SetupKey Name",
				Optional:            true,
				Computed:            true,
			},
			"expires": schema.StringAttribute{
				MarkdownDescription: "SetupKey Expiration Date",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"last_used": schema.StringAttribute{
				MarkdownDescription: "Last usage time",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Setup Key type (one-off or reusable)",
				Computed:            true,
			},
			"usage_limit": schema.Int32Attribute{
				MarkdownDescription: "Maximum number of times SetupKey can be used (0 for unlimited)",
				Computed:            true,
			},
			"used_times": schema.Int32Attribute{
				MarkdownDescription: "Number of times Setup Key was used",
				Computed:            true,
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Setup key state (valid or expired)",
				Computed:            true,
			},
			"auto_groups": schema.ListAttribute{
				MarkdownDescription: "List of groups to automatically assign to peers created through this setup key",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"ephemeral": schema.BoolAttribute{
				MarkdownDescription: "Indicate that the peer will be ephemeral or not, ephemeral peers are deleted after 10 minutes of inactivity",
				Computed:            true,
			},
			"allow_extra_dns_labels": schema.BoolAttribute{
				MarkdownDescription: "Allow extra DNS labels to be added to the peer",
				Computed:            true,
			},
			"valid": schema.BoolAttribute{
				MarkdownDescription: "True if setup key can be used to create more Peers",
				Computed:            true,
			},
			"revoked": schema.BoolAttribute{
				MarkdownDescription: "Set to true to revoke setup key",
				Computed:            true,
			},
		},
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

	setupKeys, err := d.client.SetupKeys.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing SetupKeys", err.Error())
		return
	}

	var setupKey *api.SetupKey
	for _, sk := range setupKeys {
		match := 0
		match += matchString(sk.Id, data.Id)
		match += matchString(sk.Name, data.Name)
		if match > 0 {
			if setupKey != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple setup keys")
			}
			setupKey = &sk
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if setupKey == nil {
		resp.Diagnostics.AddError("No match", "Setup Key matching parameters not found")
	}

	resp.Diagnostics.Append(setupKeyAPIToTerraform(ctx, setupKey, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
