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

// Test_Account_Update_KeepsUnmodeledSettings sets settings the resource does
// not model through the API, then applies a change to a modeled one. The
// server rebuilds the settings from the update request, so the apply keeps
// them only if the provider carries them into the request.
func Test_Account_Update_KeepsUnmodeledSettings(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	t.Cleanup(func() {
		if err := setUnmodeledAccountSettings(env.AccountID, false); err != nil {
			t.Errorf("could not reset the unmodeled settings: %v", err)
		}
	})
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccountResourceWithJWT(rName, true),
				Check:  resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "true"),
			},
			{
				PreConfig: func() {
					if err := setUnmodeledAccountSettings(env.AccountID, true); err != nil {
						t.Fatalf("could not set the unmodeled settings: %v", err)
					}
				},
				Config:           testAccountResourceWithJWT(rName, false),
				ConfigPlanChecks: updatesInPlace(rNameFull),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "jwt_groups_enabled", "false"),
					func(s *terraform.State) error {
						settings, err := testAccountSettings(env.AccountID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"jwt_groups_enabled":   {false, valOr(settings.JwtGroupsEnabled, true)},
							"metrics_push_enabled": {true, valOr(settings.MetricsPushEnabled, false)},
							"auto_update_always":   {true, valOr(settings.AutoUpdateAlways, false)},
						})
					},
				),
			},
		},
	})
}

// testAccountSettings reads the account's settings from the server, by ID.
func testAccountSettings(accountID string) (api.AccountSettings, error) {
	accounts, err := testClient().Accounts.List(context.Background())
	if err != nil {
		return api.AccountSettings{}, err
	}
	idx := slices.IndexFunc(accounts, func(a api.Account) bool { return a.Id == accountID })
	if idx < 0 {
		return api.AccountSettings{}, fmt.Errorf("account %s is not among the %d accounts on the management server", accountID, len(accounts))
	}
	return accounts[idx].Settings, nil
}

// setUnmodeledAccountSettings sets metrics_push_enabled and auto_update_always
// through the API. It sends the account's current settings with only those two
// changed, because the server resets any field an update request leaves out.
func setUnmodeledAccountSettings(accountID string, enabled bool) error {
	settings, err := testAccountSettings(accountID)
	if err != nil {
		return err
	}
	settings.MetricsPushEnabled = &enabled
	settings.AutoUpdateAlways = &enabled
	_, err = testClient().Accounts.Update(context.Background(), accountID, api.AccountRequest{Settings: settings})
	return err
}

func testAccountResource(rName string) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {}`, rName)
}

func testAccountResourceWithJWT(rName string, enabled bool) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {
jwt_groups_enabled = %v
}`, rName, enabled)
}
