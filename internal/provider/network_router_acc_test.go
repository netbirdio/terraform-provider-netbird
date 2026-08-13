//go:build e2e

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_NetworkRouter_Create(t *testing.T) {
	testE2E(t)
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_router." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testCheckGone(func(ctx context.Context, id string) (*api.NetworkRouter, error) {
			return testClient().Networks.Routers(e2eNetworkID()).Get(ctx, id)
		}, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, e2eNetworkID(), fmt.Sprintf("[%q]", e2eGroupNotAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", e2eGroupNotAllID()),
					func(s *terraform.State) error {
						nroID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						router, err := testClient().Networks.Routers(e2eNetworkID()).Get(context.Background(), nroID)
						if err != nil {
							return err
						}
						if router.PeerGroups == nil || len(*router.PeerGroups) == 0 {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected %s, found nothing on management server", e2eGroupNotAllID())
						}

						if (*router.PeerGroups)[0] != e2eGroupNotAllID() {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected %s, found %s on management server", e2eGroupNotAllID(), (*router.PeerGroups)[0])
						}

						return nil
					},
				),
			},
		},
	})
}

func Test_NetworkRouter_Update(t *testing.T) {
	testE2E(t)
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_router." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testCheckGone(func(ctx context.Context, id string) (*api.NetworkRouter, error) {
			return testClient().Networks.Routers(e2eNetworkID()).Get(ctx, id)
		}, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, e2eNetworkID(), fmt.Sprintf("[%q]", e2eGroupNotAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", e2eGroupNotAllID()),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, e2eNetworkID(), fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", e2eGroupAllID()),
					func(s *terraform.State) error {
						nroID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						router, err := testClient().Networks.Routers(e2eNetworkID()).Get(context.Background(), nroID)
						if err != nil {
							return err
						}
						if router.PeerGroups == nil || len(*router.PeerGroups) == 0 {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected %s, found nothing on management server", e2eGroupAllID())
						}

						if (*router.PeerGroups)[0] != e2eGroupAllID() {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected %s, found %s on management server", e2eGroupAllID(), (*router.PeerGroups)[0])
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
