//go:build e2e

package provider

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_Peers_Create(t *testing.T) {
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "data.netbird_peers." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPeersDataSource(rName, `os`, `"Debian GNU/Linux 11"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "ids.#"),
					func(s *terraform.State) error {
						pIDCount := s.RootModule().Resources[rNameFull].Primary.Attributes["ids.#"]
						if pIDCount != "2" {
							return fmt.Errorf("Peer count mismatch, expected 2, found %s", pIDCount)
						}
						pIDs := []string{s.RootModule().Resources[rNameFull].Primary.Attributes["ids.0"], s.RootModule().Resources[rNameFull].Primary.Attributes["ids.1"]}
						slices.Sort(pIDs)
						if pIDs[0] != "peer1" || pIDs[1] != "peer2" {
							return fmt.Errorf("Peer IDs mismatch, expected [peer1 peer2], found %v", pIDs)
						}
						return nil
					},
				),
			},
		},
	})
}

func testPeersDataSource(rName, key, value string) string {
	return fmt.Sprintf(`data "netbird_peers" "%s" {
	%s = %s
}`, rName, key, value)
}
