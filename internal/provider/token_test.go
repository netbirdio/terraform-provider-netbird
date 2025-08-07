package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_tokenAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.PersonalAccessToken
		expected TokenModel
	}{
		{
			resource: &api.PersonalAccessToken{
				Id:             "r1",
				CreatedAt:      timeNow,
				ExpirationDate: timeNow,
				LastUsed:       &timeNow,
				Name:           "test",
			},
			expected: TokenModel{
				Id:             types.StringValue("r1"),
				UserID:         types.StringNull(),
				Name:           types.StringValue("test"),
				CreatedAt:      types.StringValue(timeNow.Format(time.RFC3339)),
				ExpirationDate: types.StringValue(timeNow.Format(time.RFC3339)),
				LastUsed:       types.StringValue(timeNow.Format(time.RFC3339)),
			},
		},
	}

	for _, c := range cases {
		var out TokenModel
		tokenAPIToTerraform(c.resource, &out)

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

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
