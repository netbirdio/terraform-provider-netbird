// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var (
	_ resource.Resource = &AgentNetworkSettings{}
)

func NewAgentNetworkSettings() resource.Resource {
	return &AgentNetworkSettings{}
}

type AgentNetworkSettings struct {
	client *netbird.AgentNetworkAPI
}

// AgentNetworkSettingsModel mirrors the identity, mutable and read-only fields of
// AgentNetworkSettings.
type AgentNetworkSettingsModel struct {
	// identity — assigned at bootstrap, immutable afterwards
	Endpoint     types.String `tfsdk:"endpoint"`
	ProxyAddress types.String `tfsdk:"proxy_address"`
	Dedicated    types.Bool   `tfsdk:"dedicated"`
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
		MarkdownDescription: "Manage the account's Agent Network gateway: the endpoint agents call, and the collection " +
			"settings (log collection, prompt capture, PII redaction).\n\n" +
			"Set exactly one of `proxy_address` or `endpoint` to bootstrap an account that has none yet. " +
			"`proxy_address` allocates a hostname one label beneath a shared proxy cluster; `endpoint` claims a " +
			"hostname outright, served by a proxy dedicated to this account. Both are assigned once and immutable " +
			"afterwards, so changing either replaces the resource: the endpoint is released and a new one allocated. " +
			"Omit both to adopt whatever the account already has.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Hostname agents call for this account. Set it to claim that hostname outright, " +
					"which makes the gateway dedicated: only a proxy declaring exactly this address serves it, and it " +
					"is rejected when another account already holds it. Mutually exclusive with `proxy_address`. " +
					"Assigned by the server when `proxy_address` is used instead. Immutable once assigned: changing it " +
					"releases the current endpoint and allocates anew, which requires the account's Agent Network " +
					"providers to be destroyed first.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					// Only a configured change forces a replacement: once the
					// server has assigned an endpoint, state carries it and a
					// configuration that omits it must keep it.
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"proxy_address": schema.StringAttribute{
				MarkdownDescription: "Cluster address of the proxy serving this account's gateway. Set it to allocate an " +
					"endpoint one label beneath a shared cluster — the `netbird_reverse_proxy_clusters` data source lists " +
					"valid addresses. Mutually exclusive with `endpoint`, and equal to it when the gateway is dedicated. " +
					"Immutable once assigned, with the same replacement semantics as `endpoint`.",
				Optional:   true,
				Computed:   true,
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"dedicated": schema.BoolAttribute{
				MarkdownDescription: "Whether a proxy dedicated to this account serves the gateway, which is the case when " +
					"`endpoint` and `proxy_address` are the same address (read-only).",
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
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
	data.Endpoint = types.StringValue(s.Endpoint)
	data.ProxyAddress = types.StringValue(s.ProxyAddress)
	data.Dedicated = types.BoolValue(s.Dedicated)
	data.EnableLogCollection = types.BoolValue(s.EnableLogCollection)
	data.EnablePromptCollection = types.BoolValue(s.EnablePromptCollection)
	data.RedactPii = types.BoolValue(s.RedactPii)
	if s.AccessLogRetentionDays != nil {
		data.AccessLogRetentionDays = types.Int64Value(int64(*s.AccessLogRetentionDays))
	} else {
		data.AccessLogRetentionDays = types.Int64Null()
	}
}

// configured reports whether the configuration set a string attribute to a
// usable value, as opposed to omitting it or leaving it to be interpolated.
func configured(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}

// settingsUpdateRequest builds an UpdateSettings request that starts from the
// current server-side values and overrides only the fields the plan explicitly
// set. The server replaces every mutable field on PUT, so without this merge an
// omitted attribute would reset the account's value (turning off log/prompt
// collection, or dropping access-log retention) instead of leaving it alone.
//
// The endpoint and proxy address are echoed from what the server holds: the PUT
// requires them and rejects anything other than the assigned values.
func settingsUpdateRequest(data *AgentNetworkSettingsModel, current *api.AgentNetworkSettings) api.AgentNetworkSettingsRequest {
	req := api.AgentNetworkSettingsRequest{
		Endpoint:               current.Endpoint,
		ProxyAddress:           current.ProxyAddress,
		EnableLogCollection:    current.EnableLogCollection,
		EnablePromptCollection: current.EnablePromptCollection,
		RedactPii:              current.RedactPii,
	}
	if current.AccessLogRetentionDays != nil {
		req.AccessLogRetentionDays = *current.AccessLogRetentionDays
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
		req.AccessLogRetentionDays = int(data.AccessLogRetentionDays.ValueInt64())
	}
	return req
}

// settingsCreateRequest builds the bootstrap request. Only the collection fields
// the configuration set are sent, so the server applies its own defaults to the
// rest rather than this resource inventing them.
func settingsCreateRequest(data *AgentNetworkSettingsModel) api.AgentNetworkSettingsCreateRequest {
	req := api.AgentNetworkSettingsCreateRequest{}
	if configured(data.Endpoint) {
		req.Endpoint = data.Endpoint.ValueStringPointer()
	}
	if configured(data.ProxyAddress) {
		req.ProxyAddress = data.ProxyAddress.ValueStringPointer()
	}
	if !data.EnableLogCollection.IsNull() && !data.EnableLogCollection.IsUnknown() {
		req.EnableLogCollection = data.EnableLogCollection.ValueBoolPointer()
	}
	if !data.EnablePromptCollection.IsNull() && !data.EnablePromptCollection.IsUnknown() {
		req.EnablePromptCollection = data.EnablePromptCollection.ValueBoolPointer()
	}
	if !data.RedactPii.IsNull() && !data.RedactPii.IsUnknown() {
		req.RedactPii = data.RedactPii.ValueBoolPointer()
	}
	if !data.AccessLogRetentionDays.IsNull() && !data.AccessLogRetentionDays.IsUnknown() {
		v := int(data.AccessLogRetentionDays.ValueInt64())
		req.AccessLogRetentionDays = &v
	}
	return req
}

// applySettings bootstraps the account when it has no gateway yet, and otherwise
// updates the settings it already has.
//
// An empty endpoint on the read is the server's not-bootstrapped signal. In that
// state only the create endpoint works — the update refuses with not-found — so
// the two are not interchangeable.
func (r *AgentNetworkSettings) applySettings(ctx context.Context, data *AgentNetworkSettingsModel, diags *diag.Diagnostics) {
	current, err := r.client.GetSettings(ctx)
	switch {
	case err == nil && current.Endpoint != "":
		if configured(data.Endpoint) && data.Endpoint.ValueString() != current.Endpoint {
			diags.AddAttributeError(path.Root("endpoint"), "Endpoint is immutable once assigned",
				fmt.Sprintf("The account's gateway endpoint is %q. Changing it means releasing that endpoint and "+
					"allocating a new one, which this resource does by replacement; the account's Agent Network "+
					"providers have to be destroyed first.", current.Endpoint))
			return
		}
		if configured(data.ProxyAddress) && data.ProxyAddress.ValueString() != current.ProxyAddress {
			diags.AddAttributeError(path.Root("proxy_address"), "Proxy address is immutable once assigned",
				fmt.Sprintf("The account's gateway is served from %q. Changing it means releasing the endpoint and "+
					"allocating a new one, which this resource does by replacement; the account's Agent Network "+
					"providers have to be destroyed first.", current.ProxyAddress))
			return
		}

		updated, err := r.client.UpdateSettings(ctx, settingsUpdateRequest(data, current))
		if err != nil {
			diags.AddError("Error applying Agent Network Settings", err.Error())
			return
		}
		agentNetworkSettingsAPIToTerraform(updated, data)

	case err == nil || netbird.IsNotFound(err):
		// Not bootstrapped: exactly one of the two addresses picks the shape.
		if configured(data.Endpoint) == configured(data.ProxyAddress) {
			diags.AddError(
				"Agent Network not bootstrapped",
				"The account has no Agent Network gateway yet. Set exactly one of `proxy_address`, to allocate an "+
					"endpoint beneath a shared proxy cluster, or `endpoint`, to claim a hostname served by a proxy "+
					"dedicated to this account. The `netbird_reverse_proxy_clusters` data source lists cluster addresses.",
			)
			return
		}

		created, err := r.client.CreateSettings(ctx, settingsCreateRequest(data))
		if err != nil {
			diags.AddError("Error bootstrapping Agent Network Settings", err.Error())
			return
		}
		agentNetworkSettingsAPIToTerraform(created, data)

	default:
		diags.AddError("Error reading Agent Network Settings", err.Error())
	}
}

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
	// An empty endpoint is the server's not-bootstrapped signal: the gateway this
	// resource manages does not exist (any more), so drop it from state.
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

// Delete releases the account's gateway, which is what makes a change of
// endpoint or proxy address expressible as a replacement.
//
// The server guards the release: it refuses while the account still has Agent
// Network providers, and while a proxy is actively serving a dedicated endpoint.
// Those refusals are surfaced as they are — a gateway that is still in use is
// not something to delete quietly. An already-released gateway is not an error.
func (r *AgentNetworkSettings) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.client.DeleteSettings(ctx); err != nil && !netbird.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Agent Network Settings", err.Error())
	}
}
