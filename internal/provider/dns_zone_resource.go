// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DNSZone{}
var _ resource.ResourceWithImportState = &DNSZone{}

func NewDNSZone() resource.Resource {
	return &DNSZone{}
}

// DNSZone defines the resource implementation.
type DNSZone struct {
	client *netbird.Client
}

// DNSZoneModel describes the resource data model.
type DNSZoneModel struct {
	Id                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Domain             types.String `tfsdk:"domain"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	EnableSearchDomain types.Bool   `tfsdk:"enable_search_domain"`
	DistributionGroups types.Set    `tfsdk:"distribution_groups"`
}

func (r *DNSZone) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (r *DNSZone) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage custom DNS zones",
		MarkdownDescription: "Create and manage custom DNS zones. See [NetBird Docs](https://docs.netbird.io/manage/dns/custom-zones) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "DNS Zone ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "DNS Zone name identifier",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Zone domain (FQDN)",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "DNS Zone status",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"enable_search_domain": schema.BoolAttribute{
				MarkdownDescription: "Enable this zone as a search domain",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"distribution_groups": schema.SetAttribute{
				MarkdownDescription: "Group IDs that define groups of peers that will resolve this zone",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Set{setplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *DNSZone) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func dnsZoneAPIToTerraform(ctx context.Context, zone *api.Zone, data *DNSZoneModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(zone.Id)
	data.Name = types.StringValue(zone.Name)
	data.Domain = types.StringValue(zone.Domain)
	data.Enabled = types.BoolValue(zone.Enabled)
	data.EnableSearchDomain = types.BoolValue(zone.EnableSearchDomain)

	distributionGroups, diag := types.SetValueFrom(ctx, types.StringType, zone.DistributionGroups)
	ret.Append(diag...)
	data.DistributionGroups = distributionGroups

	return ret
}

func (r *DNSZone) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DNSZoneModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	enabled := data.Enabled.ValueBool()
	zoneReq := api.PostApiDnsZonesJSONRequestBody{
		Name:               data.Name.ValueString(),
		Domain:             data.Domain.ValueString(),
		Enabled:            &enabled,
		EnableSearchDomain: data.EnableSearchDomain.ValueBool(),
		DistributionGroups: stringSetDefault(ctx, data.DistributionGroups, []string{}),
	}

	zone, err := r.client.DNSZones.CreateZone(ctx, zoneReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS zone", err.Error())
		return
	}

	resp.Diagnostics.Append(dnsZoneAPIToTerraform(ctx, zone, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSZone) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DNSZoneModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNSZones.GetZone(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting DNS Zone", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(dnsZoneAPIToTerraform(ctx, zone, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSZone) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DNSZoneModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	enabled := data.Enabled.ValueBool()
	zoneReq := api.PutApiDnsZonesZoneIdJSONRequestBody{
		Name:               data.Name.ValueString(),
		Domain:             data.Domain.ValueString(),
		Enabled:            &enabled,
		EnableSearchDomain: data.EnableSearchDomain.ValueBool(),
		DistributionGroups: stringSetDefault(ctx, data.DistributionGroups, []string{}),
	}

	zone, err := r.client.DNSZones.UpdateZone(ctx, data.Id.ValueString(), zoneReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNS Zone", err.Error())
		return
	}

	resp.Diagnostics.Append(dnsZoneAPIToTerraform(ctx, zone, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSZone) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DNSZoneModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DNSZones.DeleteZone(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DNS Zone", err.Error())
	}
}

func (r *DNSZone) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
