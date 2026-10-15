package provider

import (
	"context"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
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
					"authorized_groups":    types.MapNull(types.ListType{ElemType: types.StringType}),
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
					"authorized_groups": types.MapNull(types.ListType{ElemType: types.StringType}),
				})}),
			},
		},
		{
			resource: &api.Policy{
				Id:                  valPtr("p3"),
				Name:                "sshPolicy",
				Description:         valPtr("SSH with authorized groups"),
				Enabled:             true,
				SourcePostureChecks: []string{},
				Rules: []api.PolicyRule{
					{
						Action:        api.PolicyRuleActionAccept,
						Bidirectional: true,
						Sources: &[]api.GroupMinimum{
							{Id: "g1"},
						},
						Destinations: &[]api.GroupMinimum{
							{Id: "g2"},
						},
						Enabled:  true,
						Id:       valPtr("r3"),
						Name:     "ssh-rule",
						Protocol: api.PolicyRuleProtocolNetbirdSsh,
						AuthorizedGroups: &map[string][]string{
							"g1": {"root", "admin"},
						},
					},
				},
			},
			expected: PolicyModel{
				Id:                  types.StringValue("p3"),
				Name:                types.StringValue("sshPolicy"),
				Description:         types.StringValue("SSH with authorized groups"),
				Enabled:             types.BoolValue(true),
				SourcePostureChecks: types.ListValueMust(types.StringType, []attr.Value{}),
				Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
					"id":                   types.StringValue("r3"),
					"action":               types.StringValue("accept"),
					"bidirectional":        types.BoolValue(true),
					"description":          types.StringValue(""),
					"sources":              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
					"destinations":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
					"enabled":              types.BoolValue(true),
					"name":                 types.StringValue("ssh-rule"),
					"ports":                types.ListNull(types.StringType),
					"protocol":             types.StringValue("netbird-ssh"),
					"port_ranges":          types.ListNull(PolicyRulePortRangeModel{}.TFType()),
					"source_resource":      types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
					"destination_resource": types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
					"authorized_groups": types.MapValueMust(types.ListType{ElemType: types.StringType}, map[string]attr.Value{
						"g1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("root"), types.StringValue("admin")}),
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
						"authorized_groups":    types.MapNull(types.ListType{ElemType: types.StringType}),
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
		{
			resource: &PolicyModel{
				Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{
					types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
						"id":                   types.StringValue("r2"),
						"action":               types.StringValue("accept"),
						"bidirectional":        types.BoolValue(true),
						"description":          types.StringNull(),
						"sources":              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
						"destinations":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
						"enabled":              types.BoolValue(true),
						"name":                 types.StringValue("ssh-rule"),
						"ports":                types.ListNull(types.StringType),
						"protocol":             types.StringValue("netbird-ssh"),
						"port_ranges":          types.ListNull(PolicyRulePortRangeModel{}.TFType()),
						"source_resource":      types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
						"destination_resource": types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
						"authorized_groups": types.MapValueMust(types.ListType{ElemType: types.StringType}, map[string]attr.Value{
							"g1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("root")}),
						}),
					}),
				}),
			},
			expected: []api.PolicyRuleUpdate{
				{
					Action:        api.PolicyRuleUpdateActionAccept,
					Bidirectional: true,
					Sources:       &[]string{"g1"},
					Destinations:  &[]string{"g2"},
					Enabled:       true,
					Id:            valPtr("r2"),
					Name:          "ssh-rule",
					Protocol:      api.PolicyRuleUpdateProtocolNetbirdSsh,
					AuthorizedGroups: &map[string][]string{
						"g1": {"root"},
					},
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

func Test_policyRulesAuthorizedGroupsValidation(t *testing.T) {
	// authorized_groups should be rejected for non netbird-ssh protocols
	protocols := []string{"all", "tcp", "udp", "icmp"}
	for _, proto := range protocols {
		model := &PolicyModel{
			Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{
				types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
					"id":                   types.StringValue("r1"),
					"action":               types.StringValue("accept"),
					"bidirectional":        types.BoolValue(true),
					"description":          types.StringNull(),
					"sources":              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
					"destinations":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
					"enabled":              types.BoolValue(true),
					"name":                 types.StringValue("test"),
					"ports":                types.ListNull(types.StringType),
					"protocol":             types.StringValue(proto),
					"port_ranges":          types.ListNull(PolicyRulePortRangeModel{}.TFType()),
					"source_resource":      types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
					"destination_resource": types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
					"authorized_groups": types.MapValueMust(types.ListType{ElemType: types.StringType}, map[string]attr.Value{
						"g1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("root")}),
					}),
				}),
			}),
		}
		_, diag := policyRulesTerraformToAPI(context.Background(), model)
		if !diag.HasError() {
			t.Fatalf("Expected validation error for protocol %q with authorized_groups set, but got none", proto)
		}
	}

	// authorized_groups keys must be present in sources
	model := &PolicyModel{
		Rules: types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{
			types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, map[string]attr.Value{
				"id":                   types.StringValue("r1"),
				"action":               types.StringValue("accept"),
				"bidirectional":        types.BoolValue(true),
				"description":          types.StringNull(),
				"sources":              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				"destinations":         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
				"enabled":              types.BoolValue(true),
				"name":                 types.StringValue("test"),
				"ports":                types.ListNull(types.StringType),
				"protocol":             types.StringValue("netbird-ssh"),
				"port_ranges":          types.ListNull(PolicyRulePortRangeModel{}.TFType()),
				"source_resource":      types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
				"destination_resource": types.ObjectNull(PolicyRuleResourceModel{}.TFType().AttrTypes),
				"authorized_groups": types.MapValueMust(types.ListType{ElemType: types.StringType}, map[string]attr.Value{
					"not-in-sources": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("root")}),
				}),
			}),
		}),
	}
	_, diag := policyRulesTerraformToAPI(context.Background(), model)
	if !diag.HasError() {
		t.Fatal("Expected validation error for authorized_groups key not in sources, but got none")
	}
}

func Test_portRegex(t *testing.T) {
	r := regexp.MustCompile(portStringRegex)
	for i := range 65536 {
		str := strconv.FormatInt(int64(i), 10)
		if !r.MatchString(str) {
			t.Fatalf("Expected %d to match regex", i)
		}
	}

	noMatchCases := []string{
		"-1",
		"65536",
		"100000",
		"65635",
	}
	for _, c := range noMatchCases {
		if r.MatchString(c) {
			t.Fatalf("Exepected %s not to match regex", c)
		}
	}
}
