// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DNSSettings{}
var _ resource.ResourceWithImportState = &DNSSettings{}

func NewDNSSettings() resource.Resource {
	return &DNSSettings{}
}

// DNSSettings defines the resource implementation.
type DNSSettings struct {
	client *netbird.Client
}

// DNSSettingsModel describes the resource data model.
type DNSSettingsModel struct {
	DisabledManagementGroups types.List `tfsdk:"disabled_management_groups"`
}

func (r *DNSSettings) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_settings"
}

func (r *DNSSettings) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Account DNS Settings (Only one of this resource should be used per provider).",
		Description:         "Manage Account DNS Settings",

		Attributes: map[string]schema.Attribute{
			"disabled_management_groups": schema.ListAttribute{
				MarkdownDescription: "Groups whose DNS management is disabled",
				Required:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
		},
	}
}

func (r *DNSSettings) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func dnsSettingsAPIToTerraform(ctx context.Context, dnsSettings *api.DNSSettings, data *DNSSettingsModel) diag.Diagnostics {
	var ret diag.Diagnostics
	l, diag := types.ListValueFrom(ctx, types.StringType, dnsSettings.DisabledManagementGroups)
	ret.Append(diag...)
	data.DisabledManagementGroups = l
	return ret
}

func (r *DNSSettings) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DNSSettingsModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dnsSettings, err := r.client.DNS.GetSettings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error getting DNSSettings", err.Error())
		return
	}

	updateRequest := api.PutApiDnsSettingsJSONRequestBody{
		DisabledManagementGroups: stringListDefault(ctx, data.DisabledManagementGroups, dnsSettings.DisabledManagementGroups),
	}

	dnsSettings, err = r.client.DNS.UpdateSettings(ctx, updateRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNSSettings", err.Error())
		return
	}

	resp.Diagnostics.Append(dnsSettingsAPIToTerraform(ctx, dnsSettings, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSSettings) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DNSSettingsModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dnsSettings, err := r.client.DNS.GetSettings(ctx)

	if err != nil {
		resp.Diagnostics.AddError("Error getting DNSSettings", err.Error())
		return
	}

	resp.Diagnostics.Append(dnsSettingsAPIToTerraform(ctx, dnsSettings, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSSettings) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DNSSettingsModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dnsSettings, err := r.client.DNS.UpdateSettings(ctx, api.PutApiDnsSettingsJSONRequestBody{
		DisabledManagementGroups: stringListDefault(ctx, data.DisabledManagementGroups, []string{}),
	})

	if err != nil {
		resp.Diagnostics.AddError("Error updating DNSSettings", err.Error())
		return
	}

	resp.Diagnostics.Append(dnsSettingsAPIToTerraform(ctx, dnsSettings, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSSettings) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Do nothing
}

func (r *DNSSettings) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
