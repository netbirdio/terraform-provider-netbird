package provider

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_filterPeers(t *testing.T) {
	cases := []struct {
		peers    []api.Peer
		filter   PeersModel
		expected []string
	}{
		{
			peers: []api.Peer{
				{
					Groups: []api.GroupMinimum{
						{
							Id: "g1",
						},
					},
					Id: "p1",
				},
				{
					Groups: []api.GroupMinimum{
						{
							Id: "g1",
						},
						{
							Id: "g2",
						},
					},
					Id: "p2",
				},
				{
					Groups: []api.GroupMinimum{
						{
							Id: "g2",
						},
					},
					Id: "p3",
				},
			},
			filter: PeersModel{
				Groups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
			},
			expected: []string{"p1", "p2"},
		},
	}

	for _, c := range cases {
		out, outDiag := filterPeers(context.Background(), c.peers, c.filter)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !slices.Equal(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

// The peers data source filters server-side, so the expected result set is
// whatever management reports for the same filter. The filter value is taken
// from a real registered agent rather than hardcoded, because the OS string
// comes from whatever image the agent containers run.
func Test_Peers_Create(t *testing.T) {
	env := testE2E(t)
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
	if !slices.Contains(wantIDs, env.PeerIDs["peer1"]) {
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
