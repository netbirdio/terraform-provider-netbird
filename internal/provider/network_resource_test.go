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
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_networkResourceAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.NetworkResource
		expected NetworkResourceModel
	}{
		{
			resource: &api.NetworkResource{
				Address:     "1.1.1.1/32",
				Description: nil,
				Enabled:     false,
				Groups:      []api.GroupMinimum{},
				Id:          "r1",
				Name:        "test",
				Type:        api.NetworkResourceTypeSubnet,
			},
			expected: NetworkResourceModel{
				Id:          types.StringValue("r1"),
				Name:        types.StringValue("test"),
				Description: types.StringNull(),
				Address:     types.StringValue("1.1.1.1/32"),
				Enabled:     types.BoolValue(false),
				Groups:      types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.NetworkResource{
				Address:     "example.com",
				Description: valPtr("Test"),
				Enabled:     true,
				Groups: []api.GroupMinimum{
					{
						Id:             "g1",
						Name:           "not important",
						Issued:         nil,
						PeersCount:     0,
						ResourcesCount: 0,
					},
				},
				Id:   "r2",
				Name: "test2",
				Type: api.NetworkResourceTypeDomain,
			},
			expected: NetworkResourceModel{
				Id:          types.StringValue("r2"),
				Name:        types.StringValue("test2"),
				Description: types.StringValue("Test"),
				Address:     types.StringValue("example.com"),
				Enabled:     types.BoolValue(true),
				Groups:      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
			},
		},
	}

	for _, c := range cases {
		var out NetworkResourceModel
		outDiag := networkResourceAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_NetworkResource_Create(t *testing.T) {
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, "network1", `example.com`, `["group-notall"]`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "address", "example.com"),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "groups.0", "group-notall"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						resource, err := testClient().Networks.Resources("network1").Get(context.Background(), nreID)
						if err != nil {
							return err
						}

						if resource.Address != "example.com" {
							return fmt.Errorf("NetworkResource Address mismatch, expected example.com, found %s on management server", resource.Address)
						}

						if len(resource.Groups) != 1 || resource.Groups[0].Id != "group-notall" {
							return fmt.Errorf("NetworkResource Groups mismatch, expected [group-notall], found %#v on management server", resource.Groups)
						}

						if resource.Name != rName {
							return fmt.Errorf("NetworkResource Name mismatch, expected %s, found %s on management server", rName, resource.Name)
						}

						return nil
					},
				),
			},
		},
	})
}

func Test_NetworkResource_Update(t *testing.T) {
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, "network1", `example.com`, `["group-notall"]`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, "network1", `google.com`, `["group-all"]`, rName+"Updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "address", "google.com"),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "groups.0", "group-all"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName+"Updated"),
					func(s *terraform.State) error {
						nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						resource, err := testClient().Networks.Resources("network1").Get(context.Background(), nreID)
						if err != nil {
							return err
						}

						if resource.Address != "google.com" {
							return fmt.Errorf("NetworkResource Address mismatch, expected google.com, found %s on management server", resource.Address)
						}

						if len(resource.Groups) != 1 || resource.Groups[0].Id != "group-all" {
							return fmt.Errorf("NetworkResource Groups mismatch, expected [group-all], found %#v on management server", resource.Groups)
						}

						if resource.Name != rName+"Updated" {
							return fmt.Errorf("NetworkResource Name mismatch, expected %s, found %s on management server", rName+"Updated", resource.Name)
						}

						return nil
					},
				),
			},
		},
	})
}

func testNetworkResourceResource(rName, networkID, address, groups, name string) string {
	return fmt.Sprintf(`resource "netbird_network_resource" "%s" {
	network_id = "%s"
	address = "%s"
	groups = %s
	name = "%s"
}`, rName, networkID, address, groups, name)
}
