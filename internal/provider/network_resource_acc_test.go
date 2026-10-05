//go:build e2e

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_NetworkResource_Create(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testCheckGone(func(ctx context.Context, id string) (*api.NetworkResource, error) {
			return testClient().Networks.Resources(e2eNetworkID()).Get(ctx, id)
		}, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, e2eNetworkID(), `example.com`, fmt.Sprintf("[%q, %q]", e2eGroupNotAllID(), e2eGroupAllID()), rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "address", "example.com"),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						resource, err := testClient().Networks.Resources(e2eNetworkID()).Get(context.Background(), nreID)
						if err != nil {
							return err
						}

						if resource.Address != "example.com" {
							return fmt.Errorf("NetworkResource Address mismatch, expected example.com, found %s on management server", resource.Address)
						}

						if !sameIDSet(resource.Groups, e2eGroupNotAllID(), e2eGroupAllID()) {
							return fmt.Errorf("NetworkResource Groups mismatch, expected [%s %s], found %#v on management server", e2eGroupNotAllID(), e2eGroupAllID(), resource.Groups)
						}

						if resource.Name != rName {
							return fmt.Errorf("NetworkResource Name mismatch, expected %s, found %s on management server", rName, resource.Name)
						}

						return nil
					},
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testImportIDFrom(rNameFull, "/", "network_id", "id"),
			},
		},
	})
}

func Test_NetworkResource_Update(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testCheckGone(func(ctx context.Context, id string) (*api.NetworkResource, error) {
			return testClient().Networks.Resources(e2eNetworkID()).Get(ctx, id)
		}, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, e2eNetworkID(), `example.com`, fmt.Sprintf("[%q]", e2eGroupNotAllID()), rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNetworkResourceResource(rName, e2eNetworkID(), `google.com`, fmt.Sprintf("[%q, %q]", e2eGroupAllID(), e2eGroupNotAllID()), rName+"Updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "address", "google.com"),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName+"Updated"),
					func(s *terraform.State) error {
						nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						resource, err := testClient().Networks.Resources(e2eNetworkID()).Get(context.Background(), nreID)
						if err != nil {
							return err
						}

						if resource.Address != "google.com" {
							return fmt.Errorf("NetworkResource Address mismatch, expected google.com, found %s on management server", resource.Address)
						}

						if !sameIDSet(resource.Groups, e2eGroupNotAllID(), e2eGroupAllID()) {
							return fmt.Errorf("NetworkResource Groups mismatch, expected [%s %s], found %#v on management server", e2eGroupNotAllID(), e2eGroupAllID(), resource.Groups)
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

func Test_NetworkResource_NoGroups(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_network_resource." + rName
	var createdID string
	checkServerGroups := func(want ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			nreID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
			nre, err := testClient().Networks.Resources(e2eNetworkID()).Get(context.Background(), nreID)
			if err != nil {
				return err
			}
			if !sameIDSet(nre.Groups, want...) {
				return fmt.Errorf("NetworkResource Groups mismatch, expected %v, found %#v on management server", want, nre.Groups)
			}
			return nil
		}
	}
	noGroupsConfig := fmt.Sprintf(`resource "netbird_network_resource" "%s" {
	network_id = "%s"
	address = "example.com"
	name = "%s"
}`, rName, e2eNetworkID(), rName)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: testCheckGone(func(ctx context.Context, id string) (*api.NetworkResource, error) {
			return testClient().Networks.Resources(e2eNetworkID()).Get(ctx, id)
		}, &createdID),
		Steps: []resource.TestStep{
			{
				Config: noGroupsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "0"),
					checkServerGroups(),
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					return e2eNetworkID() + "/" + createdID, nil
				},
			},
			{
				Config: testNetworkResourceResource(rName, e2eNetworkID(), `example.com`, fmt.Sprintf("[%q]", e2eGroupNotAllID()), rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "1"),
					checkServerGroups(e2eGroupNotAllID()),
				),
			},
			{
				Config: noGroupsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "groups.#", "0"),
					checkServerGroups(),
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
