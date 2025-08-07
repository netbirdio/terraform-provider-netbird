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

func Test_Peers_Create(t *testing.T) {
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "data.netbird_peers." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPeersDataSource(rName, `os`, `"Debian GNU/Linux 11"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "ids.#"),
					func(s *terraform.State) error {
						pIDCount := s.RootModule().Resources[rNameFull].Primary.Attributes["ids.#"]
						if pIDCount != "2" {
							return fmt.Errorf("Peer count mismatch, expected 1, found %s", pIDCount)
						}
						pIDs := []string{s.RootModule().Resources[rNameFull].Primary.Attributes["ids.0"], s.RootModule().Resources[rNameFull].Primary.Attributes["ids.1"]}
						slices.Sort(pIDs)
						if pIDs[0] != "peer1" || pIDs[1] != "peer2" {
							return fmt.Errorf("Peer IDs mismatch, expected [peer1 peer2], found %v", pIDs)
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
