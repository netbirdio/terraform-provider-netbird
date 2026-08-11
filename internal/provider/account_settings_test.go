package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_accountAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Account
		expected AccountSettingsModel
	}{
		{
			resource: &api.Account{
				Id: "a",
				Settings: api.AccountSettings{
					AutoUpdateVersion:               nil,
					DnsDomain:                       nil,
					NetworkRange:                    nil,
					LazyConnectionEnabled:           nil,
					GroupsPropagationEnabled:        nil,
					JwtAllowGroups:                  nil,
					JwtGroupsClaimName:              nil,
					JwtGroupsEnabled:                nil,
					PeerInactivityExpiration:        1800,
					PeerInactivityExpirationEnabled: false,
					PeerLoginExpiration:             1800,
					PeerLoginExpirationEnabled:      false,
					RegularUsersViewBlocked:         false,
					RoutingPeerDnsResolutionEnabled: nil,
					PeerExposeEnabled:               false,
					PeerExposeGroups:                nil,
					Extra: &api.AccountExtraSettings{
						NetworkTrafficLogsEnabled:          false,
						NetworkTrafficPacketCounterEnabled: false,
						PeerApprovalEnabled:                false,
						UserApprovalRequired:               false,
						NetworkTrafficLogsGroups:           nil,
					},
				},
			},
			expected: AccountSettingsModel{
				Id:                                 types.StringValue("a"),
				JwtAllowGroups:                     types.ListNull(types.StringType),
				JwtGroupsClaimName:                 types.StringNull(),
				PeerLoginExpiration:                types.Int32Value(1800),
				PeerInactivityExpiration:           types.Int32Value(1800),
				PeerLoginExpirationEnabled:         types.BoolValue(false),
				PeerInactivityExpirationEnabled:    types.BoolValue(false),
				RegularUsersViewBlocked:            types.BoolValue(false),
				GroupsPropagationEnabled:           types.BoolNull(),
				JwtGroupsEnabled:                   types.BoolNull(),
				RoutingPeerDnsResolutionEnabled:    types.BoolNull(),
				PeerApprovalEnabled:                types.BoolValue(false),
				NetworkTrafficLogsEnabled:          types.BoolValue(false),
				NetworkTrafficPacketCounterEnabled: types.BoolValue(false),
				AutoUpdateVersion:                  types.StringNull(),
				DnsDomain:                          types.StringNull(),
				NetworkRange:                       types.StringNull(),
				LazyConnectionEnabled:              types.BoolNull(),
				UserApprovalRequired:               types.BoolValue(false),
				NetworkTrafficLogsGroups:           types.ListNull(types.StringType),
				PeerExposeEnabled:                  types.BoolValue(false),
				PeerExposeGroups:                   types.ListNull(types.StringType),
			},
		},
		{
			resource: &api.Account{
				Id: "b",
				Settings: api.AccountSettings{
					AutoUpdateVersion:               valPtr("latest"),
					DnsDomain:                       valPtr("custom.com"),
					NetworkRange:                    valPtr("100.64.0.0/10"),
					LazyConnectionEnabled:           valPtr(true),
					GroupsPropagationEnabled:        valPtr(true),
					JwtAllowGroups:                  &[]string{"test"},
					JwtGroupsClaimName:              valPtr("test"),
					JwtGroupsEnabled:                valPtr(true),
					PeerInactivityExpiration:        3600,
					PeerInactivityExpirationEnabled: true,
					PeerLoginExpiration:             3600,
					PeerLoginExpirationEnabled:      true,
					RegularUsersViewBlocked:         true,
					RoutingPeerDnsResolutionEnabled: valPtr(true),
					PeerExposeEnabled:               true,
					PeerExposeGroups:                []string{"group1"},
					Extra: &api.AccountExtraSettings{
						NetworkTrafficLogsEnabled:          true,
						NetworkTrafficPacketCounterEnabled: true,
						PeerApprovalEnabled:                true,
						UserApprovalRequired:               true,
						NetworkTrafficLogsGroups:           []string{"group1"},
					},
				},
			},
			expected: AccountSettingsModel{
				Id:                                 types.StringValue("b"),
				JwtAllowGroups:                     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("test")}),
				JwtGroupsClaimName:                 types.StringValue("test"),
				PeerLoginExpiration:                types.Int32Value(3600),
				PeerInactivityExpiration:           types.Int32Value(3600),
				PeerLoginExpirationEnabled:         types.BoolValue(true),
				PeerInactivityExpirationEnabled:    types.BoolValue(true),
				RegularUsersViewBlocked:            types.BoolValue(true),
				GroupsPropagationEnabled:           types.BoolValue(true),
				JwtGroupsEnabled:                   types.BoolValue(true),
				RoutingPeerDnsResolutionEnabled:    types.BoolValue(true),
				PeerApprovalEnabled:                types.BoolValue(true),
				NetworkTrafficLogsEnabled:          types.BoolValue(true),
				NetworkTrafficPacketCounterEnabled: types.BoolValue(true),
				AutoUpdateVersion:                  types.StringValue("latest"),
				DnsDomain:                          types.StringValue("custom.com"),
				NetworkRange:                       types.StringValue("100.64.0.0/10"),
				LazyConnectionEnabled:              types.BoolValue(true),
				UserApprovalRequired:               types.BoolValue(true),
				NetworkTrafficLogsGroups:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("group1")}),
				PeerExposeEnabled:                  types.BoolValue(true),
				PeerExposeGroups:                   types.ListValueMust(types.StringType, []attr.Value{types.StringValue("group1")}),
			},
		},
	}

	for _, c := range cases {
		var out AccountSettingsModel
		outDiag := accountAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_accountTerraformToAPI(t *testing.T) {
	cases := []struct {
		currentAccount *api.Account
		data           AccountSettingsModel
		expected       api.AccountRequest
	}{
		{
			currentAccount: &api.Account{
				Id: "a1",
				Settings: api.AccountSettings{
					AutoUpdateVersion:               nil,
					DnsDomain:                       nil,
					NetworkRange:                    nil,
					LazyConnectionEnabled:           nil,
					GroupsPropagationEnabled:        nil,
					JwtAllowGroups:                  nil,
					JwtGroupsClaimName:              nil,
					JwtGroupsEnabled:                nil,
					PeerInactivityExpiration:        1800,
					PeerInactivityExpirationEnabled: false,
					PeerLoginExpiration:             1800,
					PeerLoginExpirationEnabled:      false,
					RegularUsersViewBlocked:         false,
					RoutingPeerDnsResolutionEnabled: nil,
					PeerExposeEnabled:               false,
					PeerExposeGroups:                nil,
					Extra: &api.AccountExtraSettings{
						NetworkTrafficLogsEnabled:          false,
						NetworkTrafficPacketCounterEnabled: false,
						PeerApprovalEnabled:                false,
						UserApprovalRequired:               false,
						NetworkTrafficLogsGroups:           nil,
					},
				},
			},
			data: AccountSettingsModel{
				Id:                  types.StringValue("a1"),
				PeerLoginExpiration: types.Int32Value(3600),
			},
			expected: api.AccountRequest{
				Settings: api.AccountSettings{
					AutoUpdateVersion:               nil,
					DnsDomain:                       nil,
					NetworkRange:                    nil,
					LazyConnectionEnabled:           nil,
					GroupsPropagationEnabled:        nil,
					JwtAllowGroups:                  nil,
					JwtGroupsClaimName:              nil,
					JwtGroupsEnabled:                nil,
					PeerInactivityExpiration:        1800,
					PeerInactivityExpirationEnabled: false,
					PeerLoginExpiration:             3600,
					PeerLoginExpirationEnabled:      false,
					RegularUsersViewBlocked:         false,
					RoutingPeerDnsResolutionEnabled: nil,
					PeerExposeEnabled:               false,
					PeerExposeGroups:                nil,
					Extra: &api.AccountExtraSettings{
						NetworkTrafficLogsEnabled:          false,
						NetworkTrafficPacketCounterEnabled: false,
						PeerApprovalEnabled:                false,
						UserApprovalRequired:               false,
						NetworkTrafficLogsGroups:           nil,
					},
				},
			},
		},
		{
			currentAccount: &api.Account{
				Id: "a2",
				Settings: api.AccountSettings{
					AutoUpdateVersion:               nil,
					DnsDomain:                       nil,
					NetworkRange:                    nil,
					LazyConnectionEnabled:           nil,
					GroupsPropagationEnabled:        nil,
					JwtAllowGroups:                  nil,
					JwtGroupsClaimName:              nil,
					JwtGroupsEnabled:                nil,
					PeerInactivityExpiration:        1800,
					PeerInactivityExpirationEnabled: false,
					PeerLoginExpiration:             1800,
					PeerLoginExpirationEnabled:      false,
					RegularUsersViewBlocked:         false,
					RoutingPeerDnsResolutionEnabled: nil,
					PeerExposeEnabled:               false,
					PeerExposeGroups:                nil,
					Extra: &api.AccountExtraSettings{
						NetworkTrafficLogsEnabled:          false,
						NetworkTrafficPacketCounterEnabled: false,
						PeerApprovalEnabled:                false,
						UserApprovalRequired:               false,
						NetworkTrafficLogsGroups:           nil,
					},
				},
			},
			data: AccountSettingsModel{
				Id:                                 types.StringValue("a2"),
				PeerLoginExpiration:                types.Int32Value(3600),
				PeerLoginExpirationEnabled:         types.BoolValue(true),
				JwtAllowGroups:                     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				NetworkTrafficLogsEnabled:          types.BoolValue(true),
				PeerInactivityExpirationEnabled:    types.BoolValue(true),
				RegularUsersViewBlocked:            types.BoolValue(true),
				GroupsPropagationEnabled:           types.BoolValue(true),
				JwtGroupsEnabled:                   types.BoolValue(true),
				RoutingPeerDnsResolutionEnabled:    types.BoolValue(true),
				PeerApprovalEnabled:                types.BoolValue(true),
				NetworkTrafficPacketCounterEnabled: types.BoolValue(true),
				PeerInactivityExpiration:           types.Int32Value(3600),
				JwtGroupsClaimName:                 types.StringValue("name"),
				AutoUpdateVersion:                  types.StringValue("latest"),
				DnsDomain:                          types.StringValue("custom.com"),
				NetworkRange:                       types.StringValue("100.64.0.0/10"),
				LazyConnectionEnabled:              types.BoolValue(true),
				UserApprovalRequired:               types.BoolValue(true),
				NetworkTrafficLogsGroups:           types.ListValueMust(types.StringType, []attr.Value{types.StringValue("group1")}),
				PeerExposeEnabled:                  types.BoolValue(true),
				PeerExposeGroups:                   types.ListValueMust(types.StringType, []attr.Value{types.StringValue("group1")}),
			},
			expected: api.AccountRequest{
				Settings: api.AccountSettings{
					AutoUpdateVersion:               valPtr("latest"),
					DnsDomain:                       valPtr("custom.com"),
					NetworkRange:                    valPtr("100.64.0.0/10"),
					LazyConnectionEnabled:           valPtr(true),
					GroupsPropagationEnabled:        valPtr(true),
					JwtAllowGroups:                  &[]string{"g1"},
					JwtGroupsClaimName:              valPtr("name"),
					JwtGroupsEnabled:                valPtr(true),
					PeerInactivityExpiration:        3600,
					PeerInactivityExpirationEnabled: true,
					PeerLoginExpiration:             3600,
					PeerLoginExpirationEnabled:      true,
					RegularUsersViewBlocked:         true,
					RoutingPeerDnsResolutionEnabled: valPtr(true),
					PeerExposeEnabled:               true,
					PeerExposeGroups:                []string{"group1"},
					Extra: &api.AccountExtraSettings{
						NetworkTrafficLogsEnabled:          true,
						NetworkTrafficPacketCounterEnabled: true,
						PeerApprovalEnabled:                true,
						UserApprovalRequired:               true,
						NetworkTrafficLogsGroups:           []string{"group1"},
					},
				},
			},
		},
	}

	for _, c := range cases {
		out := accountTerraformToAPI(context.Background(), c.currentAccount, c.data)

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

// An empty netbird_account_settings resource adopts the account's current
// settings. The expected values are read back from the management server rather
// than hardcoded: the defaults a real bootstrap produces are management's to
// choose, and pinning them here would only assert what the seed file said.
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
						settings := accounts[0].Settings
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
