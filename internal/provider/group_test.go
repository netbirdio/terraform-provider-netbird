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

func Test_groupAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Group
		expected GroupModel
	}{
		{
			resource: &api.Group{
				Id:             "abc",
				Issued:         valPtr(api.GroupIssuedApi),
				Name:           "Test",
				Peers:          []api.PeerMinimum{},
				PeersCount:     0,
				Resources:      []api.Resource{},
				ResourcesCount: 0,
			},
			expected: GroupModel{
				Id:        types.StringValue("abc"),
				Issued:    types.StringValue("api"),
				Name:      types.StringValue("Test"),
				Peers:     types.ListValueMust(types.StringType, []attr.Value{}),
				Resources: types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Group{
				Id:     "def",
				Issued: nil,
				Name:   "Meow",
				Peers: []api.PeerMinimum{
					{
						Id:   "c1",
						Name: "Useless",
					},
					{
						Id:   "c2",
						Name: "Also useless",
					},
				},
				PeersCount: 2,
				Resources: []api.Resource{
					{
						Id:   "r1",
						Type: api.ResourceTypeDomain,
					},
					{
						Id:   "r2",
						Type: api.ResourceTypeSubnet,
					},
				},
			},
			expected: GroupModel{
				Id:        types.StringValue("def"),
				Issued:    types.StringNull(),
				Name:      types.StringValue("Meow"),
				Peers:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("c1"), types.StringValue("c2")}),
				Resources: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("r1"), types.StringValue("r2")}),
			},
		},
	}

	for _, c := range cases {
		var out GroupModel
		outDiag := groupAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_Group_Create(t *testing.T) {
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testGroupResource(rName, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
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
		},
	})
}

func Test_Group_Update(t *testing.T) {
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_group." + rName
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
				Config:       testGroupResource(rName, `["peer1"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "peers.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "peers.0", "peer1"),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						group, err := testClient().Groups.Get(context.Background(), gID)
						if err != nil {
							return err
						}
						if len(group.Peers) != 1 {
							return fmt.Errorf("Group Peers not updated in management")
						}
						if group.Peers[0].Id != "peer1" {
							return fmt.Errorf("Group Peers incorrect")
						}
						return nil
					},
				),
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
