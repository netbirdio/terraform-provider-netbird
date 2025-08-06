// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SetupKey{}
var _ resource.ResourceWithImportState = &SetupKey{}

func NewSetupKey() resource.Resource {
	return &SetupKey{}
}

// SetupKey defines the resource implementation.
type SetupKey struct {
	client *netbird.Client
}

// SetupKeyModel describes the resource data model.
type SetupKeyModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Expires             types.String `tfsdk:"expires"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
	LastUsed            types.String `tfsdk:"last_used"`
	Key                 types.String `tfsdk:"key"`
	Type                types.String `tfsdk:"type"`
	UsageLimit          types.Int32  `tfsdk:"usage_limit"`
	UsedTimes           types.Int32  `tfsdk:"used_times"`
	ExpirySeconds       types.Int32  `tfsdk:"expiry_seconds"`
	State               types.String `tfsdk:"state"`
	AutoGroups          types.List   `tfsdk:"auto_groups"`
	Ephemeral           types.Bool   `tfsdk:"ephemeral"`
	AllowExtraDnsLabels types.Bool   `tfsdk:"allow_extra_dns_labels"`
	Valid               types.Bool   `tfsdk:"valid"`
	Revoked             types.Bool   `tfsdk:"revoked"`
}

func (r *SetupKey) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_setup_key"
}

func (r *SetupKey) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and Manage Setup Keys",
		MarkdownDescription: "Create and Manage Setup Keys, see [NetBird Docs](https://docs.netbird.io/how-to/register-machines-using-setup-keys) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "SetupKey ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "SetupKey Name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"expires": schema.StringAttribute{
				MarkdownDescription: "SetupKey Expiration Date",
				Computed:            true,
			},
			"expiry_seconds": schema.Int32Attribute{
				MarkdownDescription: "Expiry time in seconds (0 is unlimited)",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				PlanModifiers:       []planmodifier.Int32{int32planmodifier.RequiresReplace()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp",
				Computed:            true,
			},
			"last_used": schema.StringAttribute{
				MarkdownDescription: "Last usage time",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Plaintext setup key",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Setup Key type (one-off or reusable)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("one-off"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.OneOf("one-off", "reusable")},
			},
			"usage_limit": schema.Int32Attribute{
				MarkdownDescription: "Maximum number of times SetupKey can be used (0 for unlimited)",
				Computed:            true,
				Optional:            true,
				PlanModifiers:       []planmodifier.Int32{int32planmodifier.RequiresReplace()},
				Default:             int32default.StaticInt32(0),
			},
			"used_times": schema.Int32Attribute{
				MarkdownDescription: "Number of times Setup Key was used",
				Computed:            true,
				PlanModifiers:       []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Setup key state (valid or expired)",
				Computed:            true,
			},
			"auto_groups": schema.ListAttribute{
				MarkdownDescription: "List of groups to automatically assign to peers created through this setup key",
				Computed:            true,
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
			"ephemeral": schema.BoolAttribute{
				MarkdownDescription: "Indicate that the peer will be ephemeral or not, ephemeral peers are deleted after 10 minutes of inactivity",
				Computed:            true,
				Optional:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"allow_extra_dns_labels": schema.BoolAttribute{
				MarkdownDescription: "Allow extra DNS labels to be added to the peer",
				Computed:            true,
				Optional:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"valid": schema.BoolAttribute{
				MarkdownDescription: "True if setup key can be used to create more Peers",
				Computed:            true,
			},
			"revoked": schema.BoolAttribute{
				MarkdownDescription: "Set to true to revoke setup key",
				Computed:            true,
				Optional:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *SetupKey) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func setupKeyAPIToTerraform(ctx context.Context, setupKey *api.SetupKey, data *SetupKeyModel) diag.Diagnostics {
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

func (r *SetupKey) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SetupKeyModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	createRequest := api.CreateSetupKeyRequest{
		AllowExtraDnsLabels: data.AllowExtraDnsLabels.ValueBoolPointer(),
		AutoGroups:          stringListDefault(ctx, data.AutoGroups, []string{}),
		Ephemeral:           data.Ephemeral.ValueBoolPointer(),
		ExpiresIn:           int(data.ExpirySeconds.ValueInt32()),
		Name:                data.Name.ValueString(),
		Type:                data.Type.ValueString(),
		UsageLimit:          int(data.UsageLimit.ValueInt32()),
	}

	setupKey, err := r.client.SetupKeys.Create(ctx, createRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error creating SetupKey", err.Error())
		return
	}

	resp.Diagnostics.Append(setupKeyAPIToTerraform(ctx, &api.SetupKey{
		AllowExtraDnsLabels: setupKey.AllowExtraDnsLabels,
		AutoGroups:          setupKey.AutoGroups,
		Ephemeral:           setupKey.Ephemeral,
		Expires:             setupKey.Expires,
		Id:                  setupKey.Id,
		Key:                 setupKey.Key,
		LastUsed:            setupKey.LastUsed,
		Name:                setupKey.Name,
		Revoked:             setupKey.Revoked,
		State:               setupKey.State,
		Type:                setupKey.Type,
		UpdatedAt:           setupKey.UpdatedAt,
		UsageLimit:          setupKey.UsageLimit,
		UsedTimes:           setupKey.UsedTimes,
		Valid:               setupKey.Valid,
	}, &data)...)
	data.Key = types.StringValue(setupKey.Key)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SetupKey) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SetupKeyModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	setupKey, err := r.client.SetupKeys.Get(ctx, data.Id.ValueString())

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting SetupKey", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(setupKeyAPIToTerraform(ctx, setupKey, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SetupKey) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SetupKeyModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	setupKey, err := r.client.SetupKeys.Update(ctx, data.Id.ValueString(), api.SetupKeyRequest{
		AutoGroups: stringListDefault(ctx, data.AutoGroups, []string{}),
		Revoked:    data.Revoked.ValueBool(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Error updating Setup Key", err.Error())
		return
	}

	resp.Diagnostics.Append(setupKeyAPIToTerraform(ctx, setupKey, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *SetupKey) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SetupKeyModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.SetupKeys.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting SetupKey", err.Error())
	}
}

func (r *SetupKey) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
