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

func Test_NameserverGroup_Create(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						nsGroup, err := testClient().DNS.GetNameserverGroup(context.Background(), gID)
						if err != nil {
							return err
						}
						if nsGroup.Name != rName {
							return fmt.Errorf("NameserverGroup name mismatch, expected %s, found %s on management server", rName, nsGroup.Name)
						}
						if len(nsGroup.Nameservers) != 1 {
							return fmt.Errorf("NameserverGroup Nameservers mismatch, expected 1, found %d", len(nsGroup.Nameservers))
						}
						if nsGroup.Nameservers[0].Ip != `1.1.1.1` {
							return fmt.Errorf("NameserverGroup Nameservers.0.Ip mismatch, expected 1.1.1.1, found %s", nsGroup.Nameservers[0].Ip)
						}
						if nsGroup.Nameservers[0].NsType != `udp` {
							return fmt.Errorf("NameserverGroup Nameservers.0.NsType mismatch, expected udp, found %s", nsGroup.Nameservers[0].NsType)
						}
						if nsGroup.Nameservers[0].Port != 53 {
							return fmt.Errorf("NameserverGroup Nameservers.0.Port mismatch, expected 53, found %d", nsGroup.Nameservers[0].Port)
						}
						if len(nsGroup.Groups) != 1 {
							return fmt.Errorf("NameserverGroup Groups mismatch, expected 1, found %d", len(nsGroup.Groups))
						}
						if nsGroup.Groups[0] != e2eGroupAllID() {
							return fmt.Errorf("NameserverGroup Groups.0 mismatch, expected %s, found %s", e2eGroupAllID(), nsGroup.Groups[0])
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_NameserverGroup_Update(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `8.8.8.8`, `5353`, fmt.Sprintf("[%q]", e2eGroupNotAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						nsGroup, err := testClient().DNS.GetNameserverGroup(context.Background(), gID)
						if err != nil {
							return err
						}
						if nsGroup.Name != rName {
							return fmt.Errorf("NameserverGroup name mismatch, expected %s, found %s on management server", rName, nsGroup.Name)
						}
						if len(nsGroup.Nameservers) != 1 {
							return fmt.Errorf("NameserverGroup Nameservers mismatch, expected 1, found %d", len(nsGroup.Nameservers))
						}
						if nsGroup.Nameservers[0].Ip != `8.8.8.8` {
							return fmt.Errorf("NameserverGroup Nameservers.0.Ip mismatch, expected 8.8.8.8, found %s", nsGroup.Nameservers[0].Ip)
						}
						if nsGroup.Nameservers[0].NsType != `udp` {
							return fmt.Errorf("NameserverGroup Nameservers.0.NsType mismatch, expected udp, found %s", nsGroup.Nameservers[0].NsType)
						}
						if nsGroup.Nameservers[0].Port != 5353 {
							return fmt.Errorf("NameserverGroup Nameservers.0.Port mismatch, expected 5353, found %d", nsGroup.Nameservers[0].Port)
						}
						if len(nsGroup.Groups) != 1 {
							return fmt.Errorf("NameserverGroup Groups mismatch, expected 1, found %d", len(nsGroup.Groups))
						}
						if nsGroup.Groups[0] != e2eGroupNotAllID() {
							return fmt.Errorf("NameserverGroup Groups.0 mismatch, expected %s, found %s", e2eGroupNotAllID(), nsGroup.Groups[0])
						}
						return nil
					},
				),
			},
		},
	})
}

// ns_type is always "udp": it is the schema default and the only transport the
// nameserver API accepts today.

func testNameserverGroupResource(rName, ip, port, groups string) string {
	return fmt.Sprintf(`resource "netbird_nameserver_group" "%s" {
	name = "%s"
	nameservers = [
		{
			ip = "%s"
			ns_type = "udp"
			port = %s
		}
	]
	groups = %s
}`, rName, rName, ip, port, groups)
}
