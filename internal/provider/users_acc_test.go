//go:build e2e

package provider

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Test_Users_DataSource checks netbird_users filtering against a live NetBird.
func Test_Users_DataSource(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	ds := "data.netbird_users." + rName

	// testUserResource creates a service user: every selector
	// it satisfies, combined, finds only it.
	allSelectors := fmt.Sprintf(`
data "netbird_users" "%[1]s" {
  name            = netbird_user.%[1]s.name
  role            = "user"
  status          = "active"
  issued          = "api"
  is_service_user = true
  is_blocked      = false
  auto_groups     = [%[2]q]
}
`, rName, e2eGroupNotAllID())
	cfg := testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `user`)
	dsCase(t, cfg+allSelectors, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "ids.#", "1"),
		resource.TestCheckResourceAttrPair(ds, "ids.0", "netbird_user."+rName, "id"),
		checkUsersAgainstAPI(ds, func(u api.User) bool { return u.Name == rName }),
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
	dsCase(t, byRole, checkUsersAgainstAPI(ds, func(u api.User) bool { return u.Role == "owner" }))
}

// checkUsersAgainstAPI asserts the data source returned exactly the API users
// keep selects, in API order, with every attribute mapped from the API value.
func checkUsersAgainstAPI(ds string, keep func(api.User) bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		users, err := testClient().Users.List(context.Background())
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}
		var want []api.User
		for _, u := range users {
			if keep(u) {
				want = append(want, u)
			}
		}
		if len(want) == 0 {
			return fmt.Errorf("no API user matches, the case tests nothing")
		}
		checks := []resource.TestCheckFunc{resource.TestCheckResourceAttr(ds, "ids.#", strconv.Itoa(len(want)))}
		for i, u := range want {
			p := fmt.Sprintf("users.%d.", i)
			var lastLogin *string
			if u.LastLogin != nil {
				v := u.LastLogin.Format(time.RFC3339)
				lastLogin = &v
			}
			checks = append(checks,
				resource.TestCheckResourceAttr(ds, fmt.Sprintf("ids.%d", i), u.Id),
				resource.TestCheckResourceAttr(ds, p+"id", u.Id),
				resource.TestCheckResourceAttr(ds, p+"name", u.Name),
				resource.TestCheckResourceAttr(ds, p+"email", u.Email),
				resource.TestCheckResourceAttr(ds, p+"role", u.Role),
				resource.TestCheckResourceAttr(ds, p+"status", string(u.Status)),
				resource.TestCheckResourceAttr(ds, p+"is_blocked", strconv.FormatBool(u.IsBlocked)),
				resource.TestCheckResourceAttr(ds, p+"auto_groups.#", strconv.Itoa(len(u.AutoGroups))),
				checkOptionalAttr(ds, p+"last_login", lastLogin),
				checkOptionalAttr(ds, p+"issued", u.Issued),
				checkOptionalAttr(ds, p+"is_current", boolString(u.IsCurrent)),
				checkOptionalAttr(ds, p+"is_service_user", boolString(u.IsServiceUser)),
			)
			for j, g := range u.AutoGroups {
				checks = append(checks, resource.TestCheckResourceAttr(ds, fmt.Sprintf("%sauto_groups.%d", p, j), g))
			}
		}
		return resource.ComposeAggregateTestCheckFunc(checks...)(s)
	}
}

// checkOptionalAttr expects key to equal *want, or to be null when want is nil.
func checkOptionalAttr(ds, key string, want *string) resource.TestCheckFunc {
	if want == nil {
		return resource.TestCheckNoResourceAttr(ds, key)
	}
	return resource.TestCheckResourceAttr(ds, key, *want)
}

// boolString formats an optional bool, keeping nil as nil.
func boolString(b *bool) *string {
	if b == nil {
		return nil
	}
	v := strconv.FormatBool(*b)
	return &v
}
