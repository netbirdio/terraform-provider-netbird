// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

const (
	defaultRequestTimeoutSeconds = 30
	defaultMaxRetries            = 4
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
	ManagementURL  types.String `tfsdk:"management_url"`
	Token          types.String `tfsdk:"token"`
	TenantAccount  types.String `tfsdk:"tenant_account"`
	RequestTimeout types.Int64  `tfsdk:"request_timeout"`
	MaxRetries     types.Int64  `tfsdk:"max_retries"`
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
			"request_timeout": schema.Int64Attribute{
				MarkdownDescription: "Per-attempt timeout in seconds for Management API requests, can be also set through NB_REQUEST_TIMEOUT Environment Variable. Defaults to 30.",
				Optional:            true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Maximum number of retries for transient Management API failures (429, 5xx, and connection errors; 5xx and connection retries are limited to idempotent methods), can be also set through NB_MAX_RETRIES Environment Variable. Defaults to 4, set to 0 to disable retries.",
				Optional:            true,
			},
		},
	}
}

// intSetting resolves an optional int provider setting: explicit config value
// wins, then the environment variable, then the supplied default. Values below
// minVal (from either source) are rejected and fall back to the default.
// minVal lets callers forbid nonsensical values, e.g. request_timeout must be
// >= 1 (a 0 per-attempt timeout would expire immediately and cause a retry
// storm), while max_retries allows 0 to disable retries.
func intSetting(v types.Int64, envVar string, def, minVal int) int {
	if !v.IsNull() && !v.IsUnknown() {
		if n := int(v.ValueInt64()); n >= minVal {
			return n
		}
	}
	if s, ok := os.LookupEnv(envVar); ok {
		if n, err := strconv.Atoi(s); err == nil && n >= minVal {
			return n
		}
	}
	return def
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
	// request_timeout must be >= 1s; max_retries may be 0 to disable retries.
	requestTimeout := intSetting(data.RequestTimeout, "NB_REQUEST_TIMEOUT", defaultRequestTimeoutSeconds, 1)
	maxRetries := intSetting(data.MaxRetries, "NB_MAX_RETRIES", defaultMaxRetries, 0)
	httpClient := newRetryingHTTPClient(time.Duration(requestTimeout)*time.Second, maxRetries)

	client := netbird.NewWithOptions(
		netbird.WithManagementURL(managementURL),
		netbird.WithPAT(token),
		netbird.WithUserAgent(fmt.Sprintf("terraform-provider-netbird/%s Terraform/%s", p.version, req.TerraformVersion)),
		netbird.WithHttpClient(httpClient))
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
		NewDNSZone,
		NewDNSRecord,
		NewGroup,
		NewIdentityProvider,
		NewNameserverGroup,
		NewNetwork,
		NewNetworkResource,
		NewNetworkRouter,
		NewPeer,
		NewPolicy,
		NewPostureCheck,
		NewRoute,
		NewScim,
		NewSetupKey,
		NewReverseProxyDomain,
		NewReverseProxyService,
		NewToken,
		NewUser,
		NewAgentNetworkProvider,
		NewAgentNetworkPolicy,
		NewAgentNetworkGuardrail,
		NewAgentNetworkSettings,
	}
}

func (p *NetBirdProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *NetBirdProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAccountSettingsDataSource,
		NewDNSSettingsDataSource,
		NewDNSZoneDataSource,
		NewDNSRecordDataSource,
		NewGroupDataSource,
		NewIdentityProviderDataSource,
		NewNameserverGroupDataSource,
		NewNetworkDataSource,
		NewNetworkResourceDataSource,
		NewNetworkRouterDataSource,
		NewPeerDataSource,
		NewPeersDataSource,
		NewPolicyDataSource,
		NewPostureCheckDataSource,
		NewRouteDataSource,
		NewScimDataSource,
		NewSetupKeyDataSource,
		NewReverseProxyClustersDataSource,
		NewReverseProxyDomainDataSource,
		NewReverseProxyServiceDataSource,
		NewTokenDataSource,
		NewUserDataSource,
		NewAgentNetworkProviderDataSource,
		NewAgentNetworkPolicyDataSource,
		NewAgentNetworkGuardrailDataSource,
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
