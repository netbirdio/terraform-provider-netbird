//go:build e2e

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func Test_Account_Create(t *testing.T) {
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testAccountResource(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", "account1"),
					resource.TestCheckResourceAttr(rNameFull, "jwt_allow_groups.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_claim_name", ""),
					resource.TestCheckResourceAttr(rNameFull, "peer_login_expiration", "86400"),
					resource.TestCheckResourceAttr(rNameFull, "peer_inactivity_expiration", "0"),
					resource.TestCheckResourceAttr(rNameFull, "peer_login_expiration_enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "peer_inactivity_expiration_enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "regular_users_view_blocked", "true"),
					resource.TestCheckResourceAttr(rNameFull, "groups_propagation_enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "routing_peer_dns_resolution_enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "peer_approval_enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "network_traffic_logs_enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "network_traffic_packet_counter_enabled", "false"),
				),
			},
		},
	})
}

func Test_Account_Update(t *testing.T) {
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testAccountResourceWithJWT(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", "account1"),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "true"),
				),
				Destroy: false,
			},
			{
				ResourceName: rName,
				Config:       testAccountResourceWithJWT(rName, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", "account1"),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "false"),
				),
			},
		},
	})
}

func testAccountResource(rName string) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {}`, rName)
}

func testAccountResourceWithJWT(rName string, enabled bool) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {
jwt_groups_enabled = %v
}`, rName, enabled)
}
