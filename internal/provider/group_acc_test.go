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
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
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

// Test_Group_PeerOrderStable configures a group's peers in the opposite order to
// the one the API reads them back in, and requires the plan to stay empty.
//
// Management keeps membership in a join table and reads it back with no ORDER BY,
// so the order a read returns is the store's, not the order the peers were sent
// in. A provider that copies that order into state shows a change to `peers` on
// every plan for any configuration that lists them differently.
func Test_Group_PeerOrderStable(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
	// The order is learned rather than assumed, so the test does not depend on
	// which store the deployment runs; the configuration is its reverse.
	readOrder := testGroupPeerReadOrder(t, testPeerID(t, "peer1"), testPeerID(t, "peer2"))
	configured := slices.Clone(readOrder)
	slices.Reverse(configured)
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Groups.Get, &createdID),
		Steps: []resource.TestStep{
			{
				// The post-apply plan the framework runs after this step is the
				// check on Create: it fails if state took the API's order.
				Config: testGroupResource(rName, testHCLList(configured)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "peers.0", configured[0]),
					resource.TestCheckResourceAttr(rNameFull, "peers.1", configured[1]),
					// Without this the test passes vacuously on a server that
					// happens to return the configured order.
					func(s *terraform.State) error {
						group, err := testClient().Groups.Get(context.Background(), createdID)
						if err != nil {
							return err
						}
						got := groupPeerIDs(group)
						if !slices.Equal(got, readOrder) {
							return fmt.Errorf("management read the peers as %v, expected %v: the read is not reordered, so this proves nothing", got, readOrder)
						}
						return nil
					},
				),
			},
			{
				// The refresh opening this step is the check on Read, with the
				// group's prior state to compare against.
				Config: testGroupResource(rName, testHCLList(configured)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// testGroupPeerReadOrder returns the order management reads the given peers back
// in, from a group created and deleted through the API for that purpose alone.
func testGroupPeerReadOrder(t *testing.T, peerIDs ...string) []string {
	t.Helper()
	ctx := context.Background()
	group, err := testClient().Groups.Create(ctx, api.GroupRequest{
		Name:  "order-probe-" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha),
		Peers: &peerIDs,
	})
	if err != nil {
		t.Fatalf("create the probe group: %v", err)
	}
	defer func() {
		if err := testClient().Groups.Delete(ctx, group.Id); err != nil {
			t.Errorf("delete the probe group: %v", err)
		}
	}()
	read, err := testClient().Groups.Get(ctx, group.Id)
	if err != nil {
		t.Fatalf("read the probe group: %v", err)
	}
	got := groupPeerIDs(read)
	if len(got) != len(peerIDs) {
		t.Fatalf("the probe group holds %v, expected the peers %v", got, peerIDs)
	}
	return got
}

func groupPeerIDs(group *api.Group) []string {
	ids := make([]string, len(group.Peers))
	for i, p := range group.Peers {
		ids[i] = p.Id
	}
	return ids
}

func testHCLList(v []string) string {
	q := make([]string, len(v))
	for i, s := range v {
		q[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

func testGroupResource(rName, peers string) string {
	return fmt.Sprintf(`resource "netbird_group" "%s" {
	name = "%s"
	peers = %s
}`, rName, rName, peers)
}
