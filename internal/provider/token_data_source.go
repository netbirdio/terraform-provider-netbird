// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &TokenDataSource{}

func NewTokenDataSource() datasource.DataSource {
	return &TokenDataSource{}
}

// TokenDataSource defines the data source implementation.
type TokenDataSource struct {
	client *netbird.Client
}

type TokenDataSourceModel struct {
	Id             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ExpirationDate types.String `tfsdk:"expiration_date"`
	UserID         types.String `tfsdk:"user_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	LastUsed       types.String `tfsdk:"last_used"`
}

func (d *TokenDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (d *TokenDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Personal Access Token metadata",
		MarkdownDescription: "Read Personal Access Token metadata, see [NetBird Docs](https://docs.netbird.io/how-to/access-netbird-public-api#creating-an-access-token) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Token ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Token Name",
				Optional:            true,
				Computed:            true,
			},
			"expiration_date": schema.StringAttribute{
				MarkdownDescription: "Token Expiration Date",
				Computed:            true,
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User ID",
				Required:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"last_used": schema.StringAttribute{
				MarkdownDescription: "Last usage time",
				Computed:            true,
			},
		},
	}
}

func (d *TokenDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func tokenDataSourceAPIToTerraform(token *api.PersonalAccessToken, data *TokenDataSourceModel) {
	data.Id = types.StringValue(token.Id)
	data.Name = types.StringValue(token.Name)
	data.ExpirationDate = types.StringValue(token.ExpirationDate.Format(time.RFC3339))
	data.UserID = types.StringValue(token.CreatedBy)
	data.CreatedAt = types.StringValue(token.CreatedAt.Format(time.RFC3339))
	if token.LastUsed == nil {
		data.LastUsed = types.StringNull()
	} else {
		data.LastUsed = types.StringValue(token.LastUsed.Format(time.RFC3339))
	}
}

func (d *TokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TokenDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
		return
	}

	tokens, err := d.client.Tokens.List(ctx, data.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing Tokens", err.Error())
		return
	}

	var token *api.PersonalAccessToken
	for _, t := range tokens {
		match := 0
		match += matchString(t.Id, data.Id)
		match += matchString(t.Name, data.Name)
		if match > 0 {
			if token != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple tokens")
			}
			token = &t
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if token == nil {
		resp.Diagnostics.AddError("No match", "Token matching parameters not found")
		return
	}

	tokenDataSourceAPIToTerraform(token, &data)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
