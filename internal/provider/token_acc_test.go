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

func Test_Token_Create(t *testing.T) {
	rName := "t" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_token." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testTokenResource(rName, `user1`, `180`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "token"),
					resource.TestCheckResourceAttrSet(rNameFull, "expiration_date"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "expiration_days", "180"),
					resource.TestCheckResourceAttr(rNameFull, "user_id", "user1"),
					func(s *terraform.State) error {
						uID := s.RootModule().Resources[rNameFull].Primary.Attributes["user_id"]
						tID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						token, err := testClient().Tokens.Get(context.Background(), uID, tID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name": {rName, token.Name},
						})
					},
				),
			},
		},
	})
}

func testTokenResource(rName, userID, expiryDays string) string {
	return fmt.Sprintf(`resource "netbird_token" "%s" {
  user_id         = "%s"
  name            = "%s"
  expiration_days = %s
}
`, rName, userID, rName, expiryDays)
}
