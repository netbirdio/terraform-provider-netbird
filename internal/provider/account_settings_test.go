package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_AccountSettings_LocalMFASchema(t *testing.T) {
	ctx := context.Background()
	var resourceResp resource.SchemaResponse
	(&AccountSettings{}).Schema(ctx, resource.SchemaRequest{}, &resourceResp)
	attribute, ok := resourceResp.Schema.Attributes["local_mfa_enabled"]
	if !ok {
		t.Fatal("account settings resource must expose local_mfa_enabled")
	}
	if !attribute.IsOptional() || !attribute.IsComputed() {
		t.Fatal("local_mfa_enabled must be optional and computed to adopt server settings")
	}
	var dataSourceResp datasource.SchemaResponse
	(&AccountSettingsDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &dataSourceResp)
	attributeDS, ok := dataSourceResp.Schema.Attributes["local_mfa_enabled"]
	if !ok || !attributeDS.IsComputed() {
		t.Fatal("account settings data source must expose computed local_mfa_enabled")
	}
}

func Test_AccountSettings_LocalMFAMapping(t *testing.T) {
	cases := []struct {
		name    string
		current *bool
		planned types.Bool
		want    *bool
	}{
		{"enable", valPtr(false), types.BoolValue(true), valPtr(true)},
		{"disable", valPtr(true), types.BoolValue(false), valPtr(false)},
		{"omitted enabled", valPtr(true), types.BoolNull(), valPtr(true)},
		{"omitted disabled", valPtr(false), types.BoolNull(), valPtr(false)},
		{"unknown enabled", valPtr(true), types.BoolUnknown(), valPtr(true)},
		{"unknown disabled", valPtr(false), types.BoolUnknown(), valPtr(false)},
		{"unavailable", nil, types.BoolNull(), nil},
		{"unknown unavailable", nil, types.BoolUnknown(), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			account := &api.Account{Settings: api.AccountSettings{
				LocalMfaEnabled: c.current,
				Extra:           &api.AccountExtraSettings{},
			}}
			var data AccountSettingsModel
			if diagnostics := accountAPIToTerraform(ctx, account, &data); diagnostics.HasError() {
				t.Fatalf("map account settings: %v", diagnostics)
			}
			if !data.LocalMfaEnabled.Equal(types.BoolPointerValue(c.current)) {
				t.Fatalf("read local_mfa_enabled: got %v, want %v", data.LocalMfaEnabled, c.current)
			}
			data.LocalMfaEnabled = c.planned
			request := accountTerraformToAPI(ctx, account, data)
			if !reflect.DeepEqual(request.Settings.LocalMfaEnabled, c.want) {
				t.Fatalf("write local_mfa_enabled: got %v, want %v", request.Settings.LocalMfaEnabled, c.want)
			}
		})
	}
}

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
					LocalMfaEnabled:                 valPtr(true),
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
				LocalMfaEnabled:                    types.BoolValue(true),
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
