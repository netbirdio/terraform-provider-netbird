// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
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
	TenantAccount types.String `tfsdk:"tenant_account"`
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
				MarkdownDescription: "NetBird Management API URL, can be also set through NB_MANAGEMENT_URL Environment Variable, value defined in Terraform files takes precedence",
				Optional:            true,
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "Admin PAT for NetBird Management Server, can be also set through NB_PAT Environment Variable, value defined in Terraform files takes precedence",
				Optional:            true,
				Sensitive:           true,
			},
			"tenant_account": schema.StringAttribute{
				MarkdownDescription: "Account ID to impersonate, can be also set through NB_ACCOUNT Environment Variable, value defined in Terraform files takes precedence",
				Optional:            true,
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
	if !data.ManagementURL.IsUnknown() && !data.ManagementURL.IsNull() {
		managementURL = data.ManagementURL.ValueString()
	} else if v, ok := os.LookupEnv("NB_MANAGEMENT_URL"); ok {
		managementURL = v
	}
	token := data.Token.ValueString()
	if data.Token.IsNull() || data.Token.IsUnknown() {
		if v, ok := os.LookupEnv("NB_PAT"); ok {
			token = v
		} else {
			resp.Diagnostics.AddError("Missing required argument", `The argument "token" is required, but was not set.`)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	client := netbird.New(managementURL, token)
	if !data.TenantAccount.IsNull() && !data.TenantAccount.IsUnknown() {
		client = client.Impersonate(data.TenantAccount.ValueString())
	} else if v, ok := os.LookupEnv("NB_ACCOUNT"); ok {
		client = client.Impersonate(v)
	}
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
		NewRoute,
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
		NewPeersDataSource,
		NewPolicyDataSource,
		NewPostureCheckDataSource,
		NewRouteDataSource,
		NewSetupKeyDataSource,
		NewTokenDataSource,
		NewUserDataSource,
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
