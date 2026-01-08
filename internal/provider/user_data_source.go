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
var _ datasource.DataSource = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

// UserDataSource defines the data source implementation.
type UserDataSource struct {
	client *netbird.Client
}

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Existing Users metadata",
		MarkdownDescription: "Read Existing Users metadata, see [NetBird Docs](https://docs.netbird.io/how-to/add-users-to-your-network) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a user",
				Optional:            true,
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "User Email",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "User Name",
				Optional:            true,
				Computed:            true,
			},
			"last_login": schema.StringAttribute{
				MarkdownDescription: "User Last Login timedate",
				Computed:            true,
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "User's NetBird account role (owner|admin|user|billing_admin|auditor|network_admin).",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "User status (active or invited)",
				Computed:            true,
			},
			"issued": schema.StringAttribute{
				MarkdownDescription: "User issue method",
				Computed:            true,
			},
			"auto_groups": schema.ListAttribute{
				MarkdownDescription: "Group IDs to auto-assign to peers registered by this user",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"is_current": schema.BoolAttribute{
				MarkdownDescription: "Set to true if the caller user is the same as the resource user",
				Computed:            true,
			},
			"is_service_user": schema.BoolAttribute{
				MarkdownDescription: "If set to true, user is a Service Account User",
				Computed:            true,
			},
			"is_blocked": schema.BoolAttribute{
				MarkdownDescription: "If set to true then user is blocked and can't use the system",
				Computed:            true,
			},
		},
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name, data.Email) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name, email)")
		return
	}

	users, err := d.client.Users.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing users", err.Error())
		return
	}

	var user *api.User

	for _, u := range users {
		match := 0
		match += matchString(u.Id, data.Id)
		match += matchString(u.Name, data.Name)
		match += matchString(u.Email, data.Email)
		if match > 0 {
			if user != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple users")
			}
			user = &u
		}
	}

	if user == nil {
		resp.Diagnostics.AddError("No match", "User matching parameters not found")
		return
	}

	resp.Diagnostics.Append(userAPIToTerraform(ctx, user, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
