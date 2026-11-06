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
			netResourceID := s.RootModule().Resources["netbird_network_resource."+rName].Primary.Attributes["id"]
			if group.Resources[0].Id != netResourceID {
				return fmt.Errorf("expected attached resource %s on management server, found %s", netResourceID, group.Resources[0].Id)
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
					resource.TestCheckResourceAttrPair(rNameFull, "resources.0", "netbird_network_resource."+rName, "id"),
					checkServer(renamed, 0),
				),
			},
			{
				Config: config(renamed, withPeer),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "resources.0", "netbird_network_resource."+rName, "id"),
					checkServer(renamed, 1),
				),
			},
			{
				Config: config(renamed, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "resources.0", "netbird_network_resource."+rName, "id"),
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

// Setting resources on the group itself resolves each ID's type through the network
// scan, which the round-trip test above never reaches. The network resources are
// attached to a separate group: referencing this group from them would be a cycle.
func Test_Group_ConfiguredResources(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
	resA := "netbird_network_resource." + rName + "_a"
	resB := "netbird_network_resource." + rName + "_b"
	var createdID string

	config := func(resources string) string {
		return fmt.Sprintf(`resource "netbird_group" "%[1]s_anchor" {
	name = "%[1]s_anchor"
}

resource "netbird_network_resource" "%[1]s_a" {
	network_id = %[2]q
	name       = "%[1]s_a"
	address    = "a.example.com"
	groups     = [netbird_group.%[1]s_anchor.id]

	# Attaching from the group also adds the group here.
	lifecycle {
		ignore_changes = [groups]
	}
}

resource "netbird_network_resource" "%[1]s_b" {
	network_id = %[2]q
	name       = "%[1]s_b"
	address    = "b.example.com"
	groups     = [netbird_group.%[1]s_anchor.id]

	lifecycle {
		ignore_changes = [groups]
	}
}

resource "netbird_group" "%[1]s" {
	name      = "%[1]s"
	resources = %[3]s
}`, rName, e2eNetworkID(), resources)
	}

	// checkServer compares the group's resources on the server with the IDs of the
	// named network resources in state.
	checkServer := func(names ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
			group, err := testClient().Groups.Get(context.Background(), gID)
			if err != nil {
				return err
			}
			if len(group.Resources) != len(names) {
				return fmt.Errorf("expected %d attached resources on management server, found %d", len(names), len(group.Resources))
			}
			for i, name := range names {
				want := s.RootModule().Resources[name].Primary.Attributes["id"]
				if group.Resources[i].Id != want {
					return fmt.Errorf("expected attached resource %s on management server, found %s", want, group.Resources[i].Id)
				}
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
				Config: config(fmt.Sprintf("[%s.id]", resA)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "resources.0", resA, "id"),
					checkServer(resA),
				),
			},
			{
				Config: config(fmt.Sprintf("[%s.id]", resB)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "resources.0", resB, "id"),
					checkServer(resB),
				),
			},
			{
				Config: config(`[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID),
					resource.TestCheckResourceAttr(rNameFull, "resources.#", "0"),
					checkServer(),
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
