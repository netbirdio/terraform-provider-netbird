// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Policy{}
var _ resource.ResourceWithImportState = &Policy{}

const portStringRegex = "^([0-9]{1,4}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$"

func NewPolicy() resource.Resource {
	return &Policy{}
}

// Policy defines the resource implementation.
type Policy struct {
	client *netbird.Client
}

// PolicyModel describes the resource data model.
type PolicyModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	SourcePostureChecks types.List   `tfsdk:"source_posture_checks"`
	Rules               types.List   `tfsdk:"rule"`
}

type PolicyRuleModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Action              types.String `tfsdk:"action"`
	Protocol            types.String `tfsdk:"protocol"`
	Ports               types.List   `tfsdk:"ports"`
	PortRanges          types.List   `tfsdk:"port_ranges"`
	Enabled             types.Bool   `tfsdk:"enabled"`
	Bidirectional       types.Bool   `tfsdk:"bidirectional"`
	Sources             types.List   `tfsdk:"sources"`
	SourceResource      types.Object `tfsdk:"source_resource"`
	Destinations        types.List   `tfsdk:"destinations"`
	DestinationResource types.Object `tfsdk:"destination_resource"`
}

func (p PolicyRuleModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":          types.StringType,
			"name":        types.StringType,
			"description": types.StringType,
			"action":      types.StringType,
			"protocol":    types.StringType,
			"ports": types.ListType{
				ElemType: types.StringType,
			},
			"port_ranges": types.ListType{
				ElemType: PolicyRulePortRangeModel{}.TFType(),
			},
			"enabled":       types.BoolType,
			"bidirectional": types.BoolType,
			"sources": types.ListType{
				ElemType: types.StringType,
			},
			"source_resource": PolicyRuleResourceModel{}.TFType(),
			"destinations": types.ListType{
				ElemType: types.StringType,
			},
			"destination_resource": PolicyRuleResourceModel{}.TFType(),
		},
	}
}

type PolicyRuleResourceModel struct {
	Id   types.String `tfsdk:"id"`
	Type types.String `tfsdk:"type"`
}

func (p PolicyRuleResourceModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":   types.StringType,
			"type": types.StringType,
		},
	}
}

type PolicyRulePortRangeModel struct {
	Start types.Int32 `tfsdk:"start"`
	End   types.Int32 `tfsdk:"end"`
}

func (p PolicyRulePortRangeModel) TFType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"start": types.Int32Type,
			"end":   types.Int32Type,
		},
	}
}

