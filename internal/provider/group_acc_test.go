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

func Test_Group_Create(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Groups.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testGroupResource(rName, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						group, err := testClient().Groups.Get(context.Background(), gID)
						if err != nil {
							return err
						}
						if group.Name != rName {
							return fmt.Errorf("Group name mismatch, expected %s, found %s on management server", rName, group.Name)
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

func Test_Group_Update(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
	// Resolved once, up front: the fixture helpers can fail the test, and doing
	// that from inside a Check closure aborts the run mid-apply, before
	// terraform-plugin-testing gets to its destroy step.
	peerID := testPeerID(t, "peer1")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			groups, err := testClient().Groups.List(context.Background())
			if err != nil {
				return err
			}
			for _, g := range groups {
				if g.Name == rName {
					return fmt.Errorf("Group not deleted")
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testGroupResource(rName, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
				),
			},
			{
				ResourceName: rName,
				Config:       testGroupResource(rName, fmt.Sprintf("[%q]", peerID)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "peers.0", peerID),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						group, err := testClient().Groups.Get(context.Background(), gID)
						if err != nil {
							return err
						}
						if len(group.Peers) != 1 {
							return fmt.Errorf("Group Peers not updated in management")
						}
						if group.Peers[0].Id != peerID {
							return fmt.Errorf("Group Peers incorrect")
						}
						return nil
					},
				),
			},
		},
	})
}

// A group gets resources from netbird_network_resource.groups, so any later change
// to that group has to round-trip them. They must survive the update, not just
// stop it from erroring.
func Test_Group_Update_KeepsAttachedResources(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
	renamed := rName + "renamed"
	// Resolved up front: a fixture helper failing inside a Check would abort mid-apply.
	peerID := testPeerID(t, "peer1")
	withPeer := fmt.Sprintf("[%q]", peerID)
	var createdID string

	config := func(name, peers string) string {
		return fmt.Sprintf(`resource "netbird_group" "%[1]s" {
	name  = "%[2]s"
	peers = %[4]s

	lifecycle {
		ignore_changes = [resources]
	}
}

resource "netbird_network_resource" "%[1]s" {
	network_id = %[3]q
	name       = "%[1]s"
	address    = "example.com"
	groups     = [netbird_group.%[1]s.id]
}`, rName, name, e2eNetworkID(), peers)
	}

	checkServer := func(name string, peers int) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
			group, err := testClient().Groups.Get(context.Background(), gID)
			if err != nil {
				return err
			}
			if group.Name != name {
				return fmt.Errorf("group name mismatch, expected %s, found %s on management server", name, group.Name)
			}
			if len(group.Resources) != 1 {
				return fmt.Errorf("expected 1 attached resource on management server, found %d", len(group.Resources))
			}
			if len(group.Peers) != peers {
				return fmt.Errorf("expected %d peers on management server, found %d", peers, len(group.Peers))
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Groups.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: config(rName, `[]`),
				Check:  testRecordID(rNameFull, &createdID),
			},
			{
				// The first apply attaches the resource after the group exists, so
				// only a refresh shows it on the group.
				Config: config(rName, `[]`),
				Check:  checkServer(rName, 0),
			},
			{
				Config: config(renamed, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "name", renamed),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					checkServer(renamed, 0),
				),
			},
			{
				Config: config(renamed, withPeer),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					checkServer(renamed, 1),
				),
			},
			{
				Config: config(renamed, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					checkServer(renamed, 0),
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

func testGroupResource(rName, peers string) string {
	return fmt.Sprintf(`resource "netbird_group" "%s" {
	name = "%s"
	peers = %s
}`, rName, rName, peers)
}
