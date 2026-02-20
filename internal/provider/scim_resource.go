// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Scim{}
var _ resource.ResourceWithImportState = &Scim{}

func NewScim() resource.Resource {
	return &Scim{}
}

// Scim defines the resource implementation.
type Scim struct {
	client *netbird.Client
}

// ScimModel describes the resource data model.
type ScimModel struct {
	Id                types.String `tfsdk:"id"`
	ProviderName      types.String `tfsdk:"provider_name"`
	Prefix            types.String `tfsdk:"prefix"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	GroupPrefixes     types.List   `tfsdk:"group_prefixes"`
	UserGroupPrefixes types.List   `tfsdk:"user_group_prefixes"`
	AuthToken         types.String `tfsdk:"auth_token"`
	LastSyncedAt      types.String `tfsdk:"last_synced_at"`
}

func (r *Scim) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim"
}

func (r *Scim) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage SCIM integrations",
		MarkdownDescription: "Create and manage SCIM integrations, see [NetBird Docs](https://docs.netbird.io/manage/team/idp-sync) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "SCIM integration ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "Name of the SCIM identity provider",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"prefix": schema.StringAttribute{
				MarkdownDescription: "The connection prefix used for the SCIM provider.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether the integration is enabled",
				Optional:            true,
				Computed:            true,
			},
			"group_prefixes": schema.ListAttribute{
				MarkdownDescription: "List of start_with string patterns for groups to sync",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"user_group_prefixes": schema.ListAttribute{
				MarkdownDescription: "List of start_with string patterns for groups which users to sync",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"auth_token": schema.StringAttribute{
				MarkdownDescription: "SCIM API token (only available in full after creation)",
				Computed:            true,
				Sensitive:           true,
			},
			"last_synced_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp of when the integration was last synced",
				Computed:            true,
			},
		},
	}
}

func (r *Scim) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func scimAPIToTerraform(ctx context.Context, scim *api.ScimIntegration, data *ScimModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(strconv.FormatInt(scim.Id, 10))
	data.ProviderName = types.StringValue(scim.Provider)
	data.Enabled = types.BoolValue(scim.Enabled)
	data.LastSyncedAt = types.StringValue(scim.LastSyncedAt.Format("2006-01-02T15:04:05Z"))
	data.GroupPrefixes, d = types.ListValueFrom(ctx, types.StringType, scim.GroupPrefixes)
	ret.Append(d...)
	data.UserGroupPrefixes, d = types.ListValueFrom(ctx, types.StringType, scim.UserGroupPrefixes)
	ret.Append(d...)
	// auth_token and prefix are preserved from existing state
	return ret
}

func (r *Scim) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ScimModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scimReq := api.CreateScimIntegrationRequest{
		Provider:          data.ProviderName.ValueString(),
		Prefix:            data.Prefix.ValueString(),
		GroupPrefixes:     stringListDefaultPointer(ctx, data.GroupPrefixes, nil),
		UserGroupPrefixes: stringListDefaultPointer(ctx, data.UserGroupPrefixes, nil),
	}

	scim, err := r.client.SCIM.Create(ctx, scimReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating SCIM integration", err.Error())
		return
	}

	resp.Diagnostics.Append(scimAPIToTerraform(ctx, scim, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Capture the full auth_token from the create response
	data.AuthToken = types.StringValue(scim.AuthToken)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Scim) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ScimModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scim, err := r.client.SCIM.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error getting SCIM integration", err.Error())
		return
	}

	// Preserve auth_token and prefix from state
	authToken := data.AuthToken
	prefix := data.Prefix

	resp.Diagnostics.Append(scimAPIToTerraform(ctx, scim, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.AuthToken = authToken
	data.Prefix = prefix

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Scim) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ScimModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scimReq := api.UpdateScimIntegrationRequest{
		Enabled:           boolDefaultPointer(data.Enabled, nil),
		GroupPrefixes:     stringListDefaultPointer(ctx, data.GroupPrefixes, nil),
		UserGroupPrefixes: stringListDefaultPointer(ctx, data.UserGroupPrefixes, nil),
	}

	scim, err := r.client.SCIM.Update(ctx, data.Id.ValueString(), scimReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating SCIM integration", err.Error())
		return
	}

	// Preserve auth_token and prefix from state
	var state ScimModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(scimAPIToTerraform(ctx, scim, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.AuthToken = state.AuthToken
	data.Prefix = state.Prefix

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Scim) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ScimModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.SCIM.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting SCIM integration", err.Error())
	}
}

func (r *Scim) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
