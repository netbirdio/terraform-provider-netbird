// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var _ resource.Resource = &AgentNetworkPolicy{}
var _ resource.ResourceWithImportState = &AgentNetworkPolicy{}

func NewAgentNetworkPolicy() resource.Resource {
	return &AgentNetworkPolicy{}
}

type AgentNetworkPolicy struct {
	client *agentNetworkClient
}

type AgentNetworkPolicyModel struct {
	Id                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	SourceGroups           types.List   `tfsdk:"source_groups"`
	DestinationProviderIds types.List   `tfsdk:"destination_provider_ids"`
	GuardrailIds           types.List   `tfsdk:"guardrail_ids"`
	TokenLimit             types.Object `tfsdk:"token_limit"`
	BudgetLimit            types.Object `tfsdk:"budget_limit"`
}

var tokenLimitAttrTypes = map[string]attr.Type{
	"enabled":        types.BoolType,
	"group_cap":      types.Int64Type,
	"user_cap":       types.Int64Type,
	"window_seconds": types.Int64Type,
}

var budgetLimitAttrTypes = map[string]attr.Type{
	"enabled":        types.BoolType,
	"group_cap_usd":  types.Float64Type,
	"user_cap_usd":   types.Float64Type,
	"window_seconds": types.Int64Type,
}

func (r *AgentNetworkPolicy) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_policy"
}

func (r *AgentNetworkPolicy) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Agent Network policies (access control + spend/token limits), see [NetBird Docs](https://docs.netbird.io/api/resources/agent-network) for more information.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the policy",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional human-readable description",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy is enabled",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"source_groups": schema.ListAttribute{
				MarkdownDescription: "NetBird group IDs whose members may call the destination providers",
				Required:            true,
				ElementType:         types.StringType,
			},
			"destination_provider_ids": schema.ListAttribute{
				MarkdownDescription: "Agent Network provider IDs the source groups can reach",
				Required:            true,
				ElementType:         types.StringType,
			},
			"guardrail_ids": schema.ListAttribute{
				MarkdownDescription: "Agent Network guardrail IDs attached to this policy",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
			},
			"token_limit": schema.SingleNestedAttribute{
				MarkdownDescription: "Per-policy token cap",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the token limit is enforced",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"group_cap": schema.Int64Attribute{
						MarkdownDescription: "Tokens allowed per source group per window (0 = uncapped)",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(0),
					},
					"user_cap": schema.Int64Attribute{
						MarkdownDescription: "Tokens allowed per individual user per window (0 = uncapped)",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(0),
					},
					"window_seconds": schema.Int64Attribute{
						MarkdownDescription: "Reset frequency in seconds (minimum 60 when enabled)",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(2592000),
					},
				},
			},
			"budget_limit": schema.SingleNestedAttribute{
				MarkdownDescription: "Per-policy USD spend cap",
				Optional:            true,
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the budget limit is enforced",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"group_cap_usd": schema.Float64Attribute{
						MarkdownDescription: "USD allowed per source group per window (0 = uncapped)",
						Optional:            true,
						Computed:            true,
					},
					"user_cap_usd": schema.Float64Attribute{
						MarkdownDescription: "USD allowed per user per window (0 = uncapped)",
						Optional:            true,
						Computed:            true,
					},
					"window_seconds": schema.Int64Attribute{
						MarkdownDescription: "Reset frequency in seconds (minimum 60 when enabled)",
						Optional:            true,
						Computed:            true,
						Default:             int64default.StaticInt64(2592000),
					},
				},
			},
		},
	}
}

func (r *AgentNetworkPolicy) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func agentNetworkPolicyAPIToTerraform(ctx context.Context, p *api.AgentNetworkPolicy, data *AgentNetworkPolicyModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(p.Id)
	data.Name = types.StringValue(p.Name)
	data.Description = types.StringValue(p.Description)
	data.Enabled = types.BoolValue(p.Enabled)

	data.SourceGroups, d = types.ListValueFrom(ctx, types.StringType, p.SourceGroups)
	ret.Append(d...)
	data.DestinationProviderIds, d = types.ListValueFrom(ctx, types.StringType, p.DestinationProviderIds)
	ret.Append(d...)
	data.GuardrailIds, d = types.ListValueFrom(ctx, types.StringType, p.GuardrailIds)
	ret.Append(d...)

	tl := p.Limits.TokenLimit
	data.TokenLimit, d = types.ObjectValue(tokenLimitAttrTypes, map[string]attr.Value{
		"enabled":        types.BoolValue(tl.Enabled),
		"group_cap":      types.Int64Value(tl.GroupCap),
		"user_cap":       types.Int64Value(tl.UserCap),
		"window_seconds": types.Int64Value(tl.WindowSeconds),
	})
	ret.Append(d...)

	bl := p.Limits.BudgetLimit
	data.BudgetLimit, d = types.ObjectValue(budgetLimitAttrTypes, map[string]attr.Value{
		"enabled":        types.BoolValue(bl.Enabled),
		"group_cap_usd":  types.Float64Value(bl.GroupCapUsd),
		"user_cap_usd":   types.Float64Value(bl.UserCapUsd),
		"window_seconds": types.Int64Value(bl.WindowSeconds),
	})
	ret.Append(d...)
	return ret
}

