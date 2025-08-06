package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_networkRouterAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.NetworkRouter
		expected NetworkRouterModel
	}{
		{
			resource: &api.NetworkRouter{
				Enabled:    false,
				Id:         "ro1",
				Masquerade: false,
				Metric:     1,
				Peer:       nil,
				PeerGroups: nil,
			},
			expected: NetworkRouterModel{
				Id:         types.StringValue("ro1"),
				Enabled:    types.BoolValue(false),
				PeerGroups: types.ListNull(types.StringType),
				Peer:       types.StringNull(),
				Metric:     types.Int32Value(1),
				Masquerade: types.BoolValue(false),
			},
		},
		{
			resource: &api.NetworkRouter{
				Enabled:    true,
				Id:         "ro2",
				Masquerade: true,
				Metric:     9999,
				Peer:       valPtr(""),
				PeerGroups: &[]string{"g1", "g2"},
			},
			expected: NetworkRouterModel{
				Id:         types.StringValue("ro2"),
				Enabled:    types.BoolValue(true),
				PeerGroups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1"), types.StringValue("g2")}),
				Peer:       types.StringNull(),
				Metric:     types.Int32Value(9999),
				Masquerade: types.BoolValue(true),
			},
		},
		{
			resource: &api.NetworkRouter{
				Enabled:    true,
				Id:         "ro3",
				Masquerade: true,
				Metric:     9999,
				Peer:       valPtr("p1"),
				PeerGroups: nil,
			},
			expected: NetworkRouterModel{
				Id:         types.StringValue("ro3"),
				Enabled:    types.BoolValue(true),
				PeerGroups: types.ListNull(types.StringType),
				Peer:       types.StringValue("p1"),
				Metric:     types.Int32Value(9999),
				Masquerade: types.BoolValue(true),
			},
		},
	}

	for _, c := range cases {
		var out NetworkRouterModel
		outDiag := networkRouterAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_NetworkRouter_Create(t *testing.T) {
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_router." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, "network1", `["group-notall"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", `group-notall`),
					func(s *terraform.State) error {
						nroID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						router, err := testClient().Networks.Routers("network1").Get(context.Background(), nroID)
						if err != nil {
							return err
						}
						if router.PeerGroups == nil || len(*router.PeerGroups) == 0 {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-notall, found nothing on management server")
						}

						if (*router.PeerGroups)[0] != "group-notall" {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-notall, found %s on management server", (*router.PeerGroups)[0])
						}

						return nil
					},
				),
			},
		},
	})
}

func Test_NetworkRouter_Update(t *testing.T) {
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_router." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, "network1", `["group-notall"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", `group-notall`),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkRouterResource(rName, "network1", `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", `1`),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", `group-all`),
					func(s *terraform.State) error {
						nroID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						router, err := testClient().Networks.Routers("network1").Get(context.Background(), nroID)
						if err != nil {
							return err
						}
						if router.PeerGroups == nil || len(*router.PeerGroups) == 0 {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-all, found nothing on management server")
						}

						if (*router.PeerGroups)[0] != "group-all" {
							return fmt.Errorf("NetworkRouter PeerGroups mismatch, expected group-all, found %s on management server", (*router.PeerGroups)[0])
						}

						return nil
					},
				),
			},
		},
	})
}

func testNetworkRouterResource(rName, networkID, peerGroup string) string {
	return fmt.Sprintf(`resource "netbird_network_router" "%s" {
	network_id = "%s"
	peer_groups = %s
}`, rName, networkID, peerGroup)
}