func (r *Policy) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *Policy) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and Manage Policies",
		MarkdownDescription: "Create and Manage Policies, See [NetBird Docs](https://docs.netbird.io/how-to/manage-network-access#policies) for more information.",

		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				Validators: []validator.List{listvalidator.SizeBetween(1, 1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Policy ID",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Policy Name",
							Required:            true,
						},
						"description": schema.StringAttribute{
							MarkdownDescription: "Policy description",
							Optional:            true,
						},
						"action": schema.StringAttribute{
							MarkdownDescription: "Policy Rule Action (accept|drop)",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("accept"),
							Validators:          []validator.String{stringvalidator.OneOf("accept", "drop")},
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Policy Rule Protocol (tcp|udp|icmp|all)",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString("all"),
							Validators:          []validator.String{stringvalidator.OneOf("tcp", "udp", "icmp", "all")},
						},
						"ports": schema.ListAttribute{
							MarkdownDescription: "Policy Rule Ports (mutually exclusive with port_ranges)",
							ElementType:         types.StringType,
							Optional:            true,
							Computed:            true,
							Validators:          []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("port_ranges")), listvalidator.ValueStringsAre(stringvalidator.RegexMatches(regexp.MustCompile(portStringRegex), "Port outside range 0 to 65535"))},
						},
						"port_ranges": schema.ListNestedAttribute{
							MarkdownDescription: "Policy Rule Port Ranges (mutually exclusive with ports)",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"start": schema.Int32Attribute{
										Required:   true,
										Validators: []validator.Int32{int32validator.Between(0, 65535)},
									},
									"end": schema.Int32Attribute{
										Required:   true,
										Validators: []validator.Int32{int32validator.Between(0, 65535)},
									},
								},
							},
							Optional:   true,
							Computed:   true,
							Validators: []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("ports"))},
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Policy Rule Enabled",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
						},
						"bidirectional": schema.BoolAttribute{
							MarkdownDescription: "Policy Rule Bidirectional",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
						},
						"sources": schema.ListAttribute{
							MarkdownDescription: "Policy Rule Source Groups (mutually exclusive with source_resource)",
							ElementType:         types.StringType,
							Optional:            true,
							Computed:            true,
							Validators:          []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("source_resource")), listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
						},
						"source_resource": schema.ObjectAttribute{
							MarkdownDescription: "Policy Rule Source Resource (mutually exclusive with sources)",
							AttributeTypes: map[string]attr.Type{
								"id":   types.StringType,
								"type": types.StringType,
							},
							Optional:   true,
							Computed:   true,
							Validators: []validator.Object{objectvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("sources"))},
						},
						"destinations": schema.ListAttribute{
							MarkdownDescription: "Policy Rule Destination Groups (mutually exclusive with destination_resource)",
							ElementType:         types.StringType,
							Optional:            true,
							Computed:            true,
							Validators:          []validator.List{listvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("destination_resource")), listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1))},
						},
						"destination_resource": schema.ObjectAttribute{
							MarkdownDescription: "Policy Rule Destination Resource (mutually exclusive with destinations)",
							AttributeTypes: map[string]attr.Type{
								"id":   types.StringType,
								"type": types.StringType,
							},
							Optional:   true,
							Computed:   true,
							Validators: []validator.Object{objectvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("destinations"))},
						},
					},
				},
			},
		},

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Policy ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Policy Name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Policy Description",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Policy enabled",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"source_posture_checks": schema.ListAttribute{
				MarkdownDescription: "Posture checks associated with policy",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *Policy) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

func policyAPIToTerraform(ctx context.Context, policy *api.Policy, data *PolicyModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var diag diag.Diagnostics
	data.Id = types.StringValue(*policy.Id)
	data.Name = types.StringValue(policy.Name)
	if policy.Description == nil || *policy.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(*policy.Description)
	}
	data.Enabled = types.BoolValue(policy.Enabled)
	data.SourcePostureChecks, diag = types.ListValueFrom(ctx, types.StringType, policy.SourcePostureChecks)
	ret.Append(diag...)
	var rulesList []PolicyRuleModel
	for _, r := range policy.Rules {
		ruleModel := PolicyRuleModel{
			Id:            types.StringValue(*r.Id),
			Name:          types.StringValue(r.Name),
			Action:        types.StringValue(string(r.Action)),
			Protocol:      types.StringValue(string(r.Protocol)),
			Enabled:       types.BoolValue(r.Enabled),
			Bidirectional: types.BoolValue(r.Bidirectional),
			Description:   types.StringPointerValue(r.Description),
		}
		if r.Description == nil || *r.Description == "" {
			ruleModel.Description = types.StringNull()
		}
		if r.Sources != nil {
			var sources []string
			for _, v := range *r.Sources {
				sources = append(sources, v.Id)
			}
			ruleModel.Sources, diag = types.ListValueFrom(ctx, types.StringType, sources)
			ret.Append(diag...)
		} else {
			ruleModel.Sources = types.ListNull(types.StringType)
		}
		if r.Destinations != nil {
			var destinations []string
			for _, v := range *r.Destinations {
				destinations = append(destinations, v.Id)
			}
			ruleModel.Destinations, diag = types.ListValueFrom(ctx, types.StringType, destinations)
			ret.Append(diag...)
		} else {
			ruleModel.Destinations = types.ListNull(types.StringType)
		}
		if r.Ports != nil {
			ruleModel.Ports, diag = types.ListValueFrom(ctx, types.StringType, *r.Ports)
			ret.Append(diag...)
		} else {
			ruleModel.Ports = types.ListNull(types.StringType)
		}
		if r.PortRanges != nil {
			var portRanges []PolicyRulePortRangeModel
			for _, v := range *r.PortRanges {
				portRanges = append(portRanges, PolicyRulePortRangeModel{
					Start: types.Int32Value(int32(v.Start)),
					End:   types.Int32Value(int32(v.End)),
				})
			}
			ruleModel.PortRanges, diag = types.ListValueFrom(ctx, PolicyRulePortRangeModel{}.TFType(), portRanges)
			ret.Append(diag...)
		} else {
			ruleModel.PortRanges = types.ListNull(PolicyRulePortRangeModel{}.TFType())
		}
		if r.SourceResource != nil {
			ruleModel.SourceResource, diag = types.ObjectValueFrom(ctx, PolicyRuleResourceModel{}.TFType().AttrTypes, PolicyRuleResourceModel{
				Id:   types.StringValue(r.SourceResource.Id),
				Type: types.StringValue(string(r.SourceResource.Type)),
			})
			ret.Append(diag...)
		} else {
			ruleModel.SourceResource = types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes)
		}
		if r.DestinationResource != nil {
			ruleModel.DestinationResource, diag = types.ObjectValueFrom(ctx, PolicyRuleResourceModel{}.TFType().AttrTypes, PolicyRuleResourceModel{
				Id:   types.StringValue(r.DestinationResource.Id),
				Type: types.StringValue(string(r.DestinationResource.Type)),
			})
			ret.Append(diag...)
		} else {
			ruleModel.DestinationResource = types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes)
		}
		rulesList = append(rulesList, ruleModel)
	}

	data.Rules, diag = types.ListValueFrom(ctx, PolicyRuleModel{}.TFType(), rulesList)
	ret.Append(diag...)
	return ret
}

