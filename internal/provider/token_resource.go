// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Token{}
var _ resource.ResourceWithImportState = &Token{}

func NewToken() resource.Resource {
	return &Token{}
}

// Token defines the resource implementation.
type Token struct {
	client *netbird.Client
}

// TokenModel describes the resource data model.
type TokenModel struct {
	Id             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	ExpirationDate types.String `tfsdk:"expiration_date"`
	ExpirationDays types.Int32  `tfsdk:"expiration_days"`
	UserID         types.String `tfsdk:"user_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	LastUsed       types.String `tfsdk:"last_used"`
	Token          types.String `tfsdk:"token"`
}

func (r *Token) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (r *Token) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Create and Manage Personal Access Tokens",
		MarkdownDescription: `Create and Manage Personal Access Tokens, see [NetBird Docs](https://docs.netbird.io/how-to/access-netbird-public-api#creating-an-access-token) for more information.

Personal Access Tokens can only be created for current user if the current user is a Service User.`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Token ID",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Token Name",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"expiration_date": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Token Expiration Date",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expiration_days": schema.Int32Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int32{int32planmodifier.RequiresReplace()},
				Validators:    []validator.Int32{int32validator.Between(1, 365)},
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User ID",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_used": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last usage time",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Plaintext token",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *Token) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func tokenAPIToTerraform(token *api.PersonalAccessToken, data *TokenModel) {
	data.Id = types.StringValue(token.Id)
	data.Name = types.StringValue(token.Name)
	data.ExpirationDate = types.StringValue(token.ExpirationDate.Format(time.RFC3339))
	data.CreatedAt = types.StringValue(token.CreatedAt.Format(time.RFC3339))
	if token.LastUsed == nil {
		data.LastUsed = types.StringNull()
	} else {
		data.LastUsed = types.StringValue(token.LastUsed.Format(time.RFC3339))
	}
}

func (r *Token) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TokenModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	createRequest := api.PersonalAccessTokenRequest{
		Name:      data.Name.ValueString(),
		ExpiresIn: int(data.ExpirationDays.ValueInt32()),
	}

	token, err := r.client.Tokens.Create(ctx, data.UserID.ValueString(), createRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Token", err.Error())
		return
	}

	tokenAPIToTerraform(&token.PersonalAccessToken, &data)
	data.Token = types.StringValue(token.PlainToken)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Token) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TokenModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	token, err := r.client.Tokens.Get(ctx, data.UserID.ValueString(), data.Id.ValueString())

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting Token", err.Error())
		}
		return
	}

	tokenAPIToTerraform(token, &data)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Token) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TokenModel

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}
	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.AddError("Invalid Operation", "Personal Access Tokens can't be updated")
}

func (r *Token) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TokenModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Tokens.Delete(ctx, data.UserID.ValueString(), data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Token", err.Error())
	}
}

func (r *Token) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	splitID := strings.Split(req.ID, "/")
	if len(splitID) != 2 {
		resp.Diagnostics.AddError("Error importing Token", "Invalid import ID, must be in format `userID/tokenID`")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), splitID[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), splitID[1])...)
}
