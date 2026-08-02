// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
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
	client *netbird.AgentNetworkAPI
}

// AgentNetworkSettingsModel mirrors the mutable + read-only fields of AgentNetworkSettings.
type AgentNetworkSettingsModel struct {
	// computed / read-only
	Cluster   types.String `tfsdk:"cluster"`
	Subdomain types.String `tfsdk:"subdomain"`
	Endpoint  types.String `tfsdk:"endpoint"`
	// mutable
	EnableLogCollection    types.Bool  `tfsdk:"enable_log_collection"`
	EnablePromptCollection types.Bool  `tfsdk:"enable_prompt_collection"`
	RedactPii              types.Bool  `tfsdk:"redact_pii"`
	AccessLogRetentionDays types.Int64 `tfsdk:"access_log_retention_days"`
}

func (r *AgentNetworkSettings) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_settings"
}

func (r *AgentNetworkSettings) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage account-level Agent Network gateway settings (log collection, prompt capture, PII redaction). Setting `cluster` bootstraps the account when it has no settings row yet; cluster and subdomain are immutable once assigned.",
		Attributes: map[string]schema.Attribute{
			"cluster": schema.StringAttribute{
				MarkdownDescription: "Proxy cluster address fronting this account's agent-network endpoint. Set it to bootstrap the account when it has no Agent Network settings row yet — the `netbird_reverse_proxy_clusters` data source lists valid addresses. Immutable once assigned; omit to adopt the cluster assigned when a provider was created with `bootstrap_cluster`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
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
				MarkdownDescription: "Collect per-request access-log entries for this account. Omit to leave the account's current value unchanged.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"enable_prompt_collection": schema.BoolAttribute{
				MarkdownDescription: "Master switch for request/response prompt capture (effective only when a policy guardrail also enables it). Omit to leave the account's current value unchanged.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"redact_pii": schema.BoolAttribute{
				MarkdownDescription: "Redact PII from captured prompts. Omit to leave the account's current value unchanged.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"access_log_retention_days": schema.Int64Attribute{
				MarkdownDescription: "Days to retain full access-log rows (0 or less = keep indefinitely). Omit to leave the account's current value unchanged.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
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
	r.client = client.AgentNetwork
}

func agentNetworkSettingsAPIToTerraform(s *api.AgentNetworkSettings, data *AgentNetworkSettingsModel) {
	data.Cluster = types.StringValue(s.Cluster)
	data.Subdomain = types.StringValue(s.Subdomain)
	data.Endpoint = types.StringValue(s.Endpoint)
	data.EnableLogCollection = types.BoolValue(s.EnableLogCollection)
	data.EnablePromptCollection = types.BoolValue(s.EnablePromptCollection)
	data.RedactPii = types.BoolValue(s.RedactPii)
	if s.AccessLogRetentionDays != nil {
		data.AccessLogRetentionDays = types.Int64Value(int64(*s.AccessLogRetentionDays))
	} else {
		data.AccessLogRetentionDays = types.Int64Null()
	}
}

// settingsUpdateRequest builds an UpdateSettings request that starts from the
// current server-side values and overrides only the fields the plan explicitly
// set. The server replaces every mutable field on PUT, so without this merge an
// omitted attribute would reset the account's value (turning off log/prompt
// collection, or dropping access-log retention) instead of leaving it alone.
func settingsUpdateRequest(data *AgentNetworkSettingsModel, current *api.AgentNetworkSettings) api.AgentNetworkSettingsRequest {
	req := api.AgentNetworkSettingsRequest{
		EnableLogCollection:    current.EnableLogCollection,
		EnablePromptCollection: current.EnablePromptCollection,
		RedactPii:              current.RedactPii,
	}
	if current.AccessLogRetentionDays != nil {
		v := *current.AccessLogRetentionDays
		req.AccessLogRetentionDays = &v
	}

	if !data.EnableLogCollection.IsNull() && !data.EnableLogCollection.IsUnknown() {
		req.EnableLogCollection = data.EnableLogCollection.ValueBool()
	}
	if !data.EnablePromptCollection.IsNull() && !data.EnablePromptCollection.IsUnknown() {
		req.EnablePromptCollection = data.EnablePromptCollection.ValueBool()
	}
	if !data.RedactPii.IsNull() && !data.RedactPii.IsUnknown() {
		req.RedactPii = data.RedactPii.ValueBool()
	}
	if !data.AccessLogRetentionDays.IsNull() && !data.AccessLogRetentionDays.IsUnknown() {
		v := int(data.AccessLogRetentionDays.ValueInt64())
		req.AccessLogRetentionDays = &v
	}
	return req
}

// applySettings reads the current server-managed singleton, merges the planned
// overrides onto it, and PUTs the result. When the account has no settings
// row yet, a configured cluster bootstraps it in the same call.
func (r *AgentNetworkSettings) applySettings(ctx context.Context, data *AgentNetworkSettingsModel, diags *diag.Diagnostics) {
	current, err := r.client.GetSettings(ctx)
	if err != nil {
		if !netbird.IsNotFound(err) {
			diags.AddError("Error reading Agent Network Settings", err.Error())
			return
		}
		// Management servers predating the defaults response answer the
		// unbootstrapped state with a null body (translated to not-found by
		// the client); fall back to the same defaults those servers apply at
		// bootstrap so the merge below has a base either way.
		retention := 30
		current = &api.AgentNetworkSettings{EnableLogCollection: true, AccessLogRetentionDays: &retention}
	}

	apiReq := settingsUpdateRequest(data, current)

	clusterConfigured := !data.Cluster.IsNull() && !data.Cluster.IsUnknown() && data.Cluster.ValueString() != ""
	if clusterConfigured {
		apiReq.Cluster = data.Cluster.ValueStringPointer()
	}
	// An empty endpoint is the server's not-bootstrapped signal.
	if current.Endpoint == "" && !clusterConfigured {
		diags.AddError(
			"Agent Network not bootstrapped",
			"The account has no Agent Network settings row yet. Set `cluster` on this resource to bootstrap it — "+
				"the `netbird_reverse_proxy_clusters` data source lists valid cluster addresses — or create an "+
				"Agent Network provider with `bootstrap_cluster` set and re-apply.",
		)
		return
	}

	s, err := r.client.UpdateSettings(ctx, apiReq)
	if err != nil {
		diags.AddError("Error applying Agent Network Settings", err.Error())
		return
	}

	agentNetworkSettingsAPIToTerraform(s, data)
}

// Create adopts the server-managed settings singleton, overriding only the
// fields the configuration sets (singleton — no actual create).
func (r *AgentNetworkSettings) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AgentNetworkSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applySettings(ctx, &data, &resp.Diagnostics)
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
		if netbird.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Agent Network Settings", err.Error())
		return
	}
	// An empty endpoint is the server's not-bootstrapped signal: the row this
	// resource manages does not exist (yet), so drop it from state.
	if s.Endpoint == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	agentNetworkSettingsAPIToTerraform(s, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AgentNetworkSettings) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AgentNetworkSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applySettings(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete is a no-op — the settings row always exists, we just stop managing it.
func (r *AgentNetworkSettings) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