func policyRulesTerraformToAPI(ctx context.Context, data *PolicyModel) ([]api.PolicyRuleUpdate, diag.Diagnostics) {
	var rules []api.PolicyRuleUpdate
	var ret diag.Diagnostics
	for i, r := range data.Rules.Elements() {
		ruleObject, ok := r.(types.Object)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d] expected to be types.Object, found %T", i, r))
			return nil, ret
		}
		ruleAction, ok := ruleObject.Attributes()["action"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].action expected to be types.String, found %T", i, ruleObject.Attributes()["action"]))
			return nil, ret
		}
		ruleBidirectional, ok := ruleObject.Attributes()["bidirectional"].(types.Bool)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].bidirectional expected to be types.Bool, found %T", i, ruleObject.Attributes()["bidirectional"]))
			return nil, ret
		}
		ruleEnabled, ok := ruleObject.Attributes()["enabled"].(types.Bool)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].enabled expected to be types.Bool, found %T", i, ruleObject.Attributes()["enabled"]))
			return nil, ret
		}
		ruleName, ok := ruleObject.Attributes()["name"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].name expected to be types.String, found %T", i, ruleObject.Attributes()["name"]))
			return nil, ret
		}
		ruleProtocol, ok := ruleObject.Attributes()["protocol"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].protocol expected to be types.String, found %T", i, ruleObject.Attributes()["protocol"]))
			return nil, ret
		}
		ruleID, ok := ruleObject.Attributes()["id"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].id expected to be types.String, found %T", i, ruleObject.Attributes()["id"]))
			return nil, ret
		}
		rule := api.PolicyRuleUpdate{
			Id:            ruleID.ValueStringPointer(),
			Action:        api.PolicyRuleUpdateAction(ruleAction.ValueString()),
			Bidirectional: ruleBidirectional.ValueBool(),
			Enabled:       ruleEnabled.ValueBool(),
			Name:          ruleName.ValueString(),
			Protocol:      api.PolicyRuleUpdateProtocol(ruleProtocol.ValueString()),
		}
		if v, ok := ruleObject.Attributes()["description"].(types.String); ok && !v.IsNull() && !v.IsUnknown() {
			rule.Description = v.ValueStringPointer()
		}
		if v, ok := ruleObject.Attributes()["destination_resource"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
			if _, ok := v.Attributes()["id"]; ok {
				rID, ok := v.Attributes()["id"].(types.String)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].destination_resource.id expected to be types.String, found %T", i, v.Attributes()["id"]))
					return nil, ret
				}
				rType, ok := v.Attributes()["type"].(types.String)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].destination_resource.type expected to be types.String, found %T", i, v.Attributes()["id"]))
					return nil, ret
				}
				rule.DestinationResource = &api.Resource{
					Id:   rID.ValueString(),
					Type: api.ResourceType(rType.ValueString()),
				}
			}
		}
		if v, ok := ruleObject.Attributes()["source_resource"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
			if _, ok := v.Attributes()["id"]; ok {
				rID, ok := v.Attributes()["id"].(types.String)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].source_resource.id expected to be types.String, found %T", i, v.Attributes()["id"]))
					return nil, ret
				}
				rType, ok := v.Attributes()["type"].(types.String)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].source_resource.type expected to be types.String, found %T", i, v.Attributes()["id"]))
					return nil, ret
				}
				rule.SourceResource = &api.Resource{
					Id:   rID.ValueString(),
					Type: api.ResourceType(rType.ValueString()),
				}
			}
		}
		if v, ok := ruleObject.Attributes()["sources"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
			rule.Sources = stringListDefaultPointer(ctx, v, nil)
		}
		if v, ok := ruleObject.Attributes()["destinations"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
			rule.Destinations = stringListDefaultPointer(ctx, v, nil)
		}
		if v, ok := ruleObject.Attributes()["ports"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
			rule.Ports = stringListDefaultPointer(ctx, v, nil)
		}
		if v, ok := ruleObject.Attributes()["port_ranges"].(types.List); ok && !v.IsNull() && !v.IsUnknown() {
			portRanges := []api.RulePortRange{}
			for j, k := range v.Elements() {
				portRange, ok := k.(types.Object)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].port_ranges[%d] expected to be types.Object, found %T", i, j, k))
					return nil, ret
				}
				prStart, ok := portRange.Attributes()["start"].(types.Int32)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].port_ranges[%d].start expected to be types.Int32, found %T", i, j, portRange.Attributes()["start"]))
					return nil, ret
				}
				prEnd, ok := portRange.Attributes()["end"].(types.Int32)
				if !ok {
					ret.AddError("Unexpected Value", fmt.Sprintf("data.Rules[%d].port_ranges[%d].end expected to be types.Int32, found %T", i, j, portRange.Attributes()["end"]))
					return nil, ret
				}
				portRanges = append(portRanges, api.RulePortRange{
					Start: int(prStart.ValueInt32()),
					End:   int(prEnd.ValueInt32()),
				})
			}
			rule.PortRanges = &portRanges
		}

		rules = append(rules, rule)
	}

	return rules, ret
}

