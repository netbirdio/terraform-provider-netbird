package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_scimAPIToTerraform(t *testing.T) {
	syncTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	cases := []struct {
		name     string
		resource *api.ScimIntegration
		expected ScimModel
	}{
		{
			name: "basic integration",
			resource: &api.ScimIntegration{
				Id:                1,
				Provider:          "okta",
				Enabled:           true,
				AuthToken:         "masked-token",
				GroupPrefixes:     []string{"engineering-"},
				UserGroupPrefixes: []string{"users-"},
				LastSyncedAt:      syncTime,
			},
			expected: ScimModel{
				Id:                types.StringValue("1"),
				ProviderName:      types.StringValue("okta"),
				Enabled:           types.BoolValue(true),
				GroupPrefixes:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("engineering-")}),
				UserGroupPrefixes: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("users-")}),
				LastSyncedAt:      types.StringValue("2025-01-15T10:30:00Z"),
			},
		},
		{
			name: "empty prefixes",
			resource: &api.ScimIntegration{
				Id:                2,
				Provider:          "azure",
				Enabled:           false,
				AuthToken:         "masked",
				GroupPrefixes:     []string{},
				UserGroupPrefixes: []string{},
				LastSyncedAt:      syncTime,
			},
			expected: ScimModel{
				Id:                types.StringValue("2"),
				ProviderName:      types.StringValue("azure"),
				Enabled:           types.BoolValue(false),
				GroupPrefixes:     types.ListValueMust(types.StringType, []attr.Value{}),
				UserGroupPrefixes: types.ListValueMust(types.StringType, []attr.Value{}),
				LastSyncedAt:      types.StringValue("2025-01-15T10:30:00Z"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out ScimModel
			// Preserve write-only fields
			out.AuthToken = types.StringValue("preserved-token")
			out.Prefix = types.StringValue("preserved-prefix")

			outDiag := scimAPIToTerraform(context.Background(), c.resource, &out)
			if outDiag.HasError() {
				t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
			}

			// auth_token and prefix should be preserved from existing state
			c.expected.AuthToken = types.StringValue("preserved-token")
			c.expected.Prefix = types.StringValue("preserved-prefix")

			if !reflect.DeepEqual(out, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
			}
		})
	}
}

func Test_Scim_Create(t *testing.T) {
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
