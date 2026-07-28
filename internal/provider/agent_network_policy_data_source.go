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

var _ datasource.DataSource = &AgentNetworkPolicyDataSource{}

func NewAgentNetworkPolicyDataSource() datasource.DataSource {
	return &AgentNetworkPolicyDataSource{}
}

type AgentNetworkPolicyDataSource struct {
	client *agentNetworkClient
}

func (d *AgentNetworkPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_policy"
}

func (d *AgentNetworkPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an Agent Network policy by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the policy",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional human-readable description",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the policy is enabled",
				Computed:            true,
			},
			"source_groups": schema.ListAttribute{
				MarkdownDescription: "NetBird group IDs whose members may call the destination providers",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"destination_provider_ids": schema.ListAttribute{
				MarkdownDescription: "Agent Network provider IDs the source groups can reach",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"guardrail_ids": schema.ListAttribute{
				MarkdownDescription: "Agent Network guardrail IDs attached to this policy",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"token_limit": schema.SingleNestedAttribute{
				MarkdownDescription: "Per-policy token cap",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled":        schema.BoolAttribute{Computed: true},
					"group_cap":      schema.Int64Attribute{Computed: true},
					"user_cap":       schema.Int64Attribute{Computed: true},
					"window_seconds": schema.Int64Attribute{Computed: true},
				},
			},
			"budget_limit": schema.SingleNestedAttribute{
				MarkdownDescription: "Per-policy USD spend cap",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"enabled":        schema.BoolAttribute{Computed: true},
					"group_cap_usd":  schema.Float64Attribute{Computed: true},
					"user_cap_usd":   schema.Float64Attribute{Computed: true},
					"window_seconds": schema.Int64Attribute{Computed: true},
				},
			},
		},
	}
}

func (d *AgentNetworkPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AgentNetworkPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentNetworkPolicyModel

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
		found, err := d.client.GetPolicy(ctx, data.Id.ValueString())
		if err != nil {
			if netbird.IsNotFound(err) {
				resp.Diagnostics.AddError("No Match", "No Agent Network Policy matched the given filters")
				return
			}
			resp.Diagnostics.AddError("Error reading Agent Network Policy", err.Error())
			return
		}
		if matchString(found.Name, data.Name) < 0 {
			resp.Diagnostics.AddError("No Match", "No Agent Network Policy matched the given filters")
			return
		}
		resp.Diagnostics.Append(agentNetworkPolicyAPIToTerraform(ctx, found, &data)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	policies, err := d.client.ListPolicies(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Agent Network Policies", err.Error())
		return
	}

	var found *api.AgentNetworkPolicy
	for _, p := range policies {
		match := 0
		match += matchString(p.Id, data.Id)
		match += matchString(p.Name, data.Name)
		if match > 0 {
			if found != nil {
				resp.Diagnostics.AddError("Multiple Matches", "Data source matched more than one Agent Network Policy")
			}
			found = &p
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if found == nil {
		resp.Diagnostics.AddError("No Match", "No Agent Network Policy matched the given filters")
		return
	}

	resp.Diagnostics.Append(agentNetworkPolicyAPIToTerraform(ctx, found, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
