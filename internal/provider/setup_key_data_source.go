// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SetupKeyDataSource{}

func NewSetupKeyDataSource() datasource.DataSource {
	return &SetupKeyDataSource{}
}

// SetupKeyDataSourceModel describes the resource data model.
type SetupKeyDataSourceModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Expires             types.String `tfsdk:"expires"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
	LastUsed            types.String `tfsdk:"last_used"`
	Type                types.String `tfsdk:"type"`
	UsageLimit          types.Int32  `tfsdk:"usage_limit"`
	UsedTimes           types.Int32  `tfsdk:"used_times"`
	State               types.String `tfsdk:"state"`
	AutoGroups          types.List   `tfsdk:"auto_groups"`
	Ephemeral           types.Bool   `tfsdk:"ephemeral"`
	AllowExtraDnsLabels types.Bool   `tfsdk:"allow_extra_dns_labels"`
	Valid               types.Bool   `tfsdk:"valid"`
	Revoked             types.Bool   `tfsdk:"revoked"`
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

func setupKeyDataSourceAPIToTerraform(ctx context.Context, setupKey *api.SetupKey, data *SetupKeyDataSourceModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(setupKey.Id)
	data.Name = types.StringValue(setupKey.Name)
	data.Expires = types.StringValue(setupKey.Expires.Format(time.RFC3339))
	data.UpdatedAt = types.StringValue(setupKey.UpdatedAt.Format(time.RFC3339))
	data.LastUsed = types.StringValue(setupKey.LastUsed.Format(time.RFC3339))
	data.AllowExtraDnsLabels = types.BoolValue(setupKey.AllowExtraDnsLabels)
	l, diag := types.ListValueFrom(ctx, types.StringType, setupKey.AutoGroups)
	ret.Append(diag...)
	data.AutoGroups = l
	data.Ephemeral = types.BoolValue(setupKey.Ephemeral)
	data.Revoked = types.BoolValue(setupKey.Revoked)
	data.State = types.StringValue(setupKey.State)
	data.Type = types.StringValue(setupKey.Type)
	data.UsageLimit = types.Int32Value(int32(setupKey.UsageLimit))
	data.UsedTimes = types.Int32Value(int32(setupKey.UsedTimes))
	data.Valid = types.BoolValue(setupKey.Valid)
	return ret
}

func (d *SetupKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SetupKeyDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
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
		return
	}

	resp.Diagnostics.Append(setupKeyDataSourceAPIToTerraform(ctx, setupKey, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
