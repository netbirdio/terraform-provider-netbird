//go:build e2e

package provider

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_SetupKey_Create(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().SetupKeys.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `1800`, `reusable`, `false`, `[]`, `false`, `false`, `0`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
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
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
				// Neither of these can be read back. The plaintext key is
				// returned once, at creation. expiry_seconds is never returned
				// at all: the API reports an absolute expiry date and no
				// creation date, so the lifetime the key was created with
				// cannot be recovered from it.
				ImportStateVerifyIgnore: []string{"key", "expiry_seconds"},
			},
		},
	})
}

func Test_SetupKey_Update_Groups(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().SetupKeys.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `1800`, `one-off`, `true`, `[]`, `true`, `false`, `1`),
				Check:        resource.TestCheckResourceAttrSet(rNameFull, "id"),
			},
			{
				ResourceName: rName,
				Config:       testSetupKeyResource(rName, `1800`, `one-off`, `true`, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `true`, `false`, `1`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "expires"),
					resource.TestCheckResourceAttrSet(rNameFull, "key"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "expiry_seconds", "1800"),
					resource.TestCheckResourceAttr(rNameFull, "type", "one-off"),
					resource.TestCheckResourceAttr(rNameFull, "allow_extra_dns_labels", "true"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.0", e2eGroupNotAllID()),
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
							"auto_groups.0":          {e2eGroupNotAllID(), sk.AutoGroups[0]},
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

// Test_SetupKey_Update_Groups_AddRemove covers what a user actually does to a
// setup key after creating it: attach groups, attach more, take some away, take
// them all away. The pre-existing group test only ever went from no groups to
// one group, so nothing asserted that removal reaches the server, that more than
// one group is handled, or — the part that was reported broken — that any of it
// is an update rather than a replacement.
//
// The key here deliberately leaves usage_limit unset, which is the only way to
// write a one-off key: the server fixes the limit of a one-off key at 1 and
// ignores what the request asked for. Recording the ID up front and checking it
// after every step is what distinguishes an update from a destroy-and-recreate,
// and a recreated setup key is a different secret — every peer still holding the
// old one can no longer register.
func Test_SetupKey_Update_Groups_AddRemove(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	var createdID string
	// Same ID, asserted at every step after the first.
	sameKey := func() resource.TestCheckFunc {
		return resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID)
	}
	groupsAre := func(want ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			sk, err := testClient().SetupKeys.Get(context.Background(), createdID)
			if err != nil {
				return err
			}
			if len(sk.AutoGroups) != len(want) {
				return fmt.Errorf("management has %d auto groups, expected %d", len(sk.AutoGroups), len(want))
			}
			for _, id := range want {
				if !slices.Contains(sk.AutoGroups, id) {
					return fmt.Errorf("group %s missing from auto groups %v", id, sk.AutoGroups)
				}
			}
			return nil
		}
	}
	all, notAll := e2eGroupAllID(), e2eGroupNotAllID()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().SetupKeys.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: testSetupKeyResourceNoLimit(rName, `one-off`, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "0"),
					// The server pins a one-off key to a single use whatever
					// the request said, so this is the server's value, not a
					// configured one.
					resource.TestCheckResourceAttr(rNameFull, "usage_limit", "1"),
				),
			},
			{
				Config: testSetupKeyResourceNoLimit(rName, `one-off`, fmt.Sprintf("[%q]", notAll)),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.0", notAll),
					groupsAre(notAll),
				),
			},
			{
				Config: testSetupKeyResourceNoLimit(rName, `one-off`, fmt.Sprintf("[%q, %q]", notAll, all)),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "2"),
					groupsAre(notAll, all),
				),
			},
			{
				Config: testSetupKeyResourceNoLimit(rName, `one-off`, fmt.Sprintf("[%q]", all)),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.0", all),
					groupsAre(all),
				),
			},
			{
				Config: testSetupKeyResourceNoLimit(rName, `one-off`, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "0"),
					groupsAre(),
				),
			},
		},
	})
}

func Test_SetupKey_Update_Revoke(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().SetupKeys.Get, &createdID),
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
					testRecordID(rNameFull, &createdID),
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

// testSetupKeyResourceNoLimit omits usage_limit, and with it the optional flags
// the reported problem does not involve. A one-off key has to be written this
// way: the server decides its usage limit, so naming one in the configuration
// either contradicts the server or repeats it. The expiry is fixed at half an
// hour because none of these tests are about expiry.
func testSetupKeyResourceNoLimit(rName, skType, groups string) string {
	return fmt.Sprintf(`resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "%[2]s"
  auto_groups    = %[3]s
}
`, rName, skType, groups)
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
