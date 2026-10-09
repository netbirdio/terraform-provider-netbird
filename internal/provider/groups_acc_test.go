//go:build e2e

package provider

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Test_Groups_DataSource checks netbird_groups filtering against a live NetBird.
func Test_Groups_DataSource(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	ds := "data.netbird_groups." + rName

	// Filtered by the name and issuer of a group the same configuration creates: exactly that group.
	byName := fmt.Sprintf(`
data "netbird_groups" "%[1]s" {
  name   = netbird_group.%[1]s.name
  issued = "api"
}
`, rName)
	dsCase(t, testGroupResource(rName, `[]`)+byName, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "ids.#", "1"),
		resource.TestCheckResourceAttrPair(ds, "ids.0", "netbird_group."+rName, "id"),
		checkGroupsAgainstAPI(ds, func(g api.Group) bool { return g.Name == rName }),
	))

	// A name no group has is an empty result, where netbird_group fails with No match.
	missing := fmt.Sprintf(`
data "netbird_groups" "%[1]s" {
  name = "%[1]s-never-created"
}
`, rName)
	dsCase(t, missing, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "ids.#", "0"),
		resource.TestCheckResourceAttr(ds, "groups.#", "0"),
	))

	// An issuer filter returns every group the server holds with that issuer.
	byIssuer := fmt.Sprintf(`
data "netbird_groups" "%s" {
  issued = "api"
}
`, rName)
	dsCase(t, byIssuer, checkGroupsAgainstAPI(ds, func(g api.Group) bool {
		return g.Issued != nil && *g.Issued == api.GroupIssuedApi
	}))
}

// checkGroupsAgainstAPI asserts the data source returned exactly the API groups
// keep selects, in API order, with every attribute mapped from the API value.
func checkGroupsAgainstAPI(ds string, keep func(api.Group) bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		groups, err := testClient().Groups.List(context.Background())
		if err != nil {
			return fmt.Errorf("list groups: %w", err)
		}
		var want []api.Group
		for _, g := range groups {
			if keep(g) {
				want = append(want, g)
			}
		}
		if len(want) == 0 {
			return fmt.Errorf("no API group matches, the case tests nothing")
		}
		checks := []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(ds, "ids.#", strconv.Itoa(len(want))),
			resource.TestCheckResourceAttr(ds, "groups.#", strconv.Itoa(len(want))),
		}
		for i, g := range want {
			p := fmt.Sprintf("groups.%d.", i)
			checks = append(checks,
				resource.TestCheckResourceAttr(ds, fmt.Sprintf("ids.%d", i), g.Id),
				resource.TestCheckResourceAttr(ds, p+"id", g.Id),
				resource.TestCheckResourceAttr(ds, p+"name", g.Name),
				checkOptionalAttr(ds, p+"issued", (*string)(g.Issued)),
				resource.TestCheckResourceAttr(ds, p+"peers.#", strconv.Itoa(len(g.Peers))),
				resource.TestCheckResourceAttr(ds, p+"resources.#", strconv.Itoa(len(g.Resources))),
			)
			for j, peer := range g.Peers {
				checks = append(checks, resource.TestCheckResourceAttr(ds, fmt.Sprintf("%speers.%d", p, j), peer.Id))
			}
			for j, r := range g.Resources {
				checks = append(checks, resource.TestCheckResourceAttr(ds, fmt.Sprintf("%sresources.%d", p, j), r.Id))
			}
		}
		return resource.ComposeAggregateTestCheckFunc(checks...)(s)
	}
}
