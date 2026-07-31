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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var _ resource.Resource = &AgentNetworkGuardrail{}
var _ resource.ResourceWithImportState = &AgentNetworkGuardrail{}

func NewAgentNetworkGuardrail() resource.Resource {
	return &AgentNetworkGuardrail{}
}

type AgentNetworkGuardrail struct {
	client *agentNetworkClient
}

type AgentNetworkGuardrailModel struct {
	Id             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	ModelAllowlist types.Object `tfsdk:"model_allowlist"`
	PromptCapture  types.Object `tfsdk:"prompt_capture"`
}

type AgentNetworkModelAllowlistModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
	Models  types.List `tfsdk:"models"`
}

func (AgentNetworkModelAllowlistModel) TFType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"enabled": types.BoolType,
		"models":  types.ListType{ElemType: types.StringType},
	}}
}

type AgentNetworkPromptCaptureModel struct {
	Enabled   types.Bool `tfsdk:"enabled"`
	RedactPii types.Bool `tfsdk:"redact_pii"`
}

func (AgentNetworkPromptCaptureModel) TFType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"enabled":    types.BoolType,
		"redact_pii": types.BoolType,
	}}
}

func (r *AgentNetworkGuardrail) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_guardrail"
}

func (r *AgentNetworkGuardrail) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Agent Network guardrails (model allowlists and prompt capture), see [NetBird Docs](https://docs.netbird.io/api/resources/agent-network) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Guardrail ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the guardrail",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional human-readable description",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"model_allowlist": schema.SingleNestedAttribute{
				MarkdownDescription: "Restrict which catalog models are allowed",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the model allowlist check is active",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"models": schema.ListAttribute{
						MarkdownDescription: "Allowed catalog model IDs",
						Required:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"prompt_capture": schema.SingleNestedAttribute{
				MarkdownDescription: "Request/response prompt capture settings",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether prompt capture is enabled for this guardrail",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"redact_pii": schema.BoolAttribute{
						MarkdownDescription: "Whether captured prompts have PII redacted",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
		},
	}
}

func (r *AgentNetworkGuardrail) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func agentNetworkGuardrailAPIToTerraform(ctx context.Context, g *api.AgentNetworkGuardrail, data *AgentNetworkGuardrailModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(g.Id)
	data.Name = types.StringValue(g.Name)
	data.Description = types.StringValue(g.Description)

	modelsList, d := types.ListValueFrom(ctx, types.StringType, g.Checks.ModelAllowlist.Models)
	ret.Append(d...)
	data.ModelAllowlist, d = types.ObjectValueFrom(ctx, AgentNetworkModelAllowlistModel{}.TFType().AttrTypes, AgentNetworkModelAllowlistModel{
		Enabled: types.BoolValue(g.Checks.ModelAllowlist.Enabled),
		Models:  modelsList,
	})
	ret.Append(d...)

	data.PromptCapture, d = types.ObjectValueFrom(ctx, AgentNetworkPromptCaptureModel{}.TFType().AttrTypes, AgentNetworkPromptCaptureModel{
		Enabled:   types.BoolValue(g.Checks.PromptCapture.Enabled),
		RedactPii: types.BoolValue(g.Checks.PromptCapture.RedactPii),
	})
	ret.Append(d...)
	return ret
}

func agentNetworkGuardrailTerraformToRequest(ctx context.Context, data *AgentNetworkGuardrailModel) (api.AgentNetworkGuardrailRequest, diag.Diagnostics) {
	var ret diag.Diagnostics
	req := api.AgentNetworkGuardrailRequest{
		Name: data.Name.ValueString(),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		req.Description = data.Description.ValueStringPointer()
	}

	req.Checks = api.AgentNetworkGuardrailChecks{}

	maAttrs := data.ModelAllowlist.Attributes()
	enabled, _ := maAttrs["enabled"].(types.Bool)
	req.Checks.ModelAllowlist.Enabled = enabled.ValueBool()
	if modelsList, ok := maAttrs["models"].(types.List); ok && !modelsList.IsNull() && !modelsList.IsUnknown() {
		ret.Append(modelsList.ElementsAs(ctx, &req.Checks.ModelAllowlist.Models, false)...)
	}

	pcAttrs := data.PromptCapture.Attributes()
	pcEnabled, _ := pcAttrs["enabled"].(types.Bool)
	redactPii, _ := pcAttrs["redact_pii"].(types.Bool)
	req.Checks.PromptCapture.Enabled = pcEnabled.ValueBool()
	req.Checks.PromptCapture.RedactPii = redactPii.ValueBool()

	return req, ret
}

func (r *AgentNetworkGuardrail) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentNetworkGuardrailModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, d := agentNetworkGuardrailTerraformToRequest(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.CreateGuardrail(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Agent Network Guardrail", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkGuardrailAPIToTerraform(ctx, g, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkGuardrail) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentNetworkGuardrailModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.GetGuardrail(ctx, data.Id.ValueString())
	if err != nil {
		if netbird.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Agent Network Guardrail", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkGuardrailAPIToTerraform(ctx, g, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkGuardrail) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentNetworkGuardrailModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, d := agentNetworkGuardrailTerraformToRequest(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.UpdateGuardrail(ctx, data.Id.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Agent Network Guardrail", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkGuardrailAPIToTerraform(ctx, g, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkGuardrail) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentNetworkGuardrailModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A guardrail already removed out-of-band is not an error: the desired end
	// state (gone) is satisfied, so let the destroy succeed.
	if err := r.client.DeleteGuardrail(ctx, data.Id.ValueString()); err != nil && !netbird.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Agent Network Guardrail", err.Error())
	}
}

func (r *AgentNetworkGuardrail) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
