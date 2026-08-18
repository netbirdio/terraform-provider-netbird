//go:build e2e

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_Policy_Create_Groups(t *testing.T) {
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "443"),
				Check: resource.ComposeAggregateTestCheckFunc(
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
		},
	})
}

func Test_Policy_Create_Resources(t *testing.T) {
	testE2E(t)
	rName := "po" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_policy." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceResources(rName, rName, "desc", "accept", "udp", e2eResourceSubnetID(), "subnet", e2eResourceDomainID(), "domain", "1000", "2000"),
				Check: resource.ComposeAggregateTestCheckFunc(
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
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "443"),
				Check: resource.ComposeAggregateTestCheckFunc(
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
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "80"),
				Check: resource.ComposeAggregateTestCheckFunc(
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
