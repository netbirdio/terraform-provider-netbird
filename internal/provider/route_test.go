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

func Test_routeAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Route
		expected RouteModel
	}{
		{
			resource: &api.Route{
				Id:          "r1",
				Description: "desc",
				Enabled:     true,
				Domains:     &[]string{"example.com"},
				Groups:      []string{"g1"},
				KeepRoute:   true,
				Masquerade:  true,
				Metric:      9999,
				NetworkId:   "route",
				NetworkType: "domain",
				PeerGroups:  &[]string{"g2"},
			},
			expected: RouteModel{
				Id:                  types.StringValue("r1"),
				Description:         types.StringValue("desc"),
				Enabled:             types.BoolValue(true),
				KeepRoute:           types.BoolValue(true),
				Masquerade:          types.BoolValue(true),
				NetworkId:           types.StringValue("route"),
				NetworkType:         types.StringValue("domain"),
				PeerGroups:          types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
				Domains:             types.ListValueMust(types.StringType, []attr.Value{types.StringValue("example.com")}),
				Metric:              types.Int32Value(9999),
				Groups:              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				Peer:                types.StringNull(),
				Network:             types.StringNull(),
				AccessControlGroups: types.ListNull(types.StringType),
			},
		},
	}

	for _, c := range cases {
		var out RouteModel
		outDiag := routeAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_Route_Create(t *testing.T) {
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_route." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testRouteResource(rName, `group-all`, `null`, `desc`, `null`, `["example.com"]`, `["group-notall"]`, `null`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "network_id", rName),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "groups.0", "group-all"),
					resource.TestCheckResourceAttr(rNameFull, "access_control_groups.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc"),
					resource.TestCheckNoResourceAttr(rNameFull, "network"),
					resource.TestCheckResourceAttr(rNameFull, "domains.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "domains.0", "example.com"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.0", "group-notall"),
					resource.TestCheckNoResourceAttr(rNameFull, "peer"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						route, err := testClient().Routes.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"network_id":            {rName, route.NetworkId},
							"groups.#":              {int(1), len(route.Groups)},
							"groups.0":              {"group-all", route.Groups[0]},
							"access_control_groups": {nil, route.AccessControlGroups},
							"description":           {"desc", route.Description},
							"domains.#":             {int(1), len(*route.Domains)},
							"domains.0":             {"example.com", (*route.Domains)[0]},
							"peer_groups.#":         {int(1), len(*route.PeerGroups)},
							"peer_groups.0":         {"group-notall", (*route.PeerGroups)[0]},
						})
					},
				),
			},
		},
	})
}

func Test_Route_Update(t *testing.T) {
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_route." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testRouteResource(rName, `group-all`, `null`, `desc`, `null`, `["example.com"]`, `["group-notall"]`, `null`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testRouteResource(rName, `group-notall`, `["group-all"]`, `desc-updated`, `"100.10.0.0/16"`, `null`, `null`, `"peer1"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "network_id", rName),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "groups.0", "group-notall"),
					resource.TestCheckResourceAttr(rNameFull, "access_control_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "access_control_groups.0", "group-all"),
					resource.TestCheckResourceAttr(rNameFull, "description", "desc-updated"),
					resource.TestCheckResourceAttr(rNameFull, "network", "100.10.0.0/16"),
					resource.TestCheckResourceAttr(rNameFull, "domains.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "peer_groups.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "peer", "peer1"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						route, err := testClient().Routes.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"network_id":              {rName, route.NetworkId},
							"groups.#":                {int(1), len(route.Groups)},
							"groups.0":                {"group-notall", route.Groups[0]},
							"access_control_groups.#": {int(1), len(*route.AccessControlGroups)},
							"access_control_groups.0": {"group-all", (*route.AccessControlGroups)[0]},
							"description":             {"desc-updated", route.Description},
							"domains":                 {nil, route.Domains},
							"network":                 {"100.10.0.0/16", route.Network},
							"peer_group":              {nil, route.PeerGroups},
							"peer":                    {"peer1", route.Peer},
						})
					},
				),
			},
		},
	})
}

func testRouteResource(rName, groups, aclGroups, description, network, domains, peerGroups, peer string) string {
	return fmt.Sprintf(`resource "netbird_route" "%s" {
  network_id            = "%s"
  groups                = ["%s"]
  access_control_groups = %s
  description           = "%s"
  network               = %s
	domains               = %s
  peer_groups           = %s
	peer                  = %s
}
`, rName, rName, groups, aclGroups, description, network, domains, peerGroups, peer)
}
