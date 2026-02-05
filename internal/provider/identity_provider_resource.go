// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &IdentityProvider{}
var _ resource.ResourceWithImportState = &IdentityProvider{}

func NewIdentityProvider() resource.Resource {
	return &IdentityProvider{}
}

// IdentityProvider defines the resource implementation.
type IdentityProvider struct {
	client *netbird.Client
}

// IdentityProviderModel describes the resource data model.
type IdentityProviderModel struct {
	Id           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	ClientId     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Issuer       types.String `tfsdk:"issuer"`
}

func (r *IdentityProvider) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_provider"
}

func (r *IdentityProvider) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage Identity Providers",
		MarkdownDescription: "Create and manage Identity Providers, see [NetBird Docs](https://docs.netbird.io/selfhosted/identity-providers) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identity provider ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name for the identity provider",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of identity provider (entra, google, microsoft, oidc, okta, pocketid, zitadel)",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("entra", "google", "microsoft", "oidc", "okta", "pocketid", "zitadel")},
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "OAuth2 client ID",
				Required:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "OAuth2 client secret",
				Required:            true,
				Sensitive:           true,
			},
			"issuer": schema.StringAttribute{
				MarkdownDescription: "OIDC issuer URL",
				Required:            true,
			},
		},
	}
}

func (r *IdentityProvider) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func identityProviderAPIToTerraform(idp *api.IdentityProvider, data *IdentityProviderModel) {
	data.Id = types.StringPointerValue(idp.Id)
	data.Name = types.StringValue(idp.Name)
	data.Type = types.StringValue(string(idp.Type))
	data.ClientId = types.StringValue(idp.ClientId)
	data.Issuer = types.StringValue(idp.Issuer)
}

func identityProviderTerraformToAPI(_ context.Context, data IdentityProviderModel) api.IdentityProviderRequest {
	return api.IdentityProviderRequest{
		Name:         data.Name.ValueString(),
		Type:         api.IdentityProviderType(data.Type.ValueString()),
		ClientId:     data.ClientId.ValueString(),
		ClientSecret: data.ClientSecret.ValueString(),
		Issuer:       data.Issuer.ValueString(),
	}
}

func (r *IdentityProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IdentityProviderModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idpReq := identityProviderTerraformToAPI(ctx, data)

	idp, err := r.client.IdentityProviders.Create(ctx, idpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Identity Provider", err.Error())
		return
	}

	identityProviderAPIToTerraform(idp, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IdentityProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IdentityProviderModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idp, err := r.client.IdentityProviders.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error getting Identity Provider", err.Error())
		return
	}

	identityProviderAPIToTerraform(idp, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IdentityProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IdentityProviderModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	idpReq := identityProviderTerraformToAPI(ctx, data)

	idp, err := r.client.IdentityProviders.Update(ctx, data.Id.ValueString(), idpReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Identity Provider", err.Error())
		return
	}

	identityProviderAPIToTerraform(idp, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IdentityProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IdentityProviderModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.IdentityProviders.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Identity Provider", err.Error())
	}
}

func (r *IdentityProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
