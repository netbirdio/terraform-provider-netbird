package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_policyAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Policy
		expected PolicyModel
	}{
		{
			resource: &api.Policy{
				Id:                  valPtr("p1"),
				Name:                "testPol",
				Description:         valPtr("Test"),
				Enabled:             true,
				SourcePostureChecks: []string{"pc1"},
				Rules: []api.PolicyRule{
					{
						Action:        api.PolicyRuleActionAccept,
						Bidirectional: true,
						Description:   valPtr("Test"),
						Sources: &[]api.GroupMinimum{
							{
								Id: "g1",
							},
						},
						Destinations: &[]api.GroupMinimum{
							{
								Id: "g2",
							},
						},
						Enabled:  true,
						Id:       valPtr("p1"),
						Name:     "testPol",
						Ports:    &[]string{"22"},
						Protocol: api.PolicyRuleProtocolAll,
					},
				},
			},
			expected: PolicyModel{
				Id:                  types.StringValue("p1"),
				Name:                types.StringValue("testPol"),
				Description:         types.StringValue("Test"),
				Enabled:             types.BoolValue(true),
				SourcePostureChecks: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("pc1")}),
				Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
					"id":                   types.StringValue("p1"),
					"action":               types.StringValue("accept"),
					"bidirectional":        types.BoolValue(true),
					"description":          types.StringValue("Test"),
					"sources":              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
					"destinations":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
					"enabled":              types.BoolValue(true),
					"name":                 types.StringValue("testPol"),
					"ports":                types.ListValueMust(types.StringType, []attr.Value{types.StringValue("22")}),
					"protocol":             types.StringValue("all"),
					"port_ranges":          types.ListNull(PolicyRulePortRangeModel{}.TFType()),
					"source_resource":      types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
					"destination_resource": types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
				})}),
			},
		},
		{
			resource: &api.Policy{
				Id:                  valPtr("p1"),
				Name:                "testPol",
				Description:         valPtr("Test"),
				Enabled:             true,
				SourcePostureChecks: []string{"pc1"},
				Rules: []api.PolicyRule{
					{
						Action:        api.PolicyRuleActionAccept,
						Bidirectional: true,
						Description:   valPtr("Test"),
						SourceResource: &api.Resource{
							Id:   "r1",
							Type: api.ResourceTypeDomain,
						},
						DestinationResource: &api.Resource{
							Id:   "r2",
							Type: api.ResourceTypeDomain,
						},
						Enabled:  true,
						Id:       valPtr("p1"),
						Name:     "testPol",
						Ports:    &[]string{"22"},
						Protocol: api.PolicyRuleProtocolAll,
					},
				},
			},
			expected: PolicyModel{
				Id:                  types.StringValue("p1"),
				Name:                types.StringValue("testPol"),
				Description:         types.StringValue("Test"),
				Enabled:             types.BoolValue(true),
				SourcePostureChecks: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("pc1")}),
				Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
					"id":            types.StringValue("p1"),
					"action":        types.StringValue("accept"),
					"bidirectional": types.BoolValue(true),
					"description":   types.StringValue("Test"),
					"sources":       types.ListNull(types.StringType),
					"destinations":  types.ListNull(types.StringType),
					"enabled":       types.BoolValue(true),
					"name":          types.StringValue("testPol"),
					"ports":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("22")}),
					"protocol":      types.StringValue("all"),
					"port_ranges":   types.ListNull(PolicyRulePortRangeModel{}.TFType()),
					"source_resource": types.ObjectValueMust(PolicyRuleResourceModel{}.TFType().AttrTypes, map[string]attr.Value{
						"id":   types.StringValue("r1"),
						"type": types.StringValue("domain"),
					}),
					"destination_resource": types.ObjectValueMust(PolicyRuleResourceModel{}.TFType().AttrTypes, map[string]attr.Value{
						"id":   types.StringValue("r2"),
						"type": types.StringValue("domain"),
					}),
				})}),
			},
		},
	}

	for _, c := range cases {
		var out PolicyModel
		outDiag := policyAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_policyRulesTerraformToAPI(t *testing.T) {
	cases := []struct {
		resource *PolicyModel
		expected []api.PolicyRuleUpdate
	}{
		{
			resource: &PolicyModel{
				Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{
					types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
						"id":                   types.StringValue("r1"),
						"action":               types.StringValue("accept"),
						"bidirectional":        types.BoolValue(false),
						"description":          types.StringValue("Test"),
						"sources":              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
						"destinations":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
						"enabled":              types.BoolValue(false),
						"name":                 types.StringValue("test"),
						"ports":                types.ListValueMust(types.StringType, []attr.Value{types.StringValue("22")}),
						"protocol":             types.StringValue("all"),
						"port_ranges":          types.ListNull(PolicyRulePortRangeModel{}.TFType()),
						"source_resource":      types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
						"destination_resource": types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
					}),
				}),
			},
			expected: []api.PolicyRuleUpdate{
				{
					Action:        api.PolicyRuleUpdateActionAccept,
					Bidirectional: false,
					Description:   valPtr("Test"),
					Sources:       &[]string{"g1"},
					Destinations:  &[]string{"g2"},
					Enabled:       false,
					Id:            valPtr("r1"),
					Name:          "test",
					Ports:         &[]string{"22"},
					Protocol:      api.PolicyRuleUpdateProtocolAll,
				},
			},
		},
	}

	for _, c := range cases {
		out, outDiag := policyRulesTerraformToAPI(context.Background(), c.resource)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}

}
