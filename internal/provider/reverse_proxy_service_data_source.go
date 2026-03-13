package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

var _ datasource.DataSource = &ReverseProxyServiceDataSource{}

// NewReverseProxyServiceDataSource creates a new reverse proxy service data source.
func NewReverseProxyServiceDataSource() datasource.DataSource {
	return &ReverseProxyServiceDataSource{}
}

// ReverseProxyServiceDataSource defines the data source implementation.
type ReverseProxyServiceDataSource struct {
	client *netbird.Client
}

// ReverseProxyServiceDataSourceModel describes the data source model.
type ReverseProxyServiceDataSourceModel struct {
	Id                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Domain             types.String `tfsdk:"domain"`
	Mode               types.String `tfsdk:"mode"`
	ListenPort         types.Int64  `tfsdk:"listen_port"`
	PortAutoAssigned   types.Bool   `tfsdk:"port_auto_assigned"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	PassHostHeader     types.Bool   `tfsdk:"pass_host_header"`
	RewriteRedirects   types.Bool   `tfsdk:"rewrite_redirects"`
	ProxyCluster       types.String `tfsdk:"proxy_cluster"`
	Targets            types.List   `tfsdk:"targets"`
	Auth               types.Object `tfsdk:"auth"`
	AccessRestrictions types.Object `tfsdk:"access_restrictions"`
}

func (d *ReverseProxyServiceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_proxy_service"
}

func (d *ReverseProxyServiceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a reverse proxy service by ID, name, or domain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Service name",
				Optional:            true,
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain for the service",
				Optional:            true,
				Computed:            true,
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "Service mode: \"http\" for L7 reverse proxy, \"tcp\"/\"udp\"/\"tls\" for L4 passthrough",
				Computed:            true,
			},
			"listen_port": schema.Int64Attribute{
				MarkdownDescription: "Port the proxy listens on (L4/TLS only)",
				Computed:            true,
			},
			"port_auto_assigned": schema.BoolAttribute{
				MarkdownDescription: "Whether the listen port was auto-assigned by the server",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the service is enabled",
				Computed:            true,
			},
			"pass_host_header": schema.BoolAttribute{
				MarkdownDescription: "When true, the original client Host header is passed through to the backend",
				Computed:            true,
			},
			"rewrite_redirects": schema.BoolAttribute{
				MarkdownDescription: "When true, Location headers in backend responses are rewritten to replace the backend address with the public-facing domain",
				Computed:            true,
			},
			"proxy_cluster": schema.StringAttribute{
				MarkdownDescription: "The proxy cluster handling this service",
				Computed:            true,
			},
			"targets": schema.ListNestedAttribute{
				MarkdownDescription: "List of target backends for this service",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"target_id": schema.StringAttribute{
							MarkdownDescription: "Target ID (resource or peer ID)",
							Computed:            true,
						},
						"target_type": schema.StringAttribute{
							MarkdownDescription: "Target type (peer, host, domain, subnet)",
							Computed:            true,
						},
						"host": schema.StringAttribute{
							MarkdownDescription: "Backend IP or domain for this target",
							Computed:            true,
						},
						"port": schema.Int64Attribute{
							MarkdownDescription: "Backend port for this target",
							Computed:            true,
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Protocol to use when connecting to the backend (http, https for HTTP mode; tcp, udp for L4 mode)",
							Computed:            true,
						},
						"path": schema.StringAttribute{
							MarkdownDescription: "URL path prefix for this target",
							Computed:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether this target is enabled",
							Computed:            true,
						},
						"options": schema.SingleNestedAttribute{
							MarkdownDescription: "Per-target options",
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"skip_tls_verify": schema.BoolAttribute{
									MarkdownDescription: "Skip TLS certificate verification for this backend (HTTPS targets only)",
									Computed:            true,
								},
								"request_timeout": schema.StringAttribute{
									MarkdownDescription: "Per-target response timeout as a Go duration string",
									Computed:            true,
								},
								"path_rewrite": schema.StringAttribute{
									MarkdownDescription: "Controls how the request path is rewritten before forwarding (HTTP only)",
									Computed:            true,
								},
								"custom_headers": schema.MapAttribute{
									MarkdownDescription: "Extra headers sent to the backend (HTTP only)",
									Computed:            true,
									ElementType:         types.StringType,
								},
								"proxy_protocol": schema.BoolAttribute{
									MarkdownDescription: "Send PROXY Protocol v2 header to this backend (TCP/TLS only)",
									Computed:            true,
								},
								"session_idle_timeout": schema.StringAttribute{
									MarkdownDescription: "Idle timeout before a UDP session is reaped (UDP only)",
									Computed:            true,
								},
							},
						},
					},
				},
			},
			"auth": schema.SingleNestedAttribute{
				MarkdownDescription: "Authentication configuration",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"password_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "Password authentication",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
							"password": schema.StringAttribute{
								Computed:  true,
								Sensitive: true,
							},
						},
					},
					"pin_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "PIN authentication",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
							"pin": schema.StringAttribute{
								Computed:  true,
								Sensitive: true,
							},
						},
					},
					"bearer_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "Bearer token authentication",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
							"distribution_groups": schema.ListAttribute{
								MarkdownDescription: "List of group IDs that can use bearer auth",
								Computed:            true,
								ElementType:         types.StringType,
							},
						},
					},
					"link_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "Link authentication",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Computed: true,
							},
						},
					},
					"header_auths": schema.ListNestedAttribute{
						MarkdownDescription: "Static header-value authentication rules",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"enabled": schema.BoolAttribute{
									Computed: true,
								},
								"header": schema.StringAttribute{
									MarkdownDescription: "HTTP header name to check",
									Computed:            true,
								},
								"value": schema.StringAttribute{
									MarkdownDescription: "Expected header value",
									Computed:            true,
									Sensitive:           true,
								},
							},
						},
					},
				},
			},
			"access_restrictions": schema.SingleNestedAttribute{
				MarkdownDescription: "Connection-level access restrictions based on IP or geography",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"allowed_cidrs": schema.ListAttribute{
						MarkdownDescription: "CIDR allowlist",
						Computed:            true,
						ElementType:         types.StringType,
					},
					"blocked_cidrs": schema.ListAttribute{
						MarkdownDescription: "CIDR blocklist",
						Computed:            true,
						ElementType:         types.StringType,
					},
					"allowed_countries": schema.ListAttribute{
						MarkdownDescription: "ISO 3166-1 alpha-2 country codes to allow",
						Computed:            true,
						ElementType:         types.StringType,
					},
					"blocked_countries": schema.ListAttribute{
						MarkdownDescription: "ISO 3166-1 alpha-2 country codes to block",
						Computed:            true,
						ElementType:         types.StringType,
					},
				},
			},
		},
	}
}

func (d *ReverseProxyServiceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ReverseProxyServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReverseProxyServiceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name, data.Domain) == 0 {
		resp.Diagnostics.AddError("Missing filter", "At least one filter attribute must be specified (id, name, or domain).")
		return
	}

	services, err := d.client.ReverseProxyServices.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing reverse proxy services", err.Error())
		return
	}

	var match *ReverseProxyServiceModel
	for _, svc := range services {
		score := matchString(svc.Id, data.Id) +
			matchString(svc.Name, data.Name) +
			matchString(svc.Domain, data.Domain)

		if score > 0 {
			if match != nil {
				resp.Diagnostics.AddError("Multiple matches", "More than one reverse proxy service matched the given filters. Please refine your query.")
				return
			}
			var m ReverseProxyServiceModel
			resp.Diagnostics.Append(reverseProxyServiceAPIToTerraform(ctx, &svc, &m)...)
			if resp.Diagnostics.HasError() {
				return
			}
			match = &m
		}
	}

	if match == nil {
		resp.Diagnostics.AddError("Not found", "No reverse proxy service matched the given filters.")
		return
	}

	result := ReverseProxyServiceDataSourceModel{
		Id:                 match.Id,
		Name:               match.Name,
		Domain:             match.Domain,
		Mode:               match.Mode,
		ListenPort:         match.ListenPort,
		PortAutoAssigned:   match.PortAutoAssigned,
		Enabled:            match.Enabled,
		PassHostHeader:     match.PassHostHeader,
		RewriteRedirects:   match.RewriteRedirects,
		ProxyCluster:       match.ProxyCluster,
		Targets:            match.Targets,
		Auth:               match.Auth,
		AccessRestrictions: match.AccessRestrictions,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}
