// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var _ datasource.DataSource = &AgentNetworkGuardrailDataSource{}

func NewAgentNetworkGuardrailDataSource() datasource.DataSource {
	return &AgentNetworkGuardrailDataSource{}
}

type AgentNetworkGuardrailDataSource struct {
	client *agentNetworkClient
}

func (d *AgentNetworkGuardrailDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_guardrail"
}

func (d *AgentNetworkGuardrailDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an Agent Network guardrail by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Guardrail ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the guardrail",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional human-readable description",
				Computed:            true,
			},
			"model_allowlist": schema.SingleNestedAttribute{
				MarkdownDescription: "Model allowlist check",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{Computed: true},
					"models": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
					},
				},
			},
			"prompt_capture": schema.SingleNestedAttribute{
				MarkdownDescription: "Prompt capture settings",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled":    schema.BoolAttribute{Computed: true},
					"redact_pii": schema.BoolAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *AgentNetworkGuardrailDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*netbird.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = newAgentNetworkClient(client)
}

func (d *AgentNetworkGuardrailDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentNetworkGuardrailModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must set at least one of (id, name)")
		return
	}

	// A known id identifies the record uniquely, so fetch it directly instead of
	// listing everything and scanning. A name filter given alongside must agree.
	if !data.Id.IsNull() && !data.Id.IsUnknown() {
		found, err := d.client.GetGuardrail(ctx, data.Id.ValueString())
		if err != nil {
			if netbird.IsNotFound(err) {
				resp.Diagnostics.AddError("No Match", "No Agent Network Guardrail matched the given filters")
				return
			}
			resp.Diagnostics.AddError("Error reading Agent Network Guardrail", err.Error())
			return
		}
		if matchString(found.Name, data.Name) < 0 {
			resp.Diagnostics.AddError("No Match", "No Agent Network Guardrail matched the given filters")
			return
		}
		resp.Diagnostics.Append(agentNetworkGuardrailAPIToTerraform(ctx, found, &data)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	guardrails, err := d.client.ListGuardrails(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Agent Network Guardrails", err.Error())
		return
	}

	var found *api.AgentNetworkGuardrail
	for _, g := range guardrails {
		match := 0
		match += matchString(g.Id, data.Id)
		match += matchString(g.Name, data.Name)
		if match > 0 {
			if found != nil {
				resp.Diagnostics.AddError("Multiple Matches", "Data source matched more than one Agent Network Guardrail")
			}
			found = &g
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if found == nil {
		resp.Diagnostics.AddError("No Match", "No Agent Network Guardrail matched the given filters")
		return
	}

	resp.Diagnostics.Append(agentNetworkGuardrailAPIToTerraform(ctx, found, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
