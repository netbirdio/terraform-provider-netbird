//go:build e2e

package provider

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

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
							"auto_update_always": {attrs["auto_update_always"], fmt.Sprint(valOr(settings.AutoUpdateAlways, false))},
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

func Test_Account_AutoUpdateAlways(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	dataSourceName := "data.netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		// Account settings are a singleton with a no-op Delete. Verify that
		// destroying the Terraform resource preserves the account and final setting.
		CheckDestroy: testCheckAccountAutoUpdateAlways(env.AccountID, false),
		Steps: []resource.TestStep{
			{
				Config: testAccountResourceWithAutoUpdateAlways(rName, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", env.AccountID),
					resource.TestCheckResourceAttr(rNameFull, "auto_update_always", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "auto_update_always", "true"),
					testCheckAccountAutoUpdateAlways(env.AccountID, true),
				),
			},
			{
				Config: testAccountResourceWithAutoUpdateAlways(rName, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "id", env.AccountID),
					resource.TestCheckResourceAttr(rNameFull, "auto_update_always", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "auto_update_always", "false"),
					testCheckAccountAutoUpdateAlways(env.AccountID, false),
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

func Test_Account_AutoUpdateAlwaysAdoptsServerValue(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_account_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			if err := setAccountAutoUpdateAlways(env.AccountID, false); err != nil {
				t.Fatalf("set auto_update_always before test: %v", err)
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		AdditionalCLIOptions: &resource.AdditionalCLIOptions{
			Plan: resource.PlanOptions{NoRefresh: true},
		},
		// Account settings are a singleton with a no-op Delete. The final step
		// restores false, which must remain after the Terraform resource is destroyed.
		CheckDestroy: testCheckAccountAutoUpdateAlways(env.AccountID, false),
		Steps: []resource.TestStep{
			{
				Config: testAccountResourceWithJWT(rName, true),
				Check:  resource.TestCheckResourceAttr(rNameFull, "auto_update_always", "false"),
			},
			{
				PreConfig: func() {
					if err := setAccountAutoUpdateAlways(env.AccountID, true); err != nil {
						t.Fatalf("change auto_update_always outside Terraform: %v", err)
					}
				},
				Config: testAccountResourceWithJWT(rName, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue(rNameFull, tfjsonpath.New("auto_update_always")),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "auto_update_always", "true"),
					testCheckAccountAutoUpdateAlways(env.AccountID, true),
				),
			},
			{
				Config: testAccountResourceWithAutoUpdateAlways(rName, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "auto_update_always", "false"),
					testCheckAccountAutoUpdateAlways(env.AccountID, false),
				),
			},
		},
	})
}

func testCheckAccountAutoUpdateAlways(accountID string, expected bool) resource.TestCheckFunc {
	return func(*terraform.State) error {
		accounts, err := testClient().Accounts.List(context.Background())
		if err != nil {
			return err
		}
		idx := slices.IndexFunc(accounts, func(a api.Account) bool { return a.Id == accountID })
		if idx < 0 {
			return fmt.Errorf("account %s is not among the %d accounts on the management server", accountID, len(accounts))
		}
		actual := accounts[idx].Settings.AutoUpdateAlways
		if actual == nil {
			return fmt.Errorf("auto_update_always is null, expected %t", expected)
		}
		if *actual != expected {
			return fmt.Errorf("auto_update_always is %t, expected %t", *actual, expected)
		}
		return nil
	}
}

func setAccountAutoUpdateAlways(accountID string, value bool) error {
	accounts, err := testClient().Accounts.List(context.Background())
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	idx := slices.IndexFunc(accounts, func(a api.Account) bool { return a.Id == accountID })
	if idx < 0 {
		return fmt.Errorf("account %s is not among the %d accounts on the management server", accountID, len(accounts))
	}
	request := api.AccountRequest{Settings: accounts[idx].Settings}
	request.Settings.AutoUpdateAlways = &value
	if _, err := testClient().Accounts.Update(context.Background(), accountID, request); err != nil {
		return fmt.Errorf("update account settings: %w", err)
	}
	return nil
}

func testAccountResourceWithAutoUpdateAlways(rName string, enabled bool) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {
auto_update_always = %v
}`, rName, enabled) + fmt.Sprintf(`
data "netbird_account_settings" "%[1]s" {
depends_on = [netbird_account_settings.%[1]s]
}`, rName)
}

func testAccountResource(rName string) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {}`, rName)
}

func testAccountResourceWithJWT(rName string, enabled bool) string {
	return fmt.Sprintf(`resource "netbird_account_settings" "%s" {
jwt_groups_enabled = %v
}`, rName, enabled)
}
