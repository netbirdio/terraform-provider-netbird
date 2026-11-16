package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	tfdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_firstAccount(t *testing.T) {
	t.Run("returns first account", func(t *testing.T) {
		accounts := []api.Account{{Id: "first"}, {Id: "second"}}

		account, err := firstAccount(accounts)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if account.Id != "first" {
			t.Fatalf("Expected first account, got %q", account.Id)
		}
	})

	t.Run("returns error for empty response", func(t *testing.T) {
		account, err := firstAccount(nil)

		if account != nil {
			t.Fatalf("Expected no account, got %#v", account)
		}
		if err == nil || !strings.Contains(err.Error(), "no accounts returned") {
			t.Fatalf("Expected empty response error, got %v", err)
		}
	})
}

func Test_AccountSettingsReadHandlesInvalidAccountResponses(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		response       string
		expectedDetail string
	}{
		{
			name:           "API error",
			status:         http.StatusInternalServerError,
			response:       `{"message":"internal error"}`,
			expectedDetail: "internal error",
		},
		{
			name:           "empty account list",
			status:         http.StatusOK,
			response:       `[]`,
			expectedDetail: "no accounts returned by API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			accountSettings := &AccountSettings{
				client: netbird.New(server.URL, "test-token"),
			}
			schemaResponse := tfresource.SchemaResponse{}
			accountSettings.Schema(context.Background(), tfresource.SchemaRequest{}, &schemaResponse)

			state := tfsdk.State{Schema: schemaResponse.Schema}
			diags := state.Set(context.Background(), &AccountSettingsModel{
				Id:                       types.StringValue("account-id"),
				JwtAllowGroups:           types.ListNull(types.StringType),
				NetworkTrafficLogsGroups: types.ListNull(types.StringType),
				PeerExposeGroups:         types.ListNull(types.StringType),
			})
			if diags.HasError() {
				t.Fatalf("Failed to prepare resource state: %v", diags.Errors())
			}

			response := tfresource.ReadResponse{State: state}
			accountSettings.Read(
				context.Background(),
				tfresource.ReadRequest{State: state},
				&response,
			)

			if !response.Diagnostics.HasError() {
				t.Fatal("Expected an error diagnostic")
			}
			if !strings.Contains(response.Diagnostics.Errors()[0].Detail(), tt.expectedDetail) {
				t.Fatalf(
					"Expected diagnostic containing %q, got %q",
					tt.expectedDetail,
					response.Diagnostics.Errors()[0].Detail(),
				)
			}
		})
	}
}

func Test_AccountSettingsCreateHandlesInvalidAccountResponses(t *testing.T) {
	testAccountSettingsWriteHandlesInvalidAccountResponses(t, "Create")
}

func Test_AccountSettingsUpdateHandlesInvalidAccountResponses(t *testing.T) {
	testAccountSettingsWriteHandlesInvalidAccountResponses(t, "Update")
}

func testAccountSettingsWriteHandlesInvalidAccountResponses(t *testing.T, operation string) {
	t.Helper()

	tests := []struct {
		name           string
		status         int
		response       string
		expectedDetail string
	}{
		{
			name:           "API error",
			status:         http.StatusInternalServerError,
			response:       `{"message":"internal error"}`,
			expectedDetail: "internal error",
		},
		{
			name:           "empty account list",
			status:         http.StatusOK,
			response:       `[]`,
			expectedDetail: "no accounts returned by API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var updateRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					updateRequests.Add(1)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			accountSettings := &AccountSettings{
				client: netbird.New(server.URL, "test-token"),
			}
			schemaResponse := tfresource.SchemaResponse{}
			accountSettings.Schema(context.Background(), tfresource.SchemaRequest{}, &schemaResponse)

			plan := tfsdk.Plan{Schema: schemaResponse.Schema}
			diags := plan.Set(context.Background(), &AccountSettingsModel{
				Id:                       types.StringValue("account-id"),
				JwtAllowGroups:           types.ListNull(types.StringType),
				NetworkTrafficLogsGroups: types.ListNull(types.StringType),
				PeerExposeGroups:         types.ListNull(types.StringType),
			})
			if diags.HasError() {
				t.Fatalf("Failed to prepare resource plan: %v", diags.Errors())
			}

			var diagnosticsError bool
			var diagnosticDetail string
			switch operation {
			case "Create":
				response := tfresource.CreateResponse{
					State: tfsdk.State{Schema: schemaResponse.Schema},
				}
				accountSettings.Create(
					context.Background(),
					tfresource.CreateRequest{Plan: plan},
					&response,
				)
				diagnosticsError = response.Diagnostics.HasError()
				if diagnosticsError {
					diagnosticDetail = response.Diagnostics.Errors()[0].Detail()
				}
			case "Update":
				response := tfresource.UpdateResponse{
					State: tfsdk.State{Schema: schemaResponse.Schema},
				}
				accountSettings.Update(
					context.Background(),
					tfresource.UpdateRequest{Plan: plan},
					&response,
				)
				diagnosticsError = response.Diagnostics.HasError()
				if diagnosticsError {
					diagnosticDetail = response.Diagnostics.Errors()[0].Detail()
				}
			default:
				t.Fatalf("Unsupported operation %q", operation)
			}

			if !diagnosticsError {
				t.Fatal("Expected an error diagnostic")
			}
			if !strings.Contains(diagnosticDetail, tt.expectedDetail) {
				t.Fatalf(
					"Expected diagnostic containing %q, got %q",
					tt.expectedDetail,
					diagnosticDetail,
				)
			}
			if got := updateRequests.Load(); got != 0 {
				t.Fatalf("Expected no account update request, got %d", got)
			}
		})
	}
}

func Test_AccountSettingsDataSourceReadHandlesInvalidAccountResponses(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		response       string
		expectedDetail string
	}{
		{
			name:           "API error",
			status:         http.StatusInternalServerError,
			response:       `{"message":"internal error"}`,
			expectedDetail: "internal error",
		},
		{
			name:           "empty account list",
			status:         http.StatusOK,
			response:       `[]`,
			expectedDetail: "no accounts returned by API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			accountSettings := &AccountSettingsDataSource{
				client: netbird.New(server.URL, "test-token"),
			}
			schemaResponse := tfdatasource.SchemaResponse{}
			accountSettings.Schema(context.Background(), tfdatasource.SchemaRequest{}, &schemaResponse)

			configValues := make(map[string]tftypes.Value, len(schemaResponse.Schema.Attributes))
			for name, attribute := range schemaResponse.Schema.Attributes {
				attributeType := attribute.GetType().TerraformType(context.Background())
				configValues[name] = tftypes.NewValue(attributeType, nil)
			}
			config := tfsdk.Config{
				Raw: tftypes.NewValue(
					schemaResponse.Schema.Type().TerraformType(context.Background()),
					configValues,
				),
				Schema: schemaResponse.Schema,
			}
			response := tfdatasource.ReadResponse{
				State: tfsdk.State{Schema: schemaResponse.Schema},
			}
			accountSettings.Read(
				context.Background(),
				tfdatasource.ReadRequest{Config: config},
				&response,
			)

			if !response.Diagnostics.HasError() {
				t.Fatal("Expected an error diagnostic")
			}
			if !strings.Contains(response.Diagnostics.Errors()[0].Detail(), tt.expectedDetail) {
				t.Fatalf(
					"Expected diagnostic containing %q, got %q",
					tt.expectedDetail,
					response.Diagnostics.Errors()[0].Detail(),
				)
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
