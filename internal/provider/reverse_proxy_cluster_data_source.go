package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

var _ datasource.DataSource = &ReverseProxyClustersDataSource{}

// NewReverseProxyClustersDataSource creates a new reverse proxy clusters data source.
func NewReverseProxyClustersDataSource() datasource.DataSource {
	return &ReverseProxyClustersDataSource{}
}

// ReverseProxyClustersDataSource defines the data source implementation.
type ReverseProxyClustersDataSource struct {
	client *netbird.Client
}

// ReverseProxyClusterModel describes a single cluster.
type ReverseProxyClusterModel struct {
	Address          types.String `tfsdk:"address"`
	ConnectedProxies types.Int64  `tfsdk:"connected_proxies"`
}

// ReverseProxyClustersDataSourceModel describes the data source model.
type ReverseProxyClustersDataSourceModel struct {
	Clusters []ReverseProxyClusterModel `tfsdk:"clusters"`
}

func (d *ReverseProxyClustersDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_proxy_clusters"
}

func (d *ReverseProxyClustersDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List available reverse proxy clusters.",
		Attributes: map[string]schema.Attribute{
			"clusters": schema.ListNestedAttribute{
				MarkdownDescription: "List of available proxy clusters",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							MarkdownDescription: "Cluster address used for CNAME targets",
							Computed:            true,
						},
						"connected_proxies": schema.Int64Attribute{
							MarkdownDescription: "Number of proxy nodes connected in this cluster",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ReverseProxyClustersDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReverseProxyClustersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReverseProxyClustersDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusters, err := d.client.ReverseProxyClusters.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing reverse proxy clusters", err.Error())
		return
	}

	data.Clusters = make([]ReverseProxyClusterModel, len(clusters))
	for i, c := range clusters {
		data.Clusters[i] = ReverseProxyClusterModel{
			Address:          types.StringValue(c.Address),
			ConnectedProxies: types.Int64Value(int64(c.ConnectedProxies)),
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
