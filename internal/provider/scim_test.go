package provider

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_scimAPIToTerraform(t *testing.T) {
	syncTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	cases := []struct {
		name     string
		resource *api.ScimIntegration
		expected ScimModel
	}{
		{
			name: "basic integration",
			resource: &api.ScimIntegration{
				Id:                1,
				Provider:          "okta",
				Enabled:           true,
				AuthToken:         "masked-token",
				GroupPrefixes:     []string{"engineering-"},
				UserGroupPrefixes: []string{"users-"},
				LastSyncedAt:      syncTime,
			},
			expected: ScimModel{
				Id:                types.StringValue("1"),
				ProviderName:      types.StringValue("okta"),
				Enabled:           types.BoolValue(true),
				GroupPrefixes:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("engineering-")}),
				UserGroupPrefixes: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("users-")}),
				LastSyncedAt:      types.StringValue("2025-01-15T10:30:00Z"),
			},
		},
		{
			name: "empty prefixes",
			resource: &api.ScimIntegration{
				Id:                2,
				Provider:          "azure",
				Enabled:           false,
				AuthToken:         "masked",
				GroupPrefixes:     []string{},
				UserGroupPrefixes: []string{},
				LastSyncedAt:      syncTime,
			},
			expected: ScimModel{
				Id:                types.StringValue("2"),
				ProviderName:      types.StringValue("azure"),
				Enabled:           types.BoolValue(false),
				GroupPrefixes:     types.ListValueMust(types.StringType, []attr.Value{}),
				UserGroupPrefixes: types.ListValueMust(types.StringType, []attr.Value{}),
				LastSyncedAt:      types.StringValue("2025-01-15T10:30:00Z"),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out ScimModel
			// Preserve write-only fields
			out.AuthToken = types.StringValue("preserved-token")
			out.Prefix = types.StringValue("preserved-prefix")

			outDiag := scimAPIToTerraform(context.Background(), c.resource, &out)
			if outDiag.HasError() {
				t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
			}

			// auth_token and prefix should be preserved from existing state
			c.expected.AuthToken = types.StringValue("preserved-token")
			c.expected.Prefix = types.StringValue("preserved-prefix")

			if !reflect.DeepEqual(out, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
			}
		})
	}
}
