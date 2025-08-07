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

func Test_userAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.User
		expected UserModel
	}{
		{
			resource: &api.User{
				Id:            "r1",
				AutoGroups:    []string{"g1"},
				Name:          "sk",
				LastLogin:     &timeNow,
				Email:         "me@me.com",
				IsBlocked:     true,
				IsCurrent:     valPtr(true),
				IsServiceUser: valPtr(true),
				Issued:        valPtr("api"),
				Role:          "admin",
				Status:        api.UserStatusActive,
			},
			expected: UserModel{
				Id:            types.StringValue("r1"),
				Name:          types.StringValue("sk"),
				AutoGroups:    types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				LastLogin:     types.StringValue(timeNow.Format(time.RFC3339)),
				IsBlocked:     types.BoolValue(true),
				IsCurrent:     types.BoolValue(true),
				IsServiceUser: types.BoolValue(true),
				Issued:        types.StringValue("api"),
				Email:         types.StringValue("me@me.com"),
				Role:          types.StringValue("admin"),
				Status:        types.StringValue(string(api.UserStatusActive)),
			},
		},
	}

	for _, c := range cases {
		var out UserModel
		outDiag := userAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

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
