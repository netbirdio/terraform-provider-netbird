package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

var _ datasource.DataSource = &ReverseProxyDomainDataSource{}

// NewReverseProxyDomainDataSource creates a new reverse proxy domain data source.
func NewReverseProxyDomainDataSource() datasource.DataSource {
	return &ReverseProxyDomainDataSource{}
}

// ReverseProxyDomainDataSource defines the data source implementation.
type ReverseProxyDomainDataSource struct {
	client *netbird.Client
}

// ReverseProxyDomainDataSourceModel describes the data source model.
type ReverseProxyDomainDataSourceModel struct {
	Id            types.String `tfsdk:"id"`
	Domain        types.String `tfsdk:"domain"`
	TargetCluster types.String `tfsdk:"target_cluster"`
	Type          types.String `tfsdk:"type"`
	Validated     types.Bool   `tfsdk:"validated"`
}

func (d *ReverseProxyDomainDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_proxy_domain"
}

func (d *ReverseProxyDomainDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a reverse proxy domain by ID, domain name, or type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Domain ID",
				Optional:            true,
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain name",
				Optional:            true,
				Computed:            true,
			},
			"target_cluster": schema.StringAttribute{
				MarkdownDescription: "The proxy cluster this domain is validated against",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of reverse proxy domain (free, custom)",
				Optional:            true,
				Computed:            true,
			},
			"validated": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain has been validated",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (d *ReverseProxyDomainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReverseProxyDomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReverseProxyDomainDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Domain, data.Type, data.Validated) == 0 {
		resp.Diagnostics.AddError("Missing filter", "At least one filter attribute must be specified (id, domain, type, or validated).")
		return
	}

	domains, err := d.client.ReverseProxyDomains.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing reverse proxy domains", err.Error())
		return
	}

	var match *ReverseProxyDomainDataSourceModel
	for _, dom := range domains {
		score := matchString(dom.Id, data.Id) +
			matchString(dom.Domain, data.Domain) +
			matchString(string(dom.Type), data.Type) +
			matchBool(dom.Validated, data.Validated)

		if score > 0 {
			if match != nil {
				resp.Diagnostics.AddError("Multiple matches", "More than one reverse proxy domain matched the given filters. Please refine your query.")
				return
			}
			m := ReverseProxyDomainDataSourceModel{
				Id:            types.StringValue(dom.Id),
				Domain:        types.StringValue(dom.Domain),
				TargetCluster: types.StringPointerValue(dom.TargetCluster),
				Type:          types.StringValue(string(dom.Type)),
				Validated:     types.BoolValue(dom.Validated),
			}
			match = &m
		}
	}

	if match == nil {
		resp.Diagnostics.AddError("Not found", "No reverse proxy domain matched the given filters.")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, match)...)
}
