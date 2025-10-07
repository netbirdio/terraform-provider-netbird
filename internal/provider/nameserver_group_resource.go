// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

const (
	fqdnRegex = `^(?:[_a-z0-9](?:[_a-z0-9-]{0,61}[a-z0-9])?\.)+(?:[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?)?$`
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NameserverGroup{}
var _ resource.ResourceWithImportState = &NameserverGroup{}

func NewNameserverGroup() resource.Resource {
	return &NameserverGroup{}
}

// NameserverGroup defines the resource implementation.
type NameserverGroup struct {
	client *netbird.Client
}

// NameserverGroupModel describes the resource data model.
type NameserverGroupModel struct {
	Id                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	Groups               types.List   `tfsdk:"groups"`
	Domains              types.List   `tfsdk:"domains"`
	Nameservers          types.List   `tfsdk:"nameservers"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	Primary              types.Bool   `tfsdk:"primary"`
	SearchDomainsEnabled types.Bool   `tfsdk:"search_domains_enabled"`
}

func (r *NameserverGroup) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nameserver_group"
}

func (r *NameserverGroup) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Create and manage Nameserver Groups, see [NetBird Docs](https://docs.netbird.io/how-to/manage-dns-in-your-network#managing-nameserver-groups) for more information.",
		Description:         "Create and manage Nameserver Groups",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "NameserverGroup ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of nameserver group",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 40)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the nameserver group",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"groups": schema.ListAttribute{
				MarkdownDescription: "Distribution group IDs that defines group of peers that will use this nameserver group",
				ElementType:         types.StringType,
				Required:            true,
				Validators:          []validator.List{listvalidator.SizeAtLeast(1), listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
			},
			"domains": schema.ListAttribute{
				MarkdownDescription: "Match domain list. It should be empty only if primary is true.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(fqdnRegex), "Invalid domain name"))},
			},
			"nameservers": schema.ListNestedAttribute{
				MarkdownDescription: "Nameserver list",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip": schema.StringAttribute{
							MarkdownDescription: "Nameserver IP",
							Required:            true,
						},
						"ns_type": schema.StringAttribute{
							MarkdownDescription: "Nameserver Type",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("udp"),
						},
						"port": schema.Int32Attribute{
							MarkdownDescription: "Nameserver Port",
							Optional:            true,
							Computed:            true,
							Default:             int32default.StaticInt32(53),
						},
					},
				},
				Required:   true,
				Validators: []validator.List{listvalidator.SizeBetween(1, 3)},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Nameserver group status",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"primary": schema.BoolAttribute{
				MarkdownDescription: "Defines if a nameserver group is primary that resolves all domains. It should be true only if domains list is empty.",
				Optional:            true,
				Computed:            true,
			},
			"search_domains_enabled": schema.BoolAttribute{
				MarkdownDescription: "Search domain status for match domains. It should be true only if domains list is not empty.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *NameserverGroup) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func nameserverGroupAPIToTerraform(ctx context.Context, nameserverGroup *api.NameserverGroup, data *NameserverGroupModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(nameserverGroup.Id)
	data.Name = types.StringValue(nameserverGroup.Name)
	data.Description = types.StringValue(nameserverGroup.Description)
	data.Enabled = types.BoolValue(nameserverGroup.Enabled)
	data.SearchDomainsEnabled = types.BoolValue(nameserverGroup.SearchDomainsEnabled)
	data.Primary = types.BoolValue(nameserverGroup.Primary)
	data.Groups, d = types.ListValueFrom(ctx, types.StringType, nameserverGroup.Groups)
	ret.Append(d...)
	data.Domains, d = types.ListValueFrom(ctx, types.StringType, nameserverGroup.Domains)
	ret.Append(d...)
	nameservers := make([]struct {
		IP     types.String `tfsdk:"ip"`
		NSType types.String `tfsdk:"ns_type"`
		Port   types.Int32  `tfsdk:"port"`
	}, len(nameserverGroup.Nameservers))
	for i, j := range nameserverGroup.Nameservers {
		nameservers[i] = struct {
			IP     types.String "tfsdk:\"ip\""
			NSType types.String "tfsdk:\"ns_type\""
			Port   types.Int32  "tfsdk:\"port\""
		}{
			IP:     types.StringValue(j.Ip),
			NSType: types.StringValue(string(j.NsType)),
			Port:   types.Int32Value(int32(j.Port)),
		}
	}
	data.Nameservers, d = types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{"ip": types.StringType, "ns_type": types.StringType, "port": types.Int32Type},
	}, nameservers)
	ret.Append(d...)
	return ret
}

func nameserverGroupTerraformToAPI(ctx context.Context, data *NameserverGroupModel) (api.NameserverGroupRequest, diag.Diagnostics) {
	var ret diag.Diagnostics
	nameserverGroupReq := api.NameserverGroupRequest{
		Name:                 data.Name.ValueString(),
		Description:          data.Description.ValueString(),
		Domains:              stringListDefault(ctx, data.Domains, []string{}),
		Enabled:              boolDefault(data.Enabled, true),
		Groups:               stringListDefault(ctx, data.Groups, []string{}),
		Primary:              boolDefault(data.Primary, len(data.Domains.Elements()) == 0),
		SearchDomainsEnabled: boolDefault(data.SearchDomainsEnabled, false),
		Nameservers:          make([]api.Nameserver, len(data.Nameservers.Elements())),
	}

	if nameserverGroupReq.SearchDomainsEnabled && nameserverGroupReq.Primary {
		ret.AddError("Invalid Value", "search_domains_enabled and primary cannot be both true")
		return nameserverGroupReq, ret
	}

	if len(nameserverGroupReq.Domains) != 0 && nameserverGroupReq.Primary {
		ret.AddError("Invalid Value", "nameserver group primary status is true and domains are not empty, you should set either primary or domain")
		return nameserverGroupReq, ret
	}

	if len(nameserverGroupReq.Domains) == 0 && !nameserverGroupReq.Primary {
		ret.AddError("Invalid Value", "nameserver group primary status is false and domains are empty, it should be primary or have at least one domain")
		return nameserverGroupReq, ret
	}

	for i, j := range data.Nameservers.Elements() {
		jObj, ok := j.(types.Object)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Nameservers[%d] expected to be types.Object, found %T", i, j))
			return nameserverGroupReq, ret
		}
		jIP, ok := jObj.Attributes()["ip"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Nameservers[%d].ip expected to be types.String, found %T", i, jObj.Attributes()["ip"]))
			return nameserverGroupReq, ret
		}
		jPort, ok := jObj.Attributes()["port"].(types.Int32)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Nameservers[%d].port expected to be types.Int32, found %T", i, jObj.Attributes()["port"]))
			return nameserverGroupReq, ret
		}
		jNSType, ok := jObj.Attributes()["ns_type"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Nameservers[%d].ns_type expected to be types.String, found %T", i, jObj.Attributes()["ns_type"]))
			return nameserverGroupReq, ret
		}
		nameserverGroupReq.Nameservers[i] = api.Nameserver{
			Ip:     jIP.ValueString(),
			NsType: api.NameserverNsType(jNSType.ValueString()),
			Port:   int(jPort.ValueInt32()),
		}
	}

	return nameserverGroupReq, ret
}

func (r *NameserverGroup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NameserverGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	nameserverGroupReq, d := nameserverGroupTerraformToAPI(ctx, &data)
	resp.Diagnostics = append(resp.Diagnostics, d...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameserverGroup, err := r.client.DNS.CreateNameserverGroup(ctx, nameserverGroupReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating nameserverGroup", err.Error())
		return
	}

	resp.Diagnostics.Append(nameserverGroupAPIToTerraform(ctx, nameserverGroup, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NameserverGroup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NameserverGroupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	nameserverGroup, err := r.client.DNS.GetNameserverGroup(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting NameserverGroup", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(nameserverGroupAPIToTerraform(ctx, nameserverGroup, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NameserverGroup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NameserverGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	nameserverGroupReq, d := nameserverGroupTerraformToAPI(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameserverGroup, err := r.client.DNS.UpdateNameserverGroup(ctx, data.Id.ValueString(), nameserverGroupReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating NameserverGroup", err.Error())
		return
	}

	resp.Diagnostics.Append(nameserverGroupAPIToTerraform(ctx, nameserverGroup, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NameserverGroup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NameserverGroupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DNS.DeleteNameserverGroup(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting NameserverGroup", err.Error())
	}
}

func (r *NameserverGroup) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
