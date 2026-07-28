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
			resource: &api.AgentNetworkSettings{
				Cluster:                "c1",
				Subdomain:              "sub",
				Endpoint:               "sub.c1",
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              true,
				AccessLogRetentionDays: valPtr(90),
			},
			expected: AgentNetworkSettingsModel{
				Cluster:                types.StringValue("c1"),
				Subdomain:              types.StringValue("sub"),
				Endpoint:               types.StringValue("sub.c1"),
				EnableLogCollection:    types.BoolValue(true),
				EnablePromptCollection: types.BoolValue(true),
				RedactPii:              types.BoolValue(true),
				AccessLogRetentionDays: types.Int64Value(90),
			},
		},
		{
			// Absent retention means "keep indefinitely" and maps to null.
			resource: &api.AgentNetworkSettings{
				Cluster:   "c1",
				Subdomain: "sub",
				Endpoint:  "sub.c1",
			},
			expected: AgentNetworkSettingsModel{
				Cluster:                types.StringValue("c1"),
				Subdomain:              types.StringValue("sub"),
				Endpoint:               types.StringValue("sub.c1"),
				EnableLogCollection:    types.BoolValue(false),
				EnablePromptCollection: types.BoolValue(false),
				RedactPii:              types.BoolValue(false),
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
// silently disable log/prompt collection or drop access-log retention.
func Test_settingsUpdateRequest(t *testing.T) {
	current := &api.AgentNetworkSettings{
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
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              true,
				AccessLogRetentionDays: valPtr(90),
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
				EnableLogCollection:    false,
				EnablePromptCollection: false,
				RedactPii:              false,
				AccessLogRetentionDays: valPtr(30),
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
				EnableLogCollection:    true,
				EnablePromptCollection: true,
				RedactPii:              false,
				AccessLogRetentionDays: valPtr(90),
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
