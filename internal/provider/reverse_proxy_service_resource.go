package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// NewReverseProxyService creates a new reverse proxy service resource.
func NewReverseProxyService() resource.Resource {
	return &ReverseProxyService{}
}

// ReverseProxyService defines the resource implementation.
type ReverseProxyService struct {
	client *netbird.Client
}

// ReverseProxyServiceModel describes the resource data model.
type ReverseProxyServiceModel struct {
	Id               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Domain           types.String `tfsdk:"domain"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	PassHostHeader   types.Bool   `tfsdk:"pass_host_header"`
	RewriteRedirects types.Bool   `tfsdk:"rewrite_redirects"`
	ProxyCluster     types.String `tfsdk:"proxy_cluster"`
	Targets          types.List   `tfsdk:"targets"`
	Auth             types.Object `tfsdk:"auth"`
}

// ReverseProxyServiceTargetModel describes a service target.
type ReverseProxyServiceTargetModel struct {
	TargetId   types.String `tfsdk:"target_id"`
	TargetType types.String `tfsdk:"target_type"`
	Host       types.String `tfsdk:"host"`
	Port       types.Int64  `tfsdk:"port"`
	Protocol   types.String `tfsdk:"protocol"`
	Path       types.String `tfsdk:"path"`
	Enabled    types.Bool   `tfsdk:"enabled"`
}

// TFType returns the Terraform object type for service targets.
func (m ReverseProxyServiceTargetModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"target_id":   types.StringType,
			"target_type": types.StringType,
			"host":        types.StringType,
			"port":        types.Int64Type,
			"protocol":    types.StringType,
			"path":        types.StringType,
			"enabled":     types.BoolType,
		},
	}
}

// ReverseProxyServiceAuthModel describes the auth config.
type ReverseProxyServiceAuthModel struct {
	PasswordAuth types.Object `tfsdk:"password_auth"`
	PinAuth      types.Object `tfsdk:"pin_auth"`
	BearerAuth   types.Object `tfsdk:"bearer_auth"`
	LinkAuth     types.Object `tfsdk:"link_auth"`
}

// TFType returns the Terraform object type for service auth.
func (m ReverseProxyServiceAuthModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"password_auth": ReverseProxyPasswordAuthModel{}.TFType(),
			"pin_auth":      ReverseProxyPinAuthModel{}.TFType(),
			"bearer_auth":   ReverseProxyBearerAuthModel{}.TFType(),
			"link_auth":     ReverseProxyLinkAuthModel{}.TFType(),
		},
	}
}

// ReverseProxyPasswordAuthModel describes password auth config.
type ReverseProxyPasswordAuthModel struct {
	Enabled  types.Bool   `tfsdk:"enabled"`
	Password types.String `tfsdk:"password"`
}

// TFType returns the Terraform object type for password auth.
func (m ReverseProxyPasswordAuthModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enabled":  types.BoolType,
			"password": types.StringType,
		},
	}
}

// ReverseProxyPinAuthModel describes PIN auth config.
type ReverseProxyPinAuthModel struct {
	Enabled types.Bool   `tfsdk:"enabled"`
	Pin     types.String `tfsdk:"pin"`
}

// TFType returns the Terraform object type for PIN auth.
func (m ReverseProxyPinAuthModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enabled": types.BoolType,
			"pin":     types.StringType,
		},
	}
}

// ReverseProxyBearerAuthModel describes bearer auth config.
type ReverseProxyBearerAuthModel struct {
	Enabled            types.Bool `tfsdk:"enabled"`
	DistributionGroups types.List `tfsdk:"distribution_groups"`
}

// TFType returns the Terraform object type for bearer auth.
func (m ReverseProxyBearerAuthModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enabled": types.BoolType,
			"distribution_groups": types.ListType{
				ElemType: types.StringType,
			},
		},
	}
}

// ReverseProxyLinkAuthModel describes link auth config.
type ReverseProxyLinkAuthModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

// TFType returns the Terraform object type for link auth.
func (m ReverseProxyLinkAuthModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"enabled": types.BoolType,
		},
	}
}

func (r *ReverseProxyService) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reverse_proxy_service"
}

func (r *ReverseProxyService) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and manage Reverse Proxy Services",
		MarkdownDescription: "Create and manage Reverse Proxy Services for the NetBird reverse proxy.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Service name",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "Domain for the service",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the service is enabled",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"pass_host_header": schema.BoolAttribute{
				MarkdownDescription: "When true, the original client Host header is passed through to the backend",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"rewrite_redirects": schema.BoolAttribute{
				MarkdownDescription: "When true, Location headers in backend responses are rewritten to replace the backend address with the public-facing domain",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"proxy_cluster": schema.StringAttribute{
				MarkdownDescription: "The proxy cluster handling this service (derived from domain)",
				Computed:            true,
			},
			"targets": schema.ListNestedAttribute{
				MarkdownDescription: "List of target backends for this service",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"target_id": schema.StringAttribute{
							MarkdownDescription: "Target ID (resource or peer ID)",
							Required:            true,
						},
						"target_type": schema.StringAttribute{
							MarkdownDescription: "Target type (peer, host, domain, subnet)",
							Required:            true,
							Validators:          []validator.String{stringvalidator.OneOf("peer", "host", "domain", "subnet")},
						},
						"host": schema.StringAttribute{
							MarkdownDescription: "Backend IP or domain for this target. If omitted, the API resolves it from the target peer.",
							Optional:            true,
							Computed:            true,
						},
						"port": schema.Int64Attribute{
							MarkdownDescription: "Backend port for this target (0 for scheme default)",
							Required:            true,
							Validators:          []validator.Int64{int64validator.Between(0, 65535)},
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Protocol to use when connecting to the backend (http, https)",
							Required:            true,
							Validators:          []validator.String{stringvalidator.OneOf("http", "https")},
						},
						"path": schema.StringAttribute{
							MarkdownDescription: "URL path prefix for this target. Defaults to \"/\" if omitted.",
							Optional:            true,
							Computed:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether this target is enabled",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
						},
					},
				},
			},
			"auth": schema.SingleNestedAttribute{
				MarkdownDescription: "Authentication configuration",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"password_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "Password authentication",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required: true,
							},
							"password": schema.StringAttribute{
								Optional:  true,
								Sensitive: true,
							},
						},
					},
					"pin_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "PIN authentication",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required: true,
							},
							"pin": schema.StringAttribute{
								Optional:  true,
								Sensitive: true,
							},
						},
					},
					"bearer_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "Bearer token authentication",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required: true,
							},
							"distribution_groups": schema.ListAttribute{
								MarkdownDescription: "List of group IDs that can use bearer auth",
								Optional:            true,
								ElementType:         types.StringType,
							},
						},
					},
					"link_auth": schema.SingleNestedAttribute{
						MarkdownDescription: "Link authentication",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required: true,
							},
						},
					},
				},
			},
		},
	}
}

func (r *ReverseProxyService) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func reverseProxyServiceAPIToTerraform(ctx context.Context, svc *api.Service, data *ReverseProxyServiceModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics

	data.Id = types.StringValue(svc.Id)
	data.Name = types.StringValue(svc.Name)
	data.Domain = types.StringValue(svc.Domain)
	data.Enabled = types.BoolValue(svc.Enabled)

	if svc.PassHostHeader != nil {
		data.PassHostHeader = types.BoolValue(*svc.PassHostHeader)
	} else {
		data.PassHostHeader = types.BoolValue(false)
	}

	if svc.RewriteRedirects != nil {
		data.RewriteRedirects = types.BoolValue(*svc.RewriteRedirects)
	} else {
		data.RewriteRedirects = types.BoolValue(false)
	}

	data.ProxyCluster = types.StringPointerValue(svc.ProxyCluster)

	var targets []ReverseProxyServiceTargetModel
	for _, t := range svc.Targets {
		target := ReverseProxyServiceTargetModel{
			TargetId:   types.StringValue(t.TargetId),
			TargetType: types.StringValue(string(t.TargetType)),
			Port:       types.Int64Value(int64(t.Port)),
			Protocol:   types.StringValue(string(t.Protocol)),
			Enabled:    types.BoolValue(t.Enabled),
			Host:       types.StringPointerValue(t.Host),
			Path:       types.StringPointerValue(t.Path),
		}
		targets = append(targets, target)
	}
	data.Targets, d = types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), targets)
	ret.Append(d...)

	authModel := ReverseProxyServiceAuthModel{}

	if svc.Auth.PasswordAuth != nil {
		authModel.PasswordAuth, d = types.ObjectValueFrom(ctx, ReverseProxyPasswordAuthModel{}.TFType().AttrTypes, ReverseProxyPasswordAuthModel{
			Enabled:  types.BoolValue(svc.Auth.PasswordAuth.Enabled),
			Password: types.StringValue(svc.Auth.PasswordAuth.Password),
		})
		ret.Append(d...)
	} else {
		authModel.PasswordAuth = types.ObjectNull(ReverseProxyPasswordAuthModel{}.TFType().AttrTypes)
	}

	if svc.Auth.PinAuth != nil {
		authModel.PinAuth, d = types.ObjectValueFrom(ctx, ReverseProxyPinAuthModel{}.TFType().AttrTypes, ReverseProxyPinAuthModel{
			Enabled: types.BoolValue(svc.Auth.PinAuth.Enabled),
			Pin:     types.StringValue(svc.Auth.PinAuth.Pin),
		})
		ret.Append(d...)
	} else {
		authModel.PinAuth = types.ObjectNull(ReverseProxyPinAuthModel{}.TFType().AttrTypes)
	}

	if svc.Auth.BearerAuth != nil {
		bearerModel := ReverseProxyBearerAuthModel{
			Enabled: types.BoolValue(svc.Auth.BearerAuth.Enabled),
		}
		if svc.Auth.BearerAuth.DistributionGroups != nil {
			bearerModel.DistributionGroups, d = types.ListValueFrom(ctx, types.StringType, *svc.Auth.BearerAuth.DistributionGroups)
			ret.Append(d...)
		} else {
			bearerModel.DistributionGroups = types.ListNull(types.StringType)
		}
		authModel.BearerAuth, d = types.ObjectValueFrom(ctx, ReverseProxyBearerAuthModel{}.TFType().AttrTypes, bearerModel)
		ret.Append(d...)
	} else {
		authModel.BearerAuth = types.ObjectNull(ReverseProxyBearerAuthModel{}.TFType().AttrTypes)
	}

	if svc.Auth.LinkAuth != nil {
		authModel.LinkAuth, d = types.ObjectValueFrom(ctx, ReverseProxyLinkAuthModel{}.TFType().AttrTypes, ReverseProxyLinkAuthModel{
			Enabled: types.BoolValue(svc.Auth.LinkAuth.Enabled),
		})
		ret.Append(d...)
	} else {
		authModel.LinkAuth = types.ObjectNull(ReverseProxyLinkAuthModel{}.TFType().AttrTypes)
	}

	data.Auth, d = types.ObjectValueFrom(ctx, ReverseProxyServiceAuthModel{}.TFType().AttrTypes, authModel)
	ret.Append(d...)

	return ret
}

// preserveAuthSecrets copies sensitive auth fields (password, pin) from prior state/plan
// into the current model since the API redacts these values on read.
// It also preserves the structure of optional auth blocks (like link_auth) that the API
// may not return when disabled, ensuring state matches plan.
func preserveAuthSecrets(priorAuth, currentAuth types.Object) (types.Object, diag.Diagnostics) {
	var ret diag.Diagnostics

	if priorAuth.IsNull() || priorAuth.IsUnknown() || currentAuth.IsNull() || currentAuth.IsUnknown() {
		return currentAuth, ret
	}

	priorAttrs := priorAuth.Attributes()
	currentAttrs := currentAuth.Attributes()

	// Preserve password_auth sensitive field from plan/prior state.
	if priorPw, ok := priorAttrs["password_auth"].(types.Object); ok && !priorPw.IsNull() {
		if curPw, ok := currentAttrs["password_auth"].(types.Object); ok && !curPw.IsNull() {
			priorPwAttrs := priorPw.Attributes()
			curPwAttrs := curPw.Attributes()
			// Always use prior password value — API redacts it on read.
			if pw, ok := priorPwAttrs["password"]; ok {
				curPwAttrs["password"] = pw
				obj, d := types.ObjectValue(ReverseProxyPasswordAuthModel{}.TFType().AttrTypes, curPwAttrs)
				ret.Append(d...)
				currentAttrs["password_auth"] = obj
			}
		}
	}

	// Preserve pin_auth sensitive field from plan/prior state.
	if priorPin, ok := priorAttrs["pin_auth"].(types.Object); ok && !priorPin.IsNull() {
		if curPin, ok := currentAttrs["pin_auth"].(types.Object); ok && !curPin.IsNull() {
			priorPinAttrs := priorPin.Attributes()
			curPinAttrs := curPin.Attributes()
			// Always use prior pin value — API redacts it on read.
			if pin, ok := priorPinAttrs["pin"]; ok {
				curPinAttrs["pin"] = pin
				obj, d := types.ObjectValue(ReverseProxyPinAuthModel{}.TFType().AttrTypes, curPinAttrs)
				ret.Append(d...)
				currentAttrs["pin_auth"] = obj
			}
		}
	}

	// Preserve link_auth from plan/prior when API returns null.
	if priorLink, ok := priorAttrs["link_auth"].(types.Object); ok && !priorLink.IsNull() {
		if curLink, ok := currentAttrs["link_auth"].(types.Object); ok && curLink.IsNull() {
			currentAttrs["link_auth"] = priorLink
		}
	}

	result, d := types.ObjectValue(ReverseProxyServiceAuthModel{}.TFType().AttrTypes, currentAttrs)
	ret.Append(d...)
	return result, ret
}

// preserveTargetPlanValues keeps the user's planned host/path values in targets since
// the API may override them (e.g. resolving host from the peer's IP).
// Targets are matched by target_id to handle potential reordering by the API.
func preserveTargetPlanValues(ctx context.Context, planTargets, apiTargets types.List) (types.List, diag.Diagnostics) {
	var ret diag.Diagnostics

	if planTargets.IsNull() || planTargets.IsUnknown() || apiTargets.IsNull() || apiTargets.IsUnknown() {
		return apiTargets, ret
	}

	var planModels, apiModels []ReverseProxyServiceTargetModel
	ret.Append(planTargets.ElementsAs(ctx, &planModels, false)...)
	ret.Append(apiTargets.ElementsAs(ctx, &apiModels, false)...)
	if ret.HasError() {
		return apiTargets, ret
	}

	planByID := make(map[string]ReverseProxyServiceTargetModel, len(planModels))
	for _, p := range planModels {
		planByID[p.TargetId.ValueString()] = p
	}

	for i, apiModel := range apiModels {
		planModel, ok := planByID[apiModel.TargetId.ValueString()]
		if !ok {
			continue
		}
		if !planModel.Host.IsNull() && !planModel.Host.IsUnknown() {
			apiModels[i].Host = planModel.Host
		}
		if !planModel.Path.IsNull() && !planModel.Path.IsUnknown() {
			apiModels[i].Path = planModel.Path
		}
	}

	result, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), apiModels)
	ret.Append(d...)
	return result, ret
}

func reverseProxyServiceTerraformToAPI(ctx context.Context, data *ReverseProxyServiceModel) (api.ServiceRequest, diag.Diagnostics) {
	var ret diag.Diagnostics

	req := api.ServiceRequest{
		Name:    data.Name.ValueString(),
		Domain:  data.Domain.ValueString(),
		Enabled: data.Enabled.ValueBool(),
	}

	if !data.PassHostHeader.IsNull() && !data.PassHostHeader.IsUnknown() {
		v := data.PassHostHeader.ValueBool()
		req.PassHostHeader = &v
	}
	if !data.RewriteRedirects.IsNull() && !data.RewriteRedirects.IsUnknown() {
		v := data.RewriteRedirects.ValueBool()
		req.RewriteRedirects = &v
	}

	var targetModels []ReverseProxyServiceTargetModel
	ret.Append(data.Targets.ElementsAs(ctx, &targetModels, false)...)
	if ret.HasError() {
		return req, ret
	}

	for _, t := range targetModels {
		target := api.ServiceTarget{
			TargetId:   t.TargetId.ValueString(),
			TargetType: api.ServiceTargetTargetType(t.TargetType.ValueString()),
			Port:       int(t.Port.ValueInt64()),
			Protocol:   api.ServiceTargetProtocol(t.Protocol.ValueString()),
			Enabled:    t.Enabled.ValueBool(),
		}
		if !t.Host.IsNull() && !t.Host.IsUnknown() {
			v := t.Host.ValueString()
			target.Host = &v
		}
		if !t.Path.IsNull() && !t.Path.IsUnknown() {
			v := t.Path.ValueString()
			target.Path = &v
		}
		req.Targets = append(req.Targets, target)
	}

	authAttrs := data.Auth.Attributes()

	if v, ok := authAttrs["password_auth"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
		pwAttrs := v.Attributes()
		enabled, _ := pwAttrs["enabled"].(types.Bool)
		password, _ := pwAttrs["password"].(types.String)
		req.Auth.PasswordAuth = &api.PasswordAuthConfig{
			Enabled:  enabled.ValueBool(),
			Password: password.ValueString(),
		}
	}

	if v, ok := authAttrs["pin_auth"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
		pinAttrs := v.Attributes()
		enabled, _ := pinAttrs["enabled"].(types.Bool)
		pin, _ := pinAttrs["pin"].(types.String)
		req.Auth.PinAuth = &api.PINAuthConfig{
			Enabled: enabled.ValueBool(),
			Pin:     pin.ValueString(),
		}
	}

	if v, ok := authAttrs["bearer_auth"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
		bearerAttrs := v.Attributes()
		enabled, _ := bearerAttrs["enabled"].(types.Bool)
		bearerAuth := &api.BearerAuthConfig{
			Enabled: enabled.ValueBool(),
		}
		if groupsList, ok := bearerAttrs["distribution_groups"].(types.List); ok && !groupsList.IsNull() && !groupsList.IsUnknown() {
			var groups []string
			ret.Append(groupsList.ElementsAs(ctx, &groups, false)...)
			bearerAuth.DistributionGroups = &groups
		}
		req.Auth.BearerAuth = bearerAuth
	}

	if v, ok := authAttrs["link_auth"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
		linkAttrs := v.Attributes()
		enabled, _ := linkAttrs["enabled"].(types.Bool)
		req.Auth.LinkAuth = &api.LinkAuthConfig{
			Enabled: enabled.ValueBool(),
		}
	}

	return req, ret
}

// Create creates a new reverse proxy service.
func (r *ReverseProxyService) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ReverseProxyServiceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceReq, d := reverseProxyServiceTerraformToAPI(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.client.ReverseProxyServices.Create(ctx, serviceReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating reverse proxy service", err.Error())
		return
	}

	// Save plan values to preserve fields the API may override
	planAuth := data.Auth
	planTargets := data.Targets

	resp.Diagnostics.Append(reverseProxyServiceAPIToTerraform(ctx, svc, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth, authDiags := preserveAuthSecrets(planAuth, data.Auth)
	resp.Diagnostics.Append(authDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Auth = auth

	targets, targetDiags := preserveTargetPlanValues(ctx, planTargets, data.Targets)
	resp.Diagnostics.Append(targetDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Targets = targets

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the Terraform state with the latest data from the API.
func (r *ReverseProxyService) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ReverseProxyServiceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save prior state to preserve fields the API may override
	priorAuth := data.Auth
	priorTargets := data.Targets

	svc, err := r.client.ReverseProxyServices.Get(ctx, data.Id.ValueString())
	if err != nil {
		if netbird.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error getting reverse proxy service", err.Error())
		return
	}

	resp.Diagnostics.Append(reverseProxyServiceAPIToTerraform(ctx, svc, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth, authDiags := preserveAuthSecrets(priorAuth, data.Auth)
	resp.Diagnostics.Append(authDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Auth = auth

	targets, targetDiags := preserveTargetPlanValues(ctx, priorTargets, data.Targets)
	resp.Diagnostics.Append(targetDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Targets = targets

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update modifies an existing reverse proxy service.
func (r *ReverseProxyService) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ReverseProxyServiceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceReq, d := reverseProxyServiceTerraformToAPI(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.client.ReverseProxyServices.Update(ctx, data.Id.ValueString(), serviceReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating reverse proxy service", err.Error())
		return
	}

	// Save plan values to preserve fields the API may override
	planAuth := data.Auth
	planTargets := data.Targets

	resp.Diagnostics.Append(reverseProxyServiceAPIToTerraform(ctx, svc, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auth, authDiags := preserveAuthSecrets(planAuth, data.Auth)
	resp.Diagnostics.Append(authDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Auth = auth

	targets, targetDiags := preserveTargetPlanValues(ctx, planTargets, data.Targets)
	resp.Diagnostics.Append(targetDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Targets = targets

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete removes a reverse proxy service.
func (r *ReverseProxyService) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ReverseProxyServiceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.ReverseProxyServices.Delete(ctx, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting reverse proxy service", err.Error())
	}
}

// ImportState imports an existing reverse proxy service into Terraform state.
func (r *ReverseProxyService) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var (
	_ resource.Resource                = &ReverseProxyService{}
	_ resource.ResourceWithImportState = &ReverseProxyService{}
)
