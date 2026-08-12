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

func Test_NetworkResource_Create(t *testing.T) {
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, "network1", `example.com`, `["group-notall", "group-all"]`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "address", "example.com"),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						resource, err := testClient().Networks.Resources("network1").Get(context.Background(), nreID)
						if err != nil {
							return err
						}

						if resource.Address != "example.com" {
							return fmt.Errorf("NetworkResource Address mismatch, expected example.com, found %s on management server", resource.Address)
						}

						// && binds tighter than ||, so the original condition was
						// len != 2 || (A && B): one wrong ID left the other correct
						// and the assertion passed. Compare the set instead.
						if !sameGroupIDs(resource.Groups, "group-all", "group-notall") {
							return fmt.Errorf("NetworkResource Groups mismatch, expected [group-notall, group-all], found %#v on management server", resource.Groups)
						}

						if resource.Name != rName {
							return fmt.Errorf("NetworkResource Name mismatch, expected %s, found %s on management server", rName, resource.Name)
						}

						return nil
					},
				),
			},
		},
	})
}

func Test_NetworkResource_Update(t *testing.T) {
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, "network1", `example.com`, `["group-notall"]`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, "network1", `google.com`, `["group-all", "group-notall"]`, rName+"Updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "address", "google.com"),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName+"Updated"),
					func(s *terraform.State) error {
						nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						resource, err := testClient().Networks.Resources("network1").Get(context.Background(), nreID)
						if err != nil {
							return err
						}

						if resource.Address != "google.com" {
							return fmt.Errorf("NetworkResource Address mismatch, expected google.com, found %s on management server", resource.Address)
						}

						// && binds tighter than ||, so the original condition was
						// len != 2 || (A && B): one wrong ID left the other correct
						// and the assertion passed. Compare the set instead.
						if !sameGroupIDs(resource.Groups, "group-all", "group-notall") {
							return fmt.Errorf("NetworkResource Groups mismatch, expected [group-notall, group-all], found %#v on management server", resource.Groups)
						}

						if resource.Name != rName+"Updated" {
							return fmt.Errorf("NetworkResource Name mismatch, expected %s, found %s on management server", rName+"Updated", resource.Name)
						}

						return nil
					},
				),
			},
		},
	})
}

func testNetworkResourceResource(rName, networkID, address, groups, name string) string {
	return fmt.Sprintf(`resource "netbird_network_resource" "%s" {
	network_id = "%s"
	address = "%s"
	groups = %s
	name = "%s"
}`, rName, networkID, address, groups, name)
}
