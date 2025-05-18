// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/management/client/rest"
)

// Ensure NetBirdProvider satisfies various provider interfaces.
var _ provider.Provider = &NetBirdProvider{}
var _ provider.ProviderWithFunctions = &NetBirdProvider{}
var _ provider.ProviderWithEphemeralResources = &NetBirdProvider{}

type NetBirdProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// NetBirdProviderModel describes the provider data model.
type NetBirdProviderModel struct {
	ManagementURL types.String `tfsdk:"management_url"`
	Token         types.String `tfsdk:"token"`
}

func (p *NetBirdProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "netbird"
	resp.Version = p.version
}

// Schema provider schema.
func (p *NetBirdProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"management_url": schema.StringAttribute{
				MarkdownDescription: "NetBird Management API URL",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Admin PAT for NetBird Management Server",
				Required:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *NetBirdProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data NetBirdProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	managementURL := "https://api.netbird.io"
	if data.ManagementURL.ValueString() != "" {
		managementURL = data.ManagementURL.ValueString()
	}
	client := netbird.New(managementURL, data.Token.ValueString())
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *NetBirdProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAccountSettings,
		NewDNSSettings,
		NewGroup,
		NewNameserverGroup,
		NewNetwork,
		NewNetworkResource,
		NewNetworkRouter,
		NewPeer,
		NewPolicy,
		NewPostureCheck,
		NewSetupKey,
		NewToken,
		NewUser,
	}
}

func (p *NetBirdProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *NetBirdProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAccountSettingsDataSource,
		NewDNSSettingsDataSource,
		NewGroupDataSource,
		NewNameserverGroupDataSource,
		NewNetworkDataSource,
		NewNetworkResourceDataSource,
		NewNetworkRouterDataSource,
		NewPeerDataSource,
		NewPolicyDataSource,
		NewPostureCheckDataSource,
		NewSetupKeyDataSource,
		NewTokenDataSource,
		NewUserDataSource,
		NewPeersDataSource,
	}
}

func (p *NetBirdProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NetBirdProvider{
			version: version,
		}
	}
}
