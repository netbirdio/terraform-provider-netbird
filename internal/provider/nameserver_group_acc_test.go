//go:build e2e

package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_NameserverGroup_Create(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().DNS.GetNameserverGroup, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
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
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func Test_NameserverGroup_Update(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().DNS.GetNameserverGroup, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID())),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
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

// Test_NameserverGroup_MatchDomains applies the match-domain shapes the domains
// validator accepts. The unit test on fqdnRegex decides what the provider lets
// through; only a deployment decides what management stores, and a domain the
// provider accepts is a fix only if the server keeps it as written.
//
// Single-label domains are the reported bug (#145, #146): the regex demanded a
// dot, so "lan" and "consul" never reached the API that accepts them. The rest of
// the list is there for the round-trip rather than for the validator — management
// stores a match domain verbatim, so the trailing dot and the mixed case have to
// come back as they went in.
//
// That is what the step's own post-apply plan asserts, and it is the assertion
// that matters most here: a value the server normalised on write would return
// changed and diff on every plan afterwards, which surfaces as a non-empty plan
// rather than as a failed check.
func Test_NameserverGroup_MatchDomains(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	domains := []string{"lan", "consul", "home.example.com", "trail.example.com.", "MiXeD.example.com", "123.eks.internal"}
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().DNS.GetNameserverGroup, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupMatchDomains(rName, domains),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					// primary is unset in the configuration: a group with match
					// domains cannot also resolve everything.
					resource.TestCheckResourceAttr(rNameFull, "primary", "false"),
					resource.TestCheckResourceAttr(rNameFull, "domains.#", fmt.Sprintf("%d", len(domains))),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						nsGroup, err := testClient().DNS.GetNameserverGroup(context.Background(), gID)
						if err != nil {
							return err
						}
						if !slices.Equal(nsGroup.Domains, domains) {
							return fmt.Errorf("NameserverGroup Domains mismatch, expected %v, found %v on management server", domains, nsGroup.Domains)
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

// ns_type is always "udp": it is the schema default and the only transport the
// nameserver API accepts today.

func testNameserverGroupMatchDomains(rName string, domains []string) string {
	quoted := make([]string, len(domains))
	for i, d := range domains {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return fmt.Sprintf(`resource "netbird_nameserver_group" "%[1]s" {
	name = "%[1]s"
	domains = [%[2]s]
	nameservers = [
		{
			ip = "1.1.1.1"
			ns_type = "udp"
			port = 53
		}
	]
	groups = [%[3]q]
}`, rName, strings.Join(quoted, ", "), e2eGroupAllID())
}

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
