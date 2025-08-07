package provider

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func Test_Policy_Create_Groups(t *testing.T) {
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", "group-all", "group-notall", "443"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc"),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.0", "443"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.0", "group-all"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.0", "group-notall"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						policy, err := testClient().Policies.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"Name":                    {rName, policy.Name},
							"Description":             {"desc", policy.Description},
							"Rules.#":                 {int(1), len(policy.Rules)},
							"Rules[0].Action":         {"accept", string(policy.Rules[0].Action)},
							"Rules[0].Ports.#":        {int(1), len(*policy.Rules[0].Ports)},
							"Rules[0].Ports.0":        {"443", (*policy.Rules[0].Ports)[0]},
							"Rules[0].Sources.#":      {int(1), len(*policy.Rules[0].Sources)},
							"Rules[0].Sources.0":      {"group-all", (*policy.Rules[0].Sources)[0].Id},
							"Rules[0].Destinations.#": {int(1), len(*policy.Rules[0].Destinations)},
							"Rules[0].Destinations.0": {"group-notall", (*policy.Rules[0].Destinations)[0].Id},
						})
					},
				),
			},
		},
	})
}

func Test_Policy_Create_Resources(t *testing.T) {
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceResources(rName, rName, "desc", "accept", "udp", "resource2", "subnet", "resource1", "domain", "1000", "2000"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc"),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.id", "resource2"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.type", "subnet"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destination_resource.id", "resource1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destination_resource.type", "domain"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.port_ranges.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.port_ranges.0.start", "1000"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.port_ranges.0.end", "2000"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						policy, err := testClient().Policies.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"Name":                            {rName, policy.Name},
							"Description":                     {"desc", policy.Description},
							"Rules.#":                         {int(1), len(policy.Rules)},
							"Rules[0].Action":                 {"accept", string(policy.Rules[0].Action)},
							"Rules[0].Sources.#":              {nil, policy.Rules[0].Sources},
							"Rules[0].Destinations.#":         {nil, policy.Rules[0].Destinations},
							"Rules[0].SourceResource.ID":      {"resource2", policy.Rules[0].SourceResource.Id},
							"Rules[0].DestinationResource.ID": {"resource1", policy.Rules[0].DestinationResource.Id},
							"Rules[0].PortRanges.#":           {int(1), len(*policy.Rules[0].PortRanges)},
							"Rules[0].PortRanges.0.Start":     {int(1000), (*policy.Rules[0].PortRanges)[0].Start},
							"Rules[0].PortRanges.0.End":       {int(2000), (*policy.Rules[0].PortRanges)[0].End},
						})
					},
				),
			},
		},
	})

}

func Test_Policy_Update_Groups(t *testing.T) {
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", "group-all", "group-notall", "443"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc-updated", "drop", "tcp", "group-notall", "group-all", "80"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc-updated"),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "drop"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.0", "80"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.0", "group-notall"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.0", "group-all"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						policy, err := testClient().Policies.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"Name":                    {rName, policy.Name},
							"Description":             {"desc-updated", policy.Description},
							"Rules.#":                 {int(1), len(policy.Rules)},
							"Rules[0].Action":         {"drop", string(policy.Rules[0].Action)},
							"Rules[0].Sources.#":      {int(1), len(*policy.Rules[0].Sources)},
							"Rules[0].Sources.0":      {"group-notall", (*policy.Rules[0].Sources)[0].Id},
							"Rules[0].Destinations.#": {int(1), len(*policy.Rules[0].Destinations)},
							"Rules[0].Destinations.0": {"group-all", (*policy.Rules[0].Destinations)[0].Id},
							"Rules[0].Ports.#":        {int(1), len(*policy.Rules[0].Ports)},
							"Rules[0].Ports.0":        {"80", (*policy.Rules[0].Ports)[0]},
						})
					},
				),
			},
		},
	})
}

func Test_Policy_Update_Resources(t *testing.T) {
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", "group-all", "group-notall", "80"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testPolicyResourceResources(rName, rName, "desc", "accept", "udp", "resource2", "subnet", "resource1", "domain", "1", "100"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.id", "resource2"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.type", "subnet"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destination_resource.id", "resource1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destination_resource.type", "domain"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.port_ranges.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.port_ranges.0.start", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.port_ranges.0.end", "100"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						policy, err := testClient().Policies.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"Name":                            {rName, policy.Name},
							"Description":                     {"desc", policy.Description},
							"Rules.#":                         {int(1), len(policy.Rules)},
							"Rules[0].Action":                 {"accept", string(policy.Rules[0].Action)},
							"Rules[0].Sources.#":              {nil, policy.Rules[0].Sources},
							"Rules[0].Destinations.#":         {nil, policy.Rules[0].Destinations},
							"Rules[0].SourceResource.ID":      {"resource2", policy.Rules[0].SourceResource.Id},
							"Rules[0].DestinationResource.ID": {"resource1", policy.Rules[0].DestinationResource.Id},
							"Rules[0].PortRanges.#":           {int(1), len(*policy.Rules[0].PortRanges)},
							"Rules[0].PortRanges.0.Start":     {int(1), (*policy.Rules[0].PortRanges)[0].Start},
							"Rules[0].PortRanges.0.End":       {int(100), (*policy.Rules[0].PortRanges)[0].End},
						})
					},
				),
			},
		},
	})
}

func testPolicyResourceGroups(rName, name, description, rAction, rProt, rSource, rDest, port string) string {
	return fmt.Sprintf(`resource "netbird_policy" "%s" {
	name    = "%s"
	description = "%s"
	enabled = true

	rule {
		action        = "%s"
		bidirectional = true
		enabled       = true
		protocol      = "%s"
		name          = "%s"
		sources       = ["%s"]
		destinations  = ["%s"]
		ports         = ["%s"]
	}
}`, rName, name, description, rAction, rProt, name, rSource, rDest, port)
}

func testPolicyResourceResources(rName, name, description, rAction, rProt, rSourceID, rSourceType, rDestID, rDestType, pStart, pEnd string) string {
	return fmt.Sprintf(`resource "netbird_policy" "%s" {
	name        = "%s"
	description = "%s"
	enabled     = true

	rule {
		action                = "%s"
		bidirectional         = true
		enabled               = true
		protocol              = "%s"
		name                  = "%s"
		source_resource       = {
			id = "%s"
			type = "%s"
		}
		destination_resource  = {
			id = "%s"
			type = "%s"
		}
		port_ranges = [
			{
				start = %s
				end   = %s
			}
		]
	}
}`, rName, name, description, rAction, rProt, name, rSourceID, rSourceType, rDestID, rDestType, pStart, pEnd)
}
