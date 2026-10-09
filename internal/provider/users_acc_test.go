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

// Test_Users_DataSource checks netbird_users filtering against a live NetBird.
func Test_Users_DataSource(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	ds := "data.netbird_users." + rName
	res := "netbird_user." + rName

	// testUserResource creates a service user: filtering on it and on its name finds only it.
	byName := fmt.Sprintf(`
data "netbird_users" "%[1]s" {
  name            = netbird_user.%[1]s.name
  is_service_user = true
}
`, rName)
	cfg := testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `user`)
	dsCase(t, cfg+byName, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "ids.#", "1"),
		resource.TestCheckResourceAttrPair(ds, "ids.0", res, "id"),
		resource.TestCheckResourceAttrPair(ds, "users.0.name", res, "name"),
		resource.TestCheckResourceAttrPair(ds, "users.0.role", res, "role"),
		resource.TestCheckResourceAttrPair(ds, "users.0.is_service_user", res, "is_service_user"),
		resource.TestCheckResourceAttrPair(ds, "users.0.auto_groups.#", res, "auto_groups.#"),
	))

	// An email no user has is an empty result, where netbird_user fails with No match.
	missing := fmt.Sprintf(`
data "netbird_users" "%[1]s" {
  email = "%[1]s@never-created.invalid"
}
`, rName)
	dsCase(t, missing, resource.TestCheckResourceAttr(ds, "ids.#", "0"))

	// A role filter returns every user the server holds with that role.
	byRole := fmt.Sprintf(`
data "netbird_users" "%s" {
  role = "owner"
}
`, rName)
	dsCase(t, byRole, func(s *terraform.State) error {
		users, err := testClient().Users.List(context.Background())
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		want := 0
		for _, u := range users {
			if u.Role == "owner" {
				want++
			}
		}
		if want == 0 {
			return fmt.Errorf("the deployment has no owner, the case tests nothing")
		}
		got := s.RootModule().Resources[ds].Primary.Attributes["ids.#"]
		if got != fmt.Sprint(want) {
			return fmt.Errorf("expected %d owners, found %s", want, got)
		}
		return nil
	})
}
