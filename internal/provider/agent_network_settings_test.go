package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_agentNetworkSettingsAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.AgentNetworkSettings
		expected AgentNetworkSettingsModel
	}{
		{
			// A gateway beneath a shared cluster: the endpoint hangs one label
			// below the proxy address, so the two differ and it is not dedicated.
			resource: &api.AgentNetworkSettings{
				Endpoint:               "sub.c1",
				ProxyAddress:           "c1",
				Dedicated:              false,
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              true,
				AccessLogRetentionDays: valPtr(90),
			},
			expected: AgentNetworkSettingsModel{
				Endpoint:               types.StringValue("sub.c1"),
				ProxyAddress:           types.StringValue("c1"),
				Dedicated:              types.BoolValue(false),
				EnableLogCollection:    types.BoolValue(true),
				EnablePromptCollection: types.BoolValue(true),
				RedactPii:              types.BoolValue(true),
				AccessLogRetentionDays: types.Int64Value(90),
			},
		},
		{
			// A claimed hostname: endpoint and proxy address coincide, and the
			// gateway is dedicated.
			resource: &api.AgentNetworkSettings{
				Endpoint:     "gw.example",
				ProxyAddress: "gw.example",
				Dedicated:    true,
			},
			expected: AgentNetworkSettingsModel{
				Endpoint:               types.StringValue("gw.example"),
				ProxyAddress:           types.StringValue("gw.example"),
				Dedicated:              types.BoolValue(true),
				EnableLogCollection:    types.BoolValue(false),
				EnablePromptCollection: types.BoolValue(false),
				RedactPii:              types.BoolValue(false),
				// Absent retention means "keep indefinitely" and maps to null.
				AccessLogRetentionDays: types.Int64Null(),
			},
		},
	}

	for _, c := range cases {
		var out AgentNetworkSettingsModel
		agentNetworkSettingsAPIToTerraform(c.resource, &out)

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

// The settings row is a server-managed singleton and the API replaces every
// mutable field on PUT. Fields the configuration does not set must therefore
// adopt the account's current value; sending schema defaults instead would
// silently disable log/prompt collection or drop access-log retention. The two
// addresses are echoed unchanged, because the API rejects any other value.
func Test_settingsUpdateRequest(t *testing.T) {
	current := &api.AgentNetworkSettings{
		Endpoint:               "sub.c1",
		ProxyAddress:           "c1",
		EnableLogCollection:    true,
		EnablePromptCollection: true,
		RedactPii:              true,
		AccessLogRetentionDays: valPtr(90),
	}

	cases := []struct {
		name     string
		data     AgentNetworkSettingsModel
		expected api.AgentNetworkSettingsRequest
	}{
		{
			name: "unset fields adopt current values",
			data: AgentNetworkSettingsModel{
				EnableLogCollection:    types.BoolNull(),
				EnablePromptCollection: types.BoolUnknown(),
				RedactPii:              types.BoolNull(),
				AccessLogRetentionDays: types.Int64Null(),
			},
			expected: api.AgentNetworkSettingsRequest{
				Endpoint:               "sub.c1",
				ProxyAddress:           "c1",
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              true,
				AccessLogRetentionDays: 90,
			},
		},
		{
			name: "set fields override current values",
			data: AgentNetworkSettingsModel{
				EnableLogCollection:    types.BoolValue(false),
				EnablePromptCollection: types.BoolValue(false),
				RedactPii:              types.BoolValue(false),
				AccessLogRetentionDays: types.Int64Value(30),
			},
			expected: api.AgentNetworkSettingsRequest{
				Endpoint:               "sub.c1",
				ProxyAddress:           "c1",
				EnableLogCollection:    false,
				EnablePromptCollection: false,
				RedactPii:              false,
				AccessLogRetentionDays: 30,
			},
		},
		{
			name: "partial config only overrides what it sets",
			data: AgentNetworkSettingsModel{
				EnableLogCollection:    types.BoolNull(),
				EnablePromptCollection: types.BoolNull(),
				RedactPii:              types.BoolValue(false),
				AccessLogRetentionDays: types.Int64Null(),
			},
			expected: api.AgentNetworkSettingsRequest{
				Endpoint:               "sub.c1",
				ProxyAddress:           "c1",
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              false,
				AccessLogRetentionDays: 90,
			},
		},
		{
			// A configured address is never sent as such: the assigned value is,
			// so a stale configuration cannot turn into a rejected request.
			name: "configured addresses do not override the assigned ones",
			data: AgentNetworkSettingsModel{
				Endpoint:     types.StringValue("other.example"),
				ProxyAddress: types.StringValue("other"),
			},
			expected: api.AgentNetworkSettingsRequest{
				Endpoint:               "sub.c1",
				ProxyAddress:           "c1",
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              true,
				AccessLogRetentionDays: 90,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := settingsUpdateRequest(&c.data, current)
			if !reflect.DeepEqual(out, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
			}
		})
	}
}

// Bootstrap sends only what the configuration set, so the server applies its own
// defaults to the rest rather than this provider inventing them. Exactly one of
// the two addresses travels, which is what selects the gateway's shape.
func Test_settingsCreateRequest(t *testing.T) {
	cases := []struct {
		name     string
		data     AgentNetworkSettingsModel
		expected api.AgentNetworkSettingsCreateRequest
	}{
		{
			name: "proxy address only, no collection fields",
			data: AgentNetworkSettingsModel{
				ProxyAddress:           types.StringValue("c1"),
				Endpoint:               types.StringNull(),
				EnableLogCollection:    types.BoolNull(),
				EnablePromptCollection: types.BoolUnknown(),
				RedactPii:              types.BoolNull(),
				AccessLogRetentionDays: types.Int64Null(),
			},
			expected: api.AgentNetworkSettingsCreateRequest{
				ProxyAddress: valPtr("c1"),
			},
		},
		{
			name: "endpoint claim with collection fields",
			data: AgentNetworkSettingsModel{
				Endpoint:               types.StringValue("gw.example"),
				ProxyAddress:           types.StringUnknown(),
				EnableLogCollection:    types.BoolValue(true),
				EnablePromptCollection: types.BoolValue(false),
				RedactPii:              types.BoolValue(true),
				AccessLogRetentionDays: types.Int64Value(45),
			},
			expected: api.AgentNetworkSettingsCreateRequest{
				Endpoint:               valPtr("gw.example"),
				EnableLogCollection:    valPtr(true),
				EnablePromptCollection: valPtr(false),
				RedactPii:              valPtr(true),
				AccessLogRetentionDays: valPtr(45),
			},
		},
		{
			// An empty string is an omitted address, not a claim on "".
			name: "empty addresses are omitted",
			data: AgentNetworkSettingsModel{
				Endpoint:     types.StringValue(""),
				ProxyAddress: types.StringValue(""),
			},
			expected: api.AgentNetworkSettingsCreateRequest{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := settingsCreateRequest(&c.data)
			if !reflect.DeepEqual(out, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
			}
		})
	}
}

// configured is what decides whether an address travels in a bootstrap request,
// so the states that must not count as configured are pinned here: an omitted
// attribute, one that is interpolated and still unknown, and an empty string.
func Test_configured(t *testing.T) {
	cases := []struct {
		name  string
		value types.String
		want  bool
	}{
		{"null", types.StringNull(), false},
		{"unknown", types.StringUnknown(), false},
		{"empty", types.StringValue(""), false},
		{"set", types.StringValue("c1"), true},
	}
	for _, c := range cases {
		if got := configured(c.value); got != c.want {
			t.Errorf("%s: configured = %v, want %v", c.name, got, c.want)
		}
	}
}
