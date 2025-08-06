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

func Test_setupKeyAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.SetupKey
		expected SetupKeyModel
	}{
		{
			resource: &api.SetupKey{
				Id:                  "r1",
				AllowExtraDnsLabels: true,
				AutoGroups:          []string{"g1"},
				Ephemeral:           true,
				Expires:             timeNow,
				Key:                 "abc",
				LastUsed:            timeNow,
				Name:                "sk",
				Revoked:             true,
				State:               "active",
				Type:                "reusable",
				UpdatedAt:           timeNow,
				UsageLimit:          0,
				UsedTimes:           1,
				Valid:               true,
			},
			expected: SetupKeyModel{
				Id:                  types.StringValue("r1"),
				Key:                 types.StringNull(), // Key is added only in Create flow, so it's outside this method's scope
				Name:                types.StringValue("sk"),
				State:               types.StringValue("active"),
				Type:                types.StringValue("reusable"),
				AllowExtraDnsLabels: types.BoolValue(true),
				Ephemeral:           types.BoolValue(true),
				Revoked:             types.BoolValue(true),
				Valid:               types.BoolValue(true),
				Expires:             types.StringValue(timeNow.Format(time.RFC3339)),
				LastUsed:            types.StringValue(timeNow.Format(time.RFC3339)),
				UpdatedAt:           types.StringValue(timeNow.Format(time.RFC3339)),
				UsageLimit:          types.Int32Value(0),
				UsedTimes:           types.Int32Value(1),
				AutoGroups:          types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
			},
		},
	}

	for _, c := range cases {
		var out SetupKeyModel
		outDiag := setupKeyAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_SetupKey_Create(t *testing.T) {
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `1800`, `reusable`, `false`, `[]`, `false`, `false`, `0`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "expires"),
					resource.TestCheckResourceAttrSet(rNameFull, "key"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "expiry_seconds", "1800"),
					resource.TestCheckResourceAttr(rNameFull, "type", "reusable"),
					resource.TestCheckResourceAttr(rNameFull, "allow_extra_dns_labels", "false"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "ephemeral", "false"),
					resource.TestCheckResourceAttr(rNameFull, "revoked", "false"),
					resource.TestCheckResourceAttr(rNameFull, "usage_limit", "0"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						sk, err := testClient().SetupKeys.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":                   {rName, sk.Name},
							"type":                   {"reusable", sk.Type},
							"allow_extra_dns_labels": {false, sk.AllowExtraDnsLabels},
							"auto_groups.#":          {int(0), len(sk.AutoGroups)},
							"ephemeral":              {false, sk.Ephemeral},
							"revoked":                {false, sk.Revoked},
							"usage_limit":            {int(0), sk.UsageLimit},
						})
					},
				),
			},
		},
	})
}

func Test_SetupKey_Update_Groups(t *testing.T) {
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `1800`, `one-off`, `true`, `[]`, `true`, `false`, `1`),
				Check:        resource.TestCheckResourceAttrSet(rNameFull, "id"),
			},
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `1800`, `one-off`, `true`, `["group-notall"]`, `true`, `false`, `1`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "expires"),
					resource.TestCheckResourceAttrSet(rNameFull, "key"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "expiry_seconds", "1800"),
					resource.TestCheckResourceAttr(rNameFull, "type", "one-off"),
					resource.TestCheckResourceAttr(rNameFull, "allow_extra_dns_labels", "true"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.0", "group-notall"),
					resource.TestCheckResourceAttr(rNameFull, "ephemeral", "true"),
					resource.TestCheckResourceAttr(rNameFull, "revoked", "false"),
					resource.TestCheckResourceAttr(rNameFull, "usage_limit", "1"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						sk, err := testClient().SetupKeys.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":                   {rName, sk.Name},
							"type":                   {"one-off", sk.Type},
							"allow_extra_dns_labels": {true, sk.AllowExtraDnsLabels},
							"auto_groups.#":          {int(1), len(sk.AutoGroups)},
							"auto_groups.0":          {"group-notall", sk.AutoGroups[0]},
							"ephemeral":              {true, sk.Ephemeral},
							"revoked":                {false, sk.Revoked},
							"usage_limit":            {int(1), sk.UsageLimit},
						})
					},
				),
			},
		},
	})
}

func Test_SetupKey_Update_Revoke(t *testing.T) {
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `3600`, `reusable`, `false`, `[]`, `false`, `false`, `10`),
				Check:        resource.TestCheckResourceAttrSet(rNameFull, "id"),
			},
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `3600`, `reusable`, `false`, `[]`, `false`, `true`, `10`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "expires"),
					resource.TestCheckResourceAttrSet(rNameFull, "key"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "expiry_seconds", "3600"),
					resource.TestCheckResourceAttr(rNameFull, "type", "reusable"),
					resource.TestCheckResourceAttr(rNameFull, "allow_extra_dns_labels", "false"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "0"),
					resource.TestCheckResourceAttr(rNameFull, "ephemeral", "false"),
					resource.TestCheckResourceAttr(rNameFull, "revoked", "true"),
					resource.TestCheckResourceAttr(rNameFull, "usage_limit", "10"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						sk, err := testClient().SetupKeys.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":                   {rName, sk.Name},
							"type":                   {"reusable", sk.Type},
							"allow_extra_dns_labels": {false, sk.AllowExtraDnsLabels},
							"auto_groups.#":          {int(0), len(sk.AutoGroups)},
							"ephemeral":              {false, sk.Ephemeral},
							"revoked":                {true, sk.Revoked},
							"usage_limit":            {int(10), sk.UsageLimit},
						})
					},
				),
			},
		},
	})
}

func testSetupKeyResource(rName, expiry, skType, allowExtraDNS, groups, ephemeral, revoked, usageLimit string) string {
	return fmt.Sprintf(`resource "netbird_setup_key" "%s" {
  name                   = "%s"
  expiry_seconds         = %s
  type                   = "%s"
  allow_extra_dns_labels = %s
  auto_groups            = %s
  ephemeral              = %s
  revoked                = %s
  usage_limit            = %s
}
`, rName, rName, expiry, skType, allowExtraDNS, groups, ephemeral, revoked, usageLimit)
}
