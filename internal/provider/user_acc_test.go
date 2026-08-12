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

func Test_User_Create(t *testing.T) {
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_user." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testUserResource(rName, `true`, `["group-notall"]`, `false`, `user`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "is_service_user", "true"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.0", "group-notall"),
					resource.TestCheckResourceAttr(rNameFull, "is_blocked", "false"),
					resource.TestCheckResourceAttr(rNameFull, "role", "user"),
					func(s *terraform.State) error {
						uID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						users, err := testClient().Users.List(context.Background())
						if err != nil {
							return err
						}
						var user *api.User
						for _, u := range users {
							if u.Id == uID {
								user = &u
							}
						}
						if user == nil {
							return fmt.Errorf("User not found")
						}
						// IsServiceUser is an optional *bool, so a server that
						// omits it would panic the dereference below rather than
						// failing the assertion.
						if user.IsServiceUser == nil {
							return fmt.Errorf("User %s has no is_service_user on management server", uID)
						}
						return matchPairs(map[string][]any{
							"name":            {rName, user.Name},
							"is_service_user": {true, *user.IsServiceUser},
							"auto_groups.#":   {int(1), len(user.AutoGroups)},
							"auto_groups.0":   {"group-notall", user.AutoGroups[0]},
							"is_blocked":      {false, user.IsBlocked},
							"role":            {"user", user.Role},
						})
					},
				),
			},
		},
	})
}

func testUserResource(rName, serviceUser, groups, blocked, role string) string {
	return fmt.Sprintf(`resource "netbird_user" "%s" {
	name            = "%s"
  is_service_user = %s
  auto_groups     = %s
  is_blocked      = %s
  role            = "%s"
}`, rName, rName, serviceUser, groups, blocked, role)
}
