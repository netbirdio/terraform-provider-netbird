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

func Test_Groups_DataSource(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	ds := "data.netbird_groups." + rName
	res := "netbird_group." + rName

	// Filtered by the name of a group the same configuration creates: exactly that group.
	byName := fmt.Sprintf(`
data "netbird_groups" "%[1]s" {
  name = netbird_group.%[1]s.name
}
`, rName)
	dsCase(t, testGroupResource(rName, `[]`)+byName, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "ids.#", "1"),
		resource.TestCheckResourceAttrPair(ds, "ids.0", res, "id"),
		resource.TestCheckResourceAttrPair(ds, "groups.0.id", res, "id"),
		resource.TestCheckResourceAttrPair(ds, "groups.0.name", res, "name"),
		resource.TestCheckResourceAttr(ds, "groups.0.issued", "api"),
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
	dsCase(t, byIssuer, func(s *terraform.State) error {
		groups, err := testClient().Groups.List(context.Background())
		if err != nil {
			return fmt.Errorf("list groups: %w", err)
		}
		want := 0
		for _, g := range groups {
			if g.Issued != nil && string(*g.Issued) == "api" {
				want++
			}
		}
		got := s.RootModule().Resources[ds].Primary.Attributes["ids.#"]
		if got != fmt.Sprint(want) {
			return fmt.Errorf("expected %d api groups, found %s", want, got)
		}
		return nil
	})
}
