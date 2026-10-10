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

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_Account_Create(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testAccountResource(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", env.AccountID),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "false"),
					func(s *terraform.State) error {
						accounts, err := testClient().Accounts.List(context.Background())
						if err != nil {
							return err
						}
						if len(accounts) == 0 {
							return fmt.Errorf("no accounts on the management server")
						}
						// By ID rather than by position: the assertion above is
						// about env.AccountID, so comparing settings from
						// whichever account happened to be listed first would
						// check a different account than the resource manages.
						idx := slices.IndexFunc(accounts, func(a api.Account) bool { return a.Id == env.AccountID })
						if idx < 0 {
							return fmt.Errorf("account %s is not among the %d accounts on the management server", env.AccountID, len(accounts))
						}
						settings := accounts[idx].Settings
						attrs := s.RootModule().Resources[rNameFull].Primary.Attributes
						return matchPairs(map[string][]any{
							"peer_login_expiration":              {attrs["peer_login_expiration"], fmt.Sprint(settings.PeerLoginExpiration)},
							"peer_inactivity_expiration":         {attrs["peer_inactivity_expiration"], fmt.Sprint(settings.PeerInactivityExpiration)},
							"peer_login_expiration_enabled":      {attrs["peer_login_expiration_enabled"], fmt.Sprint(settings.PeerLoginExpirationEnabled)},
							"peer_inactivity_expiration_enabled": {attrs["peer_inactivity_expiration_enabled"], fmt.Sprint(settings.PeerInactivityExpirationEnabled)},
							"regular_users_view_blocked":         {attrs["regular_users_view_blocked"], fmt.Sprint(settings.RegularUsersViewBlocked)},
							"groups_propagation_enabled":         {attrs["groups_propagation_enabled"], fmt.Sprint(valOr(settings.GroupsPropagationEnabled, false))},
							"jwt_groups_enabled":                 {attrs["jwt_groups_enabled"], fmt.Sprint(valOr(settings.JwtGroupsEnabled, false))},
							"routing_peer_dns_resolution_enabled": {attrs["routing_peer_dns_resolution_enabled"],
								fmt.Sprint(valOr(settings.RoutingPeerDnsResolutionEnabled, false))},
						})
					},
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func Test_Account_Update(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testAccountResourceWithJWT(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", env.AccountID),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "true"),
				),
				Destroy: false,
			},
			{
				ResourceName: rName,
				Config:       testAccountResourceWithJWT(rName, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", env.AccountID),
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "false"),
				),
			},
		},
	})
}

func testAccountResource(rName string) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {}`, rName)
}

func testAccountResourceWithJWT(rName string, enabled bool) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {
jwt_groups_enabled = %v
}`, rName, enabled)
}

func Test_Account_IPv6Overlay(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	gName := "grp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccountResourceWithIPv6(rName, gName, "fd00:1234:5678::/64"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", env.AccountID),
					resource.TestCheckResourceAttr(rNameFull, "network_range_v6", "fd00:1234:5678::/64"),
					resource.TestCheckResourceAttr(rNameFull, "ipv6_enabled_groups.#", "1"),
					resource.TestCheckResourceAttrPair(rNameFull, "ipv6_enabled_groups.0", "netbird_group."+gName, "id"),
					testCheckAccountIPv6(env.AccountID, "fd00:1234:5678::/64", 1),
				),
				Destroy: false,
			},
			{
				// A changed prefix has to reach the API: the range is Computed,
				// so a mapping that dropped the configured value would leave the
				// old prefix in place and still look consistent in state.
				Config: testAccountResourceWithIPv6(rName, gName, "fd00:1234:5678:9::/80"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "network_range_v6", "fd00:1234:5678:9::/80"),
					testCheckAccountIPv6(env.AccountID, "fd00:1234:5678:9::/80", 1),
				),
				Destroy: false,
			},
			{
				// Emptying the group list turns the overlay off without clearing the range.
				Config: testAccountResourceWithIPv6Groups(rName, gName, "fd00:1234:5678:9::/80", "[]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "ipv6_enabled_groups.#", "0"),
					testCheckAccountIPv6(env.AccountID, "fd00:1234:5678:9::/80", 0),
				),
			},
		},
	})
}

// Test_Account_IPv6SurvivesUnrelatedUpdate covers settings the provider does not
// manage. Management rebuilds the settings object from the request and saves it
// wholesale, so an attribute missing from the request is cleared rather than left
// alone. The v6 fields must therefore be carried forward from the current account
// whenever the configuration omits them.
func Test_Account_IPv6SurvivesUnrelatedUpdate(t *testing.T) {
	env := testE2E(t)
	ctx := context.Background()
	client := testClient()

	group, err := client.Groups.Create(ctx, api.PostApiGroupsJSONRequestBody{
		Name: "ipv6-carry-" + acctest.RandStringFromCharSet(6, acctest.CharSetAlphaNum),
	})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Enable the overlay outside Terraform, the way the dashboard or a direct API
	// call would.
	settings := testAccountSettings(t, env.AccountID)
	settings.NetworkRangeV6 = valPtr("fd00:dead:beef::/64")
	settings.Ipv6EnabledGroups = &[]string{group.Id}
	if _, err := client.Accounts.Update(ctx, env.AccountID, api.AccountRequest{Settings: settings}); err != nil {
		t.Fatalf("enable ipv6 out of band: %v", err)
	}

	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The configuration says nothing about IPv6.
				Config: testAccountResourceWithJWT(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "true"),
					testCheckAccountIPv6(env.AccountID, "fd00:dead:beef::/64", 1),
				),
			},
		},
	})
}

// testAccountSettings returns the current settings of the account under test.
func testAccountSettings(t *testing.T, accountID string) api.AccountSettings {
	t.Helper()
	accounts, err := testClient().Accounts.List(context.Background())
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	idx := slices.IndexFunc(accounts, func(a api.Account) bool { return a.Id == accountID })
	if idx < 0 {
		t.Fatalf("account %s is not among the %d accounts on the management server", accountID, len(accounts))
	}
	return accounts[idx].Settings
}

// testCheckAccountIPv6 asserts the overlay settings management actually stored,
// which is what state alone cannot show.
func testCheckAccountIPv6(accountID, wantRange string, wantGroups int) resource.TestCheckFunc {
	return func(*terraform.State) error {
		accounts, err := testClient().Accounts.List(context.Background())
		if err != nil {
			return err
		}
		idx := slices.IndexFunc(accounts, func(a api.Account) bool { return a.Id == accountID })
		if idx < 0 {
			return fmt.Errorf("account %s is not among the %d accounts on the management server", accountID, len(accounts))
		}
		settings := accounts[idx].Settings
		gotRange := valOr(settings.NetworkRangeV6, "")
		gotGroups := valOr(settings.Ipv6EnabledGroups, nil)
		return matchPairs(map[string][]any{
			"network_range_v6":      {gotRange, wantRange},
			"ipv6_enabled_groups.#": {fmt.Sprint(len(gotGroups)), fmt.Sprint(wantGroups)},
		})
	}
}

func testAccountResourceWithIPv6(rName, gName, rangeV6 string) string {
	return testAccountResourceWithIPv6Groups(rName, gName, rangeV6, fmt.Sprintf("[netbird_group.%s.id]", gName))
}

func testAccountResourceWithIPv6Groups(rName, gName, rangeV6, groups string) string {
	return fmt.Sprintf(`resource "netbird_group" "%s" {
	name = "%s"
}

resource "netbird_account_settings" "%s" {
	network_range_v6    = %q
	ipv6_enabled_groups = %s
}`, gName, gName, rName, rangeV6, groups)
}