func (r *Policy) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rules, d := policyRulesTerraformToAPI(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyReq := api.PostApiPoliciesJSONRequestBody{
		Name:                data.Name.ValueString(),
		Description:         data.Description.ValueStringPointer(),
		Enabled:             data.Enabled.ValueBool(),
		SourcePostureChecks: stringListDefaultPointer(ctx, data.SourcePostureChecks, nil),
		Rules:               rules,
	}

	policy, err := r.client.Policies.Create(ctx, policyReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating policy", err.Error())
		return
	}

	resp.Diagnostics.Append(policyAPIToTerraform(ctx, policy, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Policy) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() {
		return
	}

	policy, err := r.client.Policies.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting Policy", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(policyAPIToTerraform(ctx, policy, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Policy) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PolicyModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rules, d := policyRulesTerraformToAPI(ctx, &data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	policyReq := api.PolicyUpdate{
		Name:                data.Name.ValueString(),
		Description:         data.Description.ValueStringPointer(),
		Enabled:             data.Enabled.ValueBool(),
		SourcePostureChecks: stringListDefaultPointer(ctx, data.SourcePostureChecks, nil),
		Rules:               rules,
	}

	var policy *api.Policy
	var err error
	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		policy, err = r.client.Policies.Create(ctx, policyReq)
	} else {
		policy, err = r.client.Policies.Update(ctx, data.Id.ValueString(), api.PutApiPoliciesPolicyIdJSONRequestBody(policyReq))
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating Policy", err.Error())
		return
	}

	resp.Diagnostics.Append(policyAPIToTerraform(ctx, policy, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Policy) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Policies.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Policy", err.Error())
	}
}

func (r *Policy) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