type tokenLimitElem struct {
	Enabled       bool  `tfsdk:"enabled"`
	GroupCap      int64 `tfsdk:"group_cap"`
	UserCap       int64 `tfsdk:"user_cap"`
	WindowSeconds int64 `tfsdk:"window_seconds"`
}

type budgetLimitElem struct {
	Enabled       bool    `tfsdk:"enabled"`
	GroupCapUsd   float64 `tfsdk:"group_cap_usd"`
	UserCapUsd    float64 `tfsdk:"user_cap_usd"`
	WindowSeconds int64   `tfsdk:"window_seconds"`
}

func agentNetworkPolicyTerraformToRequest(ctx context.Context, data *AgentNetworkPolicyModel) (api.AgentNetworkPolicyRequest, diag.Diagnostics) {
	var ret diag.Diagnostics
	req := api.AgentNetworkPolicyRequest{
		Name:    data.Name.ValueString(),
		Enabled: data.Enabled.ValueBoolPointer(),
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		req.Description = data.Description.ValueStringPointer()
	}

	var srcGroups []string
	ret.Append(data.SourceGroups.ElementsAs(ctx, &srcGroups, false)...)
	req.SourceGroups = srcGroups

	var destProviders []string
	ret.Append(data.DestinationProviderIds.ElementsAs(ctx, &destProviders, false)...)
	req.DestinationProviderIds = destProviders

	if !data.GuardrailIds.IsNull() && !data.GuardrailIds.IsUnknown() {
		var gids []string
		ret.Append(data.GuardrailIds.ElementsAs(ctx, &gids, false)...)
		req.GuardrailIds = &gids
	}

	limits := api.AgentNetworkPolicyLimits{}
	if !data.TokenLimit.IsNull() && !data.TokenLimit.IsUnknown() {
		var tl tokenLimitElem
		ret.Append(data.TokenLimit.As(ctx, &tl, basetypes.ObjectAsOptions{})...)
		limits.TokenLimit = api.AgentNetworkPolicyTokenLimit{
			Enabled:       tl.Enabled,
			GroupCap:      tl.GroupCap,
			UserCap:       tl.UserCap,
			WindowSeconds: tl.WindowSeconds,
		}
	}
	if !data.BudgetLimit.IsNull() && !data.BudgetLimit.IsUnknown() {
		var bl budgetLimitElem
		ret.Append(data.BudgetLimit.As(ctx, &bl, basetypes.ObjectAsOptions{})...)
		limits.BudgetLimit = api.AgentNetworkPolicyBudgetLimit{
			Enabled:       bl.Enabled,
			GroupCapUsd:   bl.GroupCapUsd,
			UserCapUsd:    bl.UserCapUsd,
			WindowSeconds: bl.WindowSeconds,
		}
	}
	req.Limits = &limits
	return req, ret
}

func (r *AgentNetworkPolicy) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentNetworkPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq, d := agentNetworkPolicyTerraformToRequest(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.CreatePolicy(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Agent Network Policy", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkPolicyAPIToTerraform(ctx, p, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkPolicy) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentNetworkPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.GetPolicy(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Agent Network Policy", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkPolicyAPIToTerraform(ctx, p, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkPolicy) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentNetworkPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}
	apiReq, d := agentNetworkPolicyTerraformToRequest(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	p, err := r.client.UpdatePolicy(ctx, data.Id.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Agent Network Policy", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkPolicyAPIToTerraform(ctx, p, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkPolicy) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AgentNetworkPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePolicy(ctx, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting Agent Network Policy", err.Error())
	}
}

func (r *AgentNetworkPolicy) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
