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

func Test_networkAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Network
		expected NetworkModel
	}{
		{
			resource: &api.Network{
				Id:                "n1",
				Description:       valPtr("Test"),
				Name:              "Network",
				Policies:          []string{},
				Resources:         []string{},
				Routers:           []string{},
				RoutingPeersCount: 0,
			},
			expected: NetworkModel{
				Id:          types.StringValue("n1"),
				Name:        types.StringValue("Network"),
				Description: types.StringValue("Test"),
				Resources:   types.ListValueMust(types.StringType, []attr.Value{}),
				Routers:     types.ListValueMust(types.StringType, []attr.Value{}),
				Policies:    types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Network{
				Id:                "n2",
				Description:       valPtr("test2"),
				Name:              "Network2",
				Policies:          []string{"p1"},
				Resources:         []string{"r1"},
				Routers:           []string{"ro1"},
				RoutingPeersCount: 3,
			},
			expected: NetworkModel{
				Id:          types.StringValue("n2"),
				Name:        types.StringValue("Network2"),
				Description: types.StringValue("test2"),
				Resources:   types.ListValueMust(types.StringType, []attr.Value{types.StringValue("r1")}),
				Routers:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ro1")}),
				Policies:    types.ListValueMust(types.StringType, []attr.Value{types.StringValue("p1")}),
			},
		},
	}

	for _, c := range cases {
		var out NetworkModel
		outDiag := networkAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_Network_Create(t *testing.T) {
	rName := "n" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResource(rName, `Test`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", `Test`),
					func(s *terraform.State) error {
						nID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						network, err := testClient().Networks.Get(context.Background(), nID)
						if err != nil {
							return err
						}
						if network.Name != rName {
							return fmt.Errorf("Network name mismatch, expected %s, found %s on management server", rName, network.Name)
						}
						if network.Description == nil || *network.Description != `Test` {
							return fmt.Errorf("Network description mismatch, expected Test, found %s on management server", *network.Description)
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_Network_Update(t *testing.T) {
	rName := "n" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResource(rName, `Test`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkResource(rName, `Test V2`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", `Test V2`),
					func(s *terraform.State) error {
						nID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						network, err := testClient().Networks.Get(context.Background(), nID)
						if err != nil {
							return err
						}
						if network.Name != rName {
							return fmt.Errorf("Network name mismatch, expected %s, found %s on management server", rName, network.Name)
						}
						if network.Description == nil || *network.Description != `Test V2` {
							return fmt.Errorf("Network description mismatch, expected Test V2, found %s on management server", *network.Description)
						}
						return nil
					},
				),
			},
		},
	})
}

func testNetworkResource(rName, description string) string {
	return fmt.Sprintf(`resource "netbird_network" "%s" {
	name = "%s"
	description = "%s"
}`, rName, rName, description)
}
