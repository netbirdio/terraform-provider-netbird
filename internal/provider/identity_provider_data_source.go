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
var _ datasource.DataSource = &IdentityProviderDataSource{}

func NewIdentityProviderDataSource() datasource.DataSource {
	return &IdentityProviderDataSource{}
}

// IdentityProviderDataSource defines the data source implementation.
type IdentityProviderDataSource struct {
	client *netbird.Client
}

// IdentityProviderDataSourceModel describes the data source data model.
type IdentityProviderDataSourceModel struct {
	Id       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	ClientId types.String `tfsdk:"client_id"`
	Issuer   types.String `tfsdk:"issuer"`
}

func (d *IdentityProviderDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_provider"
}

func (d *IdentityProviderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Identity Provider settings",
		MarkdownDescription: "Read Identity Provider settings, see [NetBird Docs](https://docs.netbird.io/selfhosted/identity-providers) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identity provider ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for the identity provider",
				Optional:            true,
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of identity provider (entra, google, microsoft, oidc, okta, pocketid, zitadel)",
				Computed:            true,
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "OAuth2 client ID",
				Computed:            true,
			},
			"issuer": schema.StringAttribute{
				MarkdownDescription: "OIDC issuer URL",
				Computed:            true,
			},
		},
	}
}

func (d *IdentityProviderDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IdentityProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IdentityProviderDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
		return
	}

	idps, err := d.client.IdentityProviders.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Identity Providers", err.Error())
		return
	}

	var idp *api.IdentityProvider
	for _, i := range idps {
		match := 0
		if i.Id != nil {
			match += matchString(*i.Id, data.Id)
		}
		match += matchString(i.Name, data.Name)
		if match > 0 {
			if idp != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple identity providers")
			}
			idp = &i
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if idp == nil {
		resp.Diagnostics.AddError("No match", "Identity Provider matching parameters not found")
		return
	}

	data.Id = types.StringPointerValue(idp.Id)
	data.Name = types.StringValue(idp.Name)
	data.Type = types.StringValue(string(idp.Type))
	data.ClientId = types.StringValue(idp.ClientId)
	data.Issuer = types.StringValue(idp.Issuer)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
