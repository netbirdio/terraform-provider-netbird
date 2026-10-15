//go:build e2e

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_Policy_Create_Groups(t *testing.T) {
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "443"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc"),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.0", "443"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.0", e2eGroupAllID()),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.0", e2eGroupNotAllID()),
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
							"Rules[0].Sources.0":      {e2eGroupAllID(), (*policy.Rules[0].Sources)[0].Id},
							"Rules[0].Destinations.#": {int(1), len(*policy.Rules[0].Destinations)},
							"Rules[0].Destinations.0": {e2eGroupNotAllID(), (*policy.Rules[0].Destinations)[0].Id},
						})
					},
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func Test_Policy_Create_Resources(t *testing.T) {
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceResources(rName, rName, "desc", "accept", "udp", e2eResourceSubnetID(), "subnet", e2eResourceDomainID(), "domain", "1000", "2000"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc"),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.id", e2eResourceSubnetID()),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.type", "subnet"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destination_resource.id", e2eResourceDomainID()),
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
							"Rules[0].SourceResource.ID":      {e2eResourceSubnetID(), policy.Rules[0].SourceResource.Id},
							"Rules[0].DestinationResource.ID": {e2eResourceDomainID(), policy.Rules[0].DestinationResource.Id},
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
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "443"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc-updated", "drop", "tcp", e2eGroupNotAllID(), e2eGroupAllID(), "80"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc-updated"),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "drop"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.0", "80"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.0", e2eGroupNotAllID()),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destinations.0", e2eGroupAllID()),
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
							"Rules[0].Sources.0":      {e2eGroupNotAllID(), (*policy.Rules[0].Sources)[0].Id},
							"Rules[0].Destinations.#": {int(1), len(*policy.Rules[0].Destinations)},
							"Rules[0].Destinations.0": {e2eGroupAllID(), (*policy.Rules[0].Destinations)[0].Id},
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
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "80"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testPolicyResourceResources(rName, rName, "desc", "accept", "udp", e2eResourceSubnetID(), "subnet", e2eResourceDomainID(), "domain", "1", "100"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "rule.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.ports.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.sources.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.id", e2eResourceSubnetID()),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.source_resource.type", "subnet"),
					resource.TestCheckResourceAttr(rNameFull, "rule.0.destination_resource.id", e2eResourceDomainID()),
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
							"Rules[0].SourceResource.ID":      {e2eResourceSubnetID(), policy.Rules[0].SourceResource.Id},
							"Rules[0].DestinationResource.ID": {e2eResourceDomainID(), policy.Rules[0].DestinationResource.Id},
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

// Test_Policy_RuleDescription walks a rule description through omitted, set,
// omitted again and an explicit "". The server stores and returns "" for a rule
// without a description, so omitted and "" have to mean the same thing: the
// explicit "" step is the one that failed with "Provider produced inconsistent
// result after apply" while an empty description was read back as null.
func Test_Policy_RuleDescription(t *testing.T) {
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: testPolicyResourceRuleDescription(rName, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					testCheckPolicyRuleDescription(rNameFull, ""),
				),
			},
			{
				Config:           testPolicyResourceRuleDescription(rName, `description = "set"`),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check:            testCheckPolicyRuleDescription(rNameFull, "set"),
			},
			{
				Config:           testPolicyResourceRuleDescription(rName, ""),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check:            testCheckPolicyRuleDescription(rNameFull, ""),
			},
			{
				Config: testPolicyResourceRuleDescription(rName, `description = ""`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: testCheckPolicyRuleDescription(rNameFull, ""),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Test_Policy_RuleDescription_CreateEmpty creates a rule with an explicit ""
// description, which has no prior state to hide behind.
func Test_Policy_RuleDescription_CreateEmpty(t *testing.T) {
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: testPolicyResourceRuleDescription(rName, `description = ""`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					testCheckPolicyRuleDescription(rNameFull, ""),
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// testCheckPolicyRuleDescription asserts the first rule's description both in
// state and on the management server.
func testCheckPolicyRuleDescription(address, want string) resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(address, "rule.0.description", want),
		func(s *terraform.State) error {
			pID := s.RootModule().Resources[address].Primary.Attributes["id"]
			policy, err := testClient().Policies.Get(context.Background(), pID)
			if err != nil {
				return err
			}
			if len(policy.Rules) != 1 {
				return fmt.Errorf("expected 1 rule on the management server, found %d", len(policy.Rules))
			}
			if got := policy.Rules[0].Description; got == nil || *got != want {
				return fmt.Errorf("expected rule description %q on the management server, found %v", want, valOr(got, "<nil>"))
			}
			return nil
		},
	)
}

// testPolicyResourceRuleDescription is a one-rule policy whose rule carries
// ruleDescription verbatim, so an empty string leaves the attribute out.
func testPolicyResourceRuleDescription(rName, ruleDescription string) string {
	return fmt.Sprintf(`resource "netbird_policy" "%[1]s" {
	name    = "%[1]s"
	enabled = true

	rule {
		name          = "%[1]s"
		%[2]s
		action        = "accept"
		bidirectional = true
		enabled       = true
		protocol      = "tcp"
		sources       = ["%[3]s"]
		destinations  = ["%[4]s"]
		ports         = ["443"]
	}
}`, rName, ruleDescription, e2eGroupAllID(), e2eGroupNotAllID())
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
