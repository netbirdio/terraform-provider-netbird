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

func Test_NetworkRouter_Create(t *testing.T) {
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_router." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, "network1", `["group-notall"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", `group-notall`),
					func(s *terraform.State) error {
						nroID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						router, err := testClient().Networks.Routers("network1").Get(context.Background(), nroID)
						if err != nil {
							return err
						}
						if router.PeerGroups == nil || len(*router.PeerGroups) == 0 {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-notall, found nothing on management server")
						}

						if (*router.PeerGroups)[0] != "group-notall" {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-notall, found %s on management server", (*router.PeerGroups)[0])
						}

						return nil
					},
				),
			},
		},
	})
}

func Test_NetworkRouter_Update(t *testing.T) {
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_router." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, "network1", `["group-notall"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", `group-notall`),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, "network1", `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", `group-all`),
					func(s *terraform.State) error {
						nroID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						router, err := testClient().Networks.Routers("network1").Get(context.Background(), nroID)
						if err != nil {
							return err
						}
						if router.PeerGroups == nil || len(*router.PeerGroups) == 0 {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-all, found nothing on management server")
						}

						if (*router.PeerGroups)[0] != "group-all" {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-all, found %s on management server", (*router.PeerGroups)[0])
						}

						return nil
					},
				),
			},
		},
	})
}

func testNetworkRouterResource(rName, networkID, peerGroup string) string {
	return fmt.Sprintf(`resource "netbird_network_router" "%s" {
	network_id = "%s"
	peer_groups = %s
}`, rName, networkID, peerGroup)
}
