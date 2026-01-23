// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DNSRecord{}
var _ resource.ResourceWithImportState = &DNSRecord{}

func NewDNSRecord() resource.Resource {
	return &DNSRecord{}
}

// DNSRecord defines the resource implementation.
type DNSRecord struct {
	client *netbird.Client
}

// DNSRecordModel describes the resource data model.
type DNSRecordModel struct {
	Id      types.String `tfsdk:"id"`
	ZoneId  types.String `tfsdk:"zone_id"`
	Name    types.String `tfsdk:"name"`
	Type    types.String `tfsdk:"type"`
	Content types.String `tfsdk:"content"`
	Ttl     types.Int64  `tfsdk:"ttl"`
}

func (r *DNSRecord) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *DNSRecord) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage DNS records within DNS zones",
		MarkdownDescription: "Create and manage DNS records within DNS zones. See [NetBird Docs](https://docs.netbird.io/manage/dns/custom-zones) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "DNS Record ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone_id": schema.StringAttribute{
				MarkdownDescription: "DNS Zone ID that this record belongs to",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "DNS record name as a full FQDN (e.g., 'www.example.com' or 'example.com' for root domain). Short names like 'www' or '@' are not supported",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "DNS record type (A, AAAA, or CNAME)",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("A", "AAAA", "CNAME"),
				},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "DNS record content (IP address for A/AAAA, domain for CNAME)",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "Time to live in seconds",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(300),
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Validators:          []validator.Int64{int64validator.Between(60, 86400)},
			},
		},
	}
}

func (r *DNSRecord) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func dnsRecordAPIToTerraform(record *api.DNSRecord, zoneId string, data *DNSRecordModel) {
	data.Id = types.StringValue(record.Id)
	data.ZoneId = types.StringValue(zoneId)
	data.Name = types.StringValue(record.Name)
	data.Type = types.StringValue(string(record.Type))
	data.Content = types.StringValue(record.Content)
	data.Ttl = types.Int64Value(int64(record.Ttl))
}

func (r *DNSRecord) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DNSRecordModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	recordReq := api.PostApiDnsZonesZoneIdRecordsJSONRequestBody{
		Name:    data.Name.ValueString(),
		Type:    api.DNSRecordType(data.Type.ValueString()),
		Content: data.Content.ValueString(),
		Ttl:     int(data.Ttl.ValueInt64()),
	}

	record, err := r.client.DNSZones.CreateRecord(ctx, data.ZoneId.ValueString(), recordReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS record", err.Error())
		return
	}

	dnsRecordAPIToTerraform(record, data.ZoneId.ValueString(), &data)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSRecord) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DNSRecordModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	record, err := r.client.DNSZones.GetRecord(ctx, data.ZoneId.ValueString(), data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting DNS Record", err.Error())
		}
		return
	}

	dnsRecordAPIToTerraform(record, data.ZoneId.ValueString(), &data)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSRecord) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DNSRecordModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	recordReq := api.PutApiDnsZonesZoneIdRecordsRecordIdJSONRequestBody{
		Name:    data.Name.ValueString(),
		Type:    api.DNSRecordType(data.Type.ValueString()),
		Content: data.Content.ValueString(),
		Ttl:     int(data.Ttl.ValueInt64()),
	}

	record, err := r.client.DNSZones.UpdateRecord(ctx, data.ZoneId.ValueString(), data.Id.ValueString(), recordReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNS Record", err.Error())
		return
	}

	dnsRecordAPIToTerraform(record, data.ZoneId.ValueString(), &data)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSRecord) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DNSRecordModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DNSZones.DeleteRecord(ctx, data.ZoneId.ValueString(), data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DNS Record", err.Error())
	}
}

func (r *DNSRecord) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: zone_id:record_id
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected import ID format: zone_id:record_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
