// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var _ resource.Resource = &AgentNetworkSettings{}

func NewAgentNetworkSettings() resource.Resource {
	return &AgentNetworkSettings{}
}

type AgentNetworkSettings struct {
	client *agentNetworkClient
}

// AgentNetworkSettingsModel mirrors the mutable + read-only fields of AgentNetworkSettings.
type AgentNetworkSettingsModel struct {
	// computed / read-only
	Cluster                types.String `tfsdk:"cluster"`
	Subdomain              types.String `tfsdk:"subdomain"`
	Endpoint               types.String `tfsdk:"endpoint"`
	// mutable
	EnableLogCollection    types.Bool   `tfsdk:"enable_log_collection"`
	EnablePromptCollection types.Bool   `tfsdk:"enable_prompt_collection"`
	RedactPii              types.Bool   `tfsdk:"redact_pii"`
	AccessLogRetentionDays types.Int64  `tfsdk:"access_log_retention_days"`
}

func (r *AgentNetworkSettings) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_settings"
}

func (r *AgentNetworkSettings) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage account-level Agent Network gateway settings (log collection, prompt capture, PII redaction). Cluster and subdomain are auto-assigned and immutable.",
		Attributes: map[string]schema.Attribute{
			"cluster": schema.StringAttribute{
				MarkdownDescription: "Proxy cluster address fronting this account's agent-network endpoint (read-only, assigned on first provider create)",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"subdomain": schema.StringAttribute{
				MarkdownDescription: "DNS-safe subdomain prefix (read-only)",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Full agent-network endpoint hostname (`<subdomain>.<cluster>`), read-only",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enable_log_collection": schema.BoolAttribute{
				MarkdownDescription: "Collect per-request access-log entries for this account",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"enable_prompt_collection": schema.BoolAttribute{
				MarkdownDescription: "Master switch for request/response prompt capture (effective only when a policy guardrail also enables it)",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"redact_pii": schema.BoolAttribute{
				MarkdownDescription: "Redact PII from captured prompts",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"access_log_retention_days": schema.Int64Attribute{
				MarkdownDescription: "Days to retain full access-log rows (0 or less = keep indefinitely)",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *AgentNetworkSettings) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func agentNetworkSettingsAPIToTerraform(_ context.Context, s *api.AgentNetworkSettings, data *AgentNetworkSettingsModel) diag.Diagnostics {
	data.Cluster = types.StringValue(s.Cluster)
	data.Subdomain = types.StringValue(s.Subdomain)
	data.Endpoint = types.StringValue(s.Endpoint)
	data.EnableLogCollection = types.BoolValue(s.EnableLogCollection)
	data.EnablePromptCollection = types.BoolValue(s.EnablePromptCollection)
	data.RedactPii = types.BoolValue(s.RedactPii)
	if s.AccessLogRetentionDays != nil {
		data.AccessLogRetentionDays = types.Int64Value(int64(*s.AccessLogRetentionDays))
	} else {
		data.AccessLogRetentionDays = types.Int64Value(0)
	}
	return nil
}

// Create reads the current settings and stores them (singleton — no actual create).
func (r *AgentNetworkSettings) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentNetworkSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	retentionDays := int(data.AccessLogRetentionDays.ValueInt64())
	apiReq := api.AgentNetworkSettingsRequest{
		EnableLogCollection:    data.EnableLogCollection.ValueBool(),
		EnablePromptCollection: data.EnablePromptCollection.ValueBool(),
		RedactPii:              data.RedactPii.ValueBool(),
		AccessLogRetentionDays: &retentionDays,
	}
	s, err := r.client.UpdateSettings(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error applying Agent Network Settings", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkSettingsAPIToTerraform(ctx, s, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkSettings) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AgentNetworkSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetSettings(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Agent Network Settings", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkSettingsAPIToTerraform(ctx, s, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkSettings) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentNetworkSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	retentionDays := int(data.AccessLogRetentionDays.ValueInt64())
	apiReq := api.AgentNetworkSettingsRequest{
		EnableLogCollection:    data.EnableLogCollection.ValueBool(),
		EnablePromptCollection: data.EnablePromptCollection.ValueBool(),
		RedactPii:              data.RedactPii.ValueBool(),
		AccessLogRetentionDays: &retentionDays,
	}
	s, err := r.client.UpdateSettings(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Agent Network Settings", err.Error())
		return
	}
	resp.Diagnostics.Append(agentNetworkSettingsAPIToTerraform(ctx, s, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete is a no-op — the settings row always exists, we just stop managing it.
func (r *AgentNetworkSettings) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
