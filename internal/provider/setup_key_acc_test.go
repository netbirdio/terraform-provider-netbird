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
// Each step requires Terraform to have planned an in-place update, which is the
// assertion that reports the reported bug: a replaced setup key is a different
// secret, so every peer still holding the old one stops being able to register.
// The ID is checked as well, since a plan can be right and the apply still wrong.
//
// The key deliberately leaves usage_limit unset, which is the only way to write
// a one-off key: the server fixes the limit of a one-off key at 1 and ignores
// what the request asked for.
func Test_SetupKey_Update_Groups_AddRemove(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	groupA, groupB := "netbird_group."+rName+"a", "netbird_group."+rName+"b"
	var createdID string
	// Same ID, asserted at every step after the first.
	sameKey := func() resource.TestCheckFunc {
		return resource.TestCheckResourceAttrPtr(rNameFull, "id", &createdID)
	}
	// groupsAre reads the wanted IDs out of state rather than taking them as
	// literals, because the groups are created by the same configuration and
	// their IDs are not known until it has been applied.
	groupsAre := func(addresses ...string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			want := make([]string, 0, len(addresses))
			for _, a := range addresses {
				rs, ok := s.RootModule().Resources[a]
				if !ok {
					return fmt.Errorf("%s is not in state", a)
				}
				want = append(want, rs.Primary.Attributes["id"])
			}
			sk, err := testClient().SetupKeys.Get(context.Background(), createdID)
			if err != nil {
				return err
			}
			if len(sk.AutoGroups) != len(want) {
				return fmt.Errorf("management has %d auto groups %v, expected %d", len(sk.AutoGroups), sk.AutoGroups, len(want))
			}
			for _, id := range want {
				if !slices.Contains(sk.AutoGroups, id) {
					return fmt.Errorf("group %s missing from auto groups %v", id, sk.AutoGroups)
				}
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().SetupKeys.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: testSetupKeyWithGroups(rName, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "0"),
					// The server pins a one-off key to a single use whatever
					// the request said, so this is the server's value rather
					// than a configured one.
					resource.TestCheckResourceAttr(rNameFull, "usage_limit", "1"),
				),
			},
			{
				Config:           testSetupKeyWithGroups(rName, fmt.Sprintf("[%s.id]", groupA)),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "auto_groups.0", groupA, "id"),
					groupsAre(groupA),
				),
			},
			{
				Config:           testSetupKeyWithGroups(rName, fmt.Sprintf("[%s.id, %s.id]", groupA, groupB)),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "2"),
					groupsAre(groupA, groupB),
				),
			},
			{
				Config:           testSetupKeyWithGroups(rName, fmt.Sprintf("[%s.id]", groupB)),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "auto_groups.0", groupB, "id"),
					groupsAre(groupB),
				),
			},
			{
				Config:           testSetupKeyWithGroups(rName, `[]`),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameKey(),
					resource.TestCheckResourceAttr(rNameFull, "auto_groups.#", "0"),
					groupsAre(),
				),
			},
		},
	})
}

// testSetupKeyWithGroups builds a one-off key alongside the two groups it
// attaches. They are created here rather than taken from the fixtures because
// the account's "All" group cannot be attached to a setup key at all — the
// server refuses it — which leaves only one fixture group to work with.
func testSetupKeyWithGroups(rName, groups string) string {
	return fmt.Sprintf(`resource "netbird_group" "%[1]sa" {
  name  = "%[1]sa"
  peers = []
}

resource "netbird_group" "%[1]sb" {
  name  = "%[1]sb"
  peers = []
}

resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "one-off"
  auto_groups    = %[2]s
}
`, rName, groups)
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
// and groups the test using it does not involve. A one-off key has to be
// written this way: the server decides its usage limit, so naming one in the
// configuration either contradicts the server or repeats it.
func testSetupKeyResourceNoLimit(rName, skType string) string {
	return fmt.Sprintf(`resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "%[2]s"
  auto_groups    = []
}
`, rName, skType)
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
