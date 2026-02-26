package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var _ resource.Resource = &ReverseProxyDomain{}
var _ resource.ResourceWithImportState = &ReverseProxyDomain{}

// NewReverseProxyDomain creates a new reverse proxy domain resource.
func NewReverseProxyDomain() resource.Resource {
	return &ReverseProxyDomain{}
}

// ReverseProxyDomain defines the resource implementation.
type ReverseProxyDomain struct {
	client *netbird.Client
}

// ReverseProxyDomainModel describes the resource data model.
type ReverseProxyDomainModel struct {
	Id            types.String `tfsdk:"id"`
	Domain        types.String `tfsdk:"domain"`
	TargetCluster types.String `tfsdk:"target_cluster"`
	Type          types.String `tfsdk:"type"`
	Validated     types.Bool   `tfsdk:"validated"`
}

func (r *ReverseProxyDomain) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_proxy_domain"
}

func (r *ReverseProxyDomain) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage custom reverse proxy domains",
		MarkdownDescription: "Create and manage custom reverse proxy domains for the NetBird reverse proxy.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Domain ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"target_cluster": schema.StringAttribute{
				MarkdownDescription: "The proxy cluster this domain should be validated against",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of reverse proxy domain (free, custom)",
				Computed:            true,
			},
			"validated": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain has been validated",
				Computed:            true,
			},
		},
	}
}

func (r *ReverseProxyDomain) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func reverseProxyDomainAPIToTerraform(domain *api.ReverseProxyDomain, data *ReverseProxyDomainModel) {
	data.Id = types.StringValue(domain.Id)
	data.Domain = types.StringValue(domain.Domain)
	data.TargetCluster = types.StringPointerValue(domain.TargetCluster)
	data.Type = types.StringValue(string(domain.Type))
	data.Validated = types.BoolValue(domain.Validated)
}

func (r *ReverseProxyDomain) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ReverseProxyDomainModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainReq := api.ReverseProxyDomainRequest{
		Domain:        data.Domain.ValueString(),
		TargetCluster: data.TargetCluster.ValueString(),
	}

	domain, err := r.client.ReverseProxyDomains.Create(ctx, domainReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating reverse proxy domain", err.Error())
		return
	}

	reverseProxyDomainAPIToTerraform(domain, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ReverseProxyDomain) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ReverseProxyDomainModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API has no single-get endpoint for domains, so we list and filter.
	domains, err := r.client.ReverseProxyDomains.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing reverse proxy domains", err.Error())
		return
	}

	for _, d := range domains {
		if d.Id == data.Id.ValueString() {
			reverseProxyDomainAPIToTerraform(&d, &data)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *ReverseProxyDomain) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Reverse proxy domains cannot be updated in-place. Changes to domain or target_cluster require replacement.")
}

func (r *ReverseProxyDomain) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ReverseProxyDomainModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.ReverseProxyDomains.Delete(ctx, data.Id.ValueString()); err != nil {
		if netbird.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting reverse proxy domain", err.Error())
	}
}

func (r *ReverseProxyDomain) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
