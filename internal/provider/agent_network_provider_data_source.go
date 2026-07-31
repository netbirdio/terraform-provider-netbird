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

var _ datasource.DataSource = &AgentNetworkProviderDataSource{}

func NewAgentNetworkProviderDataSource() datasource.DataSource {
	return &AgentNetworkProviderDataSource{}
}

type AgentNetworkProviderDataSource struct {
	client *agentNetworkClient
}

func (d *AgentNetworkProviderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_network_provider"
}

func (d *AgentNetworkProviderDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read an Agent Network provider by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Provider ID",
				Optional:            true,
				Computed:            true,
			},
			"provider_id": schema.StringAttribute{
				MarkdownDescription: "Catalog identifier for the upstream AI provider",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name shown in the dashboard",
				Optional:            true,
				Computed:            true,
			},
			"upstream_url": schema.StringAttribute{
				MarkdownDescription: "Full upstream URL that NetBird forwards traffic to",
				Computed:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Not returned by the API (sealed at rest)",
				Computed:            true,
				Sensitive:           true,
			},
			"bootstrap_cluster": schema.StringAttribute{
				MarkdownDescription: "Bootstrap cluster (only relevant on create)",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the provider is enabled",
				Computed:            true,
			},
			"skip_tls_verification": schema.BoolAttribute{
				MarkdownDescription: "Whether upstream TLS verification is skipped",
				Computed:            true,
			},
			"identity_header_user_id": schema.StringAttribute{
				MarkdownDescription: "Wire header for caller display identity",
				Computed:            true,
			},
			"identity_header_groups": schema.StringAttribute{
				MarkdownDescription: "Wire header for caller groups",
				Computed:            true,
			},
			"metadata_disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether identity-metadata injection is suppressed for this provider",
				Computed:            true,
			},
			"extra_values": schema.MapAttribute{
				MarkdownDescription: "Catalog-specific extra header values",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"models": schema.ListNestedAttribute{
				MarkdownDescription: "Models exposed through this endpoint",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Model identifier",
							Computed:            true,
						},
						"input_per_1k": schema.Float64Attribute{
							MarkdownDescription: "Cost per 1k input tokens in USD",
							Computed:            true,
						},
						"output_per_1k": schema.Float64Attribute{
							MarkdownDescription: "Cost per 1k output tokens in USD",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *AgentNetworkProviderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *AgentNetworkProviderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AgentNetworkProviderModel

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
		found, err := d.client.GetProvider(ctx, data.Id.ValueString())
		if err != nil {
			if netbird.IsNotFound(err) {
				resp.Diagnostics.AddError("No Match", "No Agent Network Provider matched the given filters")
				return
			}
			resp.Diagnostics.AddError("Error reading Agent Network Provider", err.Error())
			return
		}
		if matchString(found.Name, data.Name) < 0 {
			resp.Diagnostics.AddError("No Match", "No Agent Network Provider matched the given filters")
			return
		}
		resp.Diagnostics.Append(agentNetworkProviderAPIToTerraform(ctx, found, &data)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	providers, err := d.client.ListProviders(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing Agent Network Providers", err.Error())
		return
	}

	var found *api.AgentNetworkProvider
	for _, p := range providers {
		match := 0
		match += matchString(p.Id, data.Id)
		match += matchString(p.Name, data.Name)
		if match > 0 {
			if found != nil {
				resp.Diagnostics.AddError("Multiple Matches", "Data source matched more than one Agent Network Provider")
			}
			found = &p
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if found == nil {
		resp.Diagnostics.AddError("No Match", "No Agent Network Provider matched the given filters")
		return
	}

	resp.Diagnostics.Append(agentNetworkProviderAPIToTerraform(ctx, found, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
