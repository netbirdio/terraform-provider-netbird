//go:build e2e

package provider

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_Peers_Create(t *testing.T) {
	testE2E(t)
	peerID := testPeerID(t, "peer1")

	reference, err := testClient().Peers.Get(context.Background(), peerID)
	if err != nil {
		t.Fatalf("read the reference agent: %v", err)
	}
	if reference.Os == "" {
		t.Skip("the registered agent reports no OS, nothing to filter on")
	}

	all, err := testClient().Peers.List(context.Background())
	if err != nil {
		t.Fatalf("list peers: %v", err)
	}
	var wantIDs []string
	for _, p := range all {
		if p.Os == reference.Os {
			wantIDs = append(wantIDs, p.Id)
		}
	}
	slices.Sort(wantIDs)
	if !slices.Contains(wantIDs, peerID) {
		t.Fatalf("expected the reference agent to match its own OS filter")
	}

	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "data.netbird_peers." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPeersDataSource(rName, `os`, fmt.Sprintf("%q", reference.Os)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "ids.#"),
					resource.TestCheckResourceAttr(rNameFull, "ids.#", fmt.Sprint(len(wantIDs))),
					func(s *terraform.State) error {
						attrs := s.RootModule().Resources[rNameFull].Primary.Attributes
						gotIDs := make([]string, 0, len(wantIDs))
						for i := range wantIDs {
							gotIDs = append(gotIDs, attrs[fmt.Sprintf("ids.%d", i)])
						}
						slices.Sort(gotIDs)
						if !slices.Equal(gotIDs, wantIDs) {
							return fmt.Errorf("Peer IDs mismatch, expected %v, found %v", wantIDs, gotIDs)
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
