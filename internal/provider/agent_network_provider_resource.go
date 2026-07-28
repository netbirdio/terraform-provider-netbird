// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var _ resource.Resource = &AgentNetworkProvider{}
var _ resource.ResourceWithImportState = &AgentNetworkProvider{}

func NewAgentNetworkProvider() resource.Resource {
	return &AgentNetworkProvider{}
}

type AgentNetworkProvider struct {
	client *agentNetworkClient
}

type AgentNetworkProviderModel struct {
	Id                   types.String `tfsdk:"id"`
	ProviderId           types.String `tfsdk:"provider_id"`
	Name                 types.String `tfsdk:"name"`
	UpstreamUrl          types.String `tfsdk:"upstream_url"`
	ApiKey               types.String `tfsdk:"api_key"`
	BootstrapCluster     types.String `tfsdk:"bootstrap_cluster"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	SkipTlsVerification  types.Bool   `tfsdk:"skip_tls_verification"`
	IdentityHeaderUserId types.String `tfsdk:"identity_header_user_id"`
	IdentityHeaderGroups types.String `tfsdk:"identity_header_groups"`
	MetadataDisabled     types.Bool   `tfsdk:"metadata_disabled"`
	ExtraValues          types.Map    `tfsdk:"extra_values"`
	Models               types.List   `tfsdk:"models"`
}

type AgentNetworkProviderModelItem struct {
	Id          types.String  `tfsdk:"id"`
	InputPer1k  types.Float64 `tfsdk:"input_per_1k"`
	OutputPer1k types.Float64 `tfsdk:"output_per_1k"`
}

func (AgentNetworkProviderModelItem) TFType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"id":            types.StringType,
		"input_per_1k":  types.Float64Type,
		"output_per_1k": types.Float64Type,
	}}
}

func (r *AgentNetworkProvider) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_provider"
}

func (r *AgentNetworkProvider) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Agent Network providers (upstream AI API endpoints), see [NetBird Docs](https://docs.netbird.io/api/resources/agent-network) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Provider ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "Catalog identifier for the upstream AI provider (e.g. `openai_api`, `anthropic_api`, `custom`)",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name shown in the dashboard",
				Required:            true,
			},
			"upstream_url": schema.StringAttribute{
				MarkdownDescription: "Full upstream URL (with scheme) that NetBird forwards traffic to",
				Required:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Upstream provider API key. Sealed at rest; never returned in responses. Required on create; omit on update to keep the existing key.",
				Required:            true,
				Sensitive:           true,
			},
			"bootstrap_cluster": schema.StringAttribute{
				MarkdownDescription: "Proxy cluster used when creating the first provider. Ignored on subsequent creates and all updates.",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the provider is enabled",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"skip_tls_verification": schema.BoolAttribute{
				MarkdownDescription: "Skip upstream TLS certificate verification (for self-hosted gateways with private certificates)",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"identity_header_user_id": schema.StringAttribute{
				MarkdownDescription: "Wire header name the proxy stamps with the caller's display identity",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"identity_header_groups": schema.StringAttribute{
				MarkdownDescription: "Wire header name the proxy stamps with the caller's NetBird groups as a comma-separated list",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"metadata_disabled": schema.BoolAttribute{
				MarkdownDescription: "Suppress identity-metadata injection for this provider (e.g. the AWS Bedrock request-metadata header). Omit to leave the stored value unchanged.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"extra_values": schema.MapAttribute{
				MarkdownDescription: "Catalog-specific extra header values (e.g. `x-portkey-config` for Portkey gateways). Omit to leave the stored values unchanged. Empty values are dropped by the API.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"models": schema.ListNestedAttribute{
				MarkdownDescription: "Models exposed through this endpoint with per-1k token prices. Empty means all catalog models at catalog prices.",
				Optional:            true,
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Model identifier (e.g. `gpt-4o-mini`)",
							Required:            true,
						},
						"input_per_1k": schema.Float64Attribute{
							MarkdownDescription: "Cost per 1k input tokens in USD",
							Required:            true,
						},
						"output_per_1k": schema.Float64Attribute{
							MarkdownDescription: "Cost per 1k output tokens in USD",
							Required:            true,
						},
					},
				},
			},
		},
	}
}

func (r *AgentNetworkProvider) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*netbird.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = newAgentNetworkClient(client)
}

// preserveConfiguredEmptyString keeps an explicitly-configured empty string
// intact when the API omits the field. The identity-header fields treat "" as
// "disable stamping for this dimension", but the server omits them from
// responses when empty, which would turn a configured "" into null and trip
// Terraform's "provider produced inconsistent result after apply" check.
func preserveConfiguredEmptyString(apiVal *string, configured types.String) types.String {
	if apiVal == nil && !configured.IsNull() && !configured.IsUnknown() && configured.ValueString() == "" {
		return types.StringValue("")
	}
	return types.StringPointerValue(apiVal)
}

func agentNetworkProviderAPIToTerraform(ctx context.Context, p *api.AgentNetworkProvider, data *AgentNetworkProviderModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(p.Id)
	data.ProviderId = types.StringValue(p.ProviderId)
	data.Name = types.StringValue(p.Name)
	data.UpstreamUrl = types.StringValue(p.UpstreamUrl)
	data.Enabled = types.BoolValue(p.Enabled)
	data.SkipTlsVerification = types.BoolValue(p.SkipTlsVerification)
	data.IdentityHeaderUserId = preserveConfiguredEmptyString(p.IdentityHeaderUserId, data.IdentityHeaderUserId)
	data.IdentityHeaderGroups = preserveConfiguredEmptyString(p.IdentityHeaderGroups, data.IdentityHeaderGroups)
	data.MetadataDisabled = types.BoolValue(p.MetadataDisabled)

	if p.ExtraValues != nil {
		m, d := types.MapValueFrom(ctx, types.StringType, *p.ExtraValues)
		ret.Append(d...)
		data.ExtraValues = m
	} else {
		data.ExtraValues = types.MapNull(types.StringType)
	}

	modelObjs := make([]AgentNetworkProviderModelItem, 0, len(p.Models))
	for _, m := range p.Models {
		modelObjs = append(modelObjs, AgentNetworkProviderModelItem{
			Id:          types.StringValue(m.Id),
			InputPer1k:  types.Float64Value(m.InputPer1k),
			OutputPer1k: types.Float64Value(m.OutputPer1k),
		})
	}
	list, d := types.ListValueFrom(ctx, AgentNetworkProviderModelItem{}.TFType(), modelObjs)
	ret.Append(d...)
	data.Models = list
	return ret
}

func agentNetworkProviderTerraformToRequest(ctx context.Context, data *AgentNetworkProviderModel) (api.AgentNetworkProviderRequest, diag.Diagnostics) {
	var ret diag.Diagnostics
	req := api.AgentNetworkProviderRequest{
		ProviderId:          data.ProviderId.ValueString(),
		Name:                data.Name.ValueString(),
		UpstreamUrl:         data.UpstreamUrl.ValueString(),
		Enabled:             data.Enabled.ValueBoolPointer(),
		SkipTlsVerification: data.SkipTlsVerification.ValueBoolPointer(),
	}
	if !data.ApiKey.IsNull() && !data.ApiKey.IsUnknown() {
		req.ApiKey = data.ApiKey.ValueStringPointer()
	}
	if !data.BootstrapCluster.IsNull() && !data.BootstrapCluster.IsUnknown() {
		req.BootstrapCluster = data.BootstrapCluster.ValueStringPointer()
	}
	if !data.IdentityHeaderUserId.IsNull() && !data.IdentityHeaderUserId.IsUnknown() {
		req.IdentityHeaderUserId = data.IdentityHeaderUserId.ValueStringPointer()
	}
	if !data.IdentityHeaderGroups.IsNull() && !data.IdentityHeaderGroups.IsUnknown() {
		req.IdentityHeaderGroups = data.IdentityHeaderGroups.ValueStringPointer()
	}
	// The server rebuilds the provider row from the request on update, carrying
	// over only the API key and session keys. Anything omitted here is reset, so
	// these must be sent whenever their value is known (UseStateForUnknown makes
	// the prior value known on update) or they would be silently wiped.
	if !data.MetadataDisabled.IsNull() && !data.MetadataDisabled.IsUnknown() {
		req.MetadataDisabled = data.MetadataDisabled.ValueBoolPointer()
	}
	if !data.ExtraValues.IsNull() && !data.ExtraValues.IsUnknown() {
		extra := make(map[string]string, len(data.ExtraValues.Elements()))
		ret.Append(data.ExtraValues.ElementsAs(ctx, &extra, false)...)
		if !ret.HasError() {
			req.ExtraValues = &extra
		}
	}
	if !data.Models.IsNull() && !data.Models.IsUnknown() {
		var elems []AgentNetworkProviderModelItem
		ret.Append(data.Models.ElementsAs(ctx, &elems, false)...)
		if !ret.HasError() {
			models := make([]api.AgentNetworkProviderModel, 0, len(elems))
			for _, e := range elems {
				models = append(models, api.AgentNetworkProviderModel{
					Id:          e.Id.ValueString(),
					InputPer1k:  e.InputPer1k.ValueFloat64(),
					OutputPer1k: e.OutputPer1k.ValueFloat64(),
				})
			}
			req.Models = &models
		}
	}
	return req, ret
}

func (r *AgentNetworkProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentNetworkProviderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, d := agentNetworkProviderTerraformToRequest(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreateProvider(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Agent Network Provider", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkProviderAPIToTerraform(ctx, p, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentNetworkProviderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetProvider(ctx, data.Id.ValueString())
	if err != nil {
		if netbird.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Agent Network Provider", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkProviderAPIToTerraform(ctx, p, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentNetworkProviderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, d := agentNetworkProviderTerraformToRequest(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.UpdateProvider(ctx, data.Id.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Agent Network Provider", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkProviderAPIToTerraform(ctx, p, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentNetworkProviderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteProvider(ctx, data.Id.ValueString()); err != nil && !netbird.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Agent Network Provider", err.Error())
	}
}

func (r *AgentNetworkProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
