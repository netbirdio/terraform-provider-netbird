//go:build e2e

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_Scim_Create(t *testing.T) {
	// Skip before testE2E: these need a cloud-only feature, and standing the
	// deployment up for a test that never runs costs an image build and a
	// container start for nothing.
	t.Skip("skipping cloud test")
	rName := "scim" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_scim." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testScimResource(rName, "okta", `"okta-scim"`, `null`, `["eng"]`, `["users"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "auth_token"),
					resource.TestCheckResourceAttrSet(rNameFull, "last_synced_at"),
					resource.TestCheckResourceAttr(rNameFull, "provider_name", "okta"),
					resource.TestCheckResourceAttr(rNameFull, "prefix", "okta-scim"),
					resource.TestCheckResourceAttr(rNameFull, "group_prefixes.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "group_prefixes.0", "eng"),
					resource.TestCheckResourceAttr(rNameFull, "user_group_prefixes.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "user_group_prefixes.0", "users"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						scim, err := testClient().SCIM.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"provider":              {"okta", scim.Provider},
							"group_prefixes.#":      {1, len(scim.GroupPrefixes)},
							"group_prefixes.0":      {"eng", scim.GroupPrefixes[0]},
							"user_group_prefixes.#": {1, len(scim.UserGroupPrefixes)},
							"user_group_prefixes.0": {"users", scim.UserGroupPrefixes[0]},
						})
					},
				),
			},
		},
	})
}

func Test_Scim_Update(t *testing.T) {
	// Skip before testE2E: these need a cloud-only feature, and standing the
	// deployment up for a test that never runs costs an image build and a
	// container start for nothing.
	t.Skip("skipping cloud test")
	rName := "scim" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_scim." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testScimResource(rName, "okta", `"okta-scim"`, `null`, `["eng"]`, `["users"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testScimResource(rName, "okta", `"okta-scim"`, `false`, `["eng", "product"]`, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "provider_name", "okta"),
					resource.TestCheckResourceAttr(rNameFull, "enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "group_prefixes.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "group_prefixes.0", "eng"),
					resource.TestCheckResourceAttr(rNameFull, "group_prefixes.1", "product"),
					resource.TestCheckResourceAttr(rNameFull, "user_group_prefixes.#", "0"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						scim, err := testClient().SCIM.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"provider":              {"okta", scim.Provider},
							"enabled":               {false, scim.Enabled},
							"group_prefixes.#":      {2, len(scim.GroupPrefixes)},
							"group_prefixes.0":      {"eng", scim.GroupPrefixes[0]},
							"group_prefixes.1":      {"product", scim.GroupPrefixes[1]},
							"user_group_prefixes.#": {0, len(scim.UserGroupPrefixes)},
						})
					},
				),
			},
		},
	})
}

func testScimResource(rName, provider, prefix, enabled, groupPrefixes, userGroupPrefixes string) string {
	return fmt.Sprintf(`resource "netbird_scim" "%s" {
  provider_name         = "%s"
  prefix                = %s
  enabled               = %s
  group_prefixes        = %s
  user_group_prefixes   = %s
}
`, rName, provider, prefix, enabled, groupPrefixes, userGroupPrefixes)
}
