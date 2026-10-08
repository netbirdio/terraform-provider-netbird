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

func Test_Network_Create(t *testing.T) {
	testE2E(t)
	rName := "n" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Networks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResource(rName, `Test`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", `Test`),
					func(s *terraform.State) error {
						nID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						network, err := testClient().Networks.Get(context.Background(), nID)
						if err != nil {
							return err
						}
						if network.Name != rName {
							return fmt.Errorf("Network name mismatch, expected %s, found %s on management server", rName, network.Name)
						}
						// The nil branch has to return before the format below, which
						// dereferences the same pointer: on a server that omits the
						// field this panicked instead of failing the assertion.
						if network.Description == nil {
							return fmt.Errorf("Network description mismatch, expected Test, found none on management server")
						}
						if *network.Description != `Test` {
							return fmt.Errorf("Network description mismatch, expected Test, found %s on management server", *network.Description)
						}
						return nil
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

func Test_Network_Update(t *testing.T) {
	testE2E(t)
	rName := "n" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Networks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResource(rName, `Test`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkResource(rName, `Test V2`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", `Test V2`),
					func(s *terraform.State) error {
						nID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						network, err := testClient().Networks.Get(context.Background(), nID)
						if err != nil {
							return err
						}
						if network.Name != rName {
							return fmt.Errorf("Network name mismatch, expected %s, found %s on management server", rName, network.Name)
						}
						// The nil branch has to return before the format below, which
						// dereferences the same pointer: on a server that omits the
						// field this panicked instead of failing the assertion.
						if network.Description == nil {
							return fmt.Errorf("Network description mismatch, expected Test V2, found none on management server")
						}
						if *network.Description != `Test V2` {
							return fmt.Errorf("Network description mismatch, expected Test V2, found %s on management server", *network.Description)
						}
						return nil
					},
				),
			},
		},
	})
}

func testNetworkResource(rName, description string) string {
	return fmt.Sprintf(`resource "netbird_network" "%s" {
	name = "%s"
	description = "%s"
}`, rName, rName, description)
}
