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

func Test_DNSSettings_Create(t *testing.T) {
	testE2E(t)
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_dns_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, "[]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "0"),
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

func Test_DNSSettings_Update(t *testing.T) {
	testE2E(t)
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_dns_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, "[]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "0"),
				),
			},
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.0", e2eGroupAllID()),
				),
			},
		},
	})
}

func Test_DNSSettings_Delete(t *testing.T) {
	testE2E(t)
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_dns_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.0", e2eGroupAllID()),
				),
			},
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Destroy:      true,
				Check: func(s *terraform.State) error {
					settings, err := testClient().DNS.GetSettings(context.Background())
					if err != nil {
						return err
					}
					if len(settings.DisabledManagementGroups) != 1 || settings.DisabledManagementGroups[0] != e2eGroupAllID() {
						return fmt.Errorf("shouldn't change anything on delete")
					}
					return nil
				},
			},
		},
	})

}

func testDNSSettingsResource(rName, disabledManagementGroups string) string {
	return fmt.Sprintf(`resource "netbird_dns_settings" "%s" {
		disabled_management_groups = %s
	}`, rName, disabledManagementGroups)
}
