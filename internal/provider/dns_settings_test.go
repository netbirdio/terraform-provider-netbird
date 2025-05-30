package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_dnsSettingsAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.DNSSettings
		expected DNSSettingsModel
	}{
		{
			resource: &api.DNSSettings{
				DisabledManagementGroups: []string{},
			},
			expected: DNSSettingsModel{
				DisabledManagementGroups: types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.DNSSettings{
				DisabledManagementGroups: []string{"abc"},
			},
			expected: DNSSettingsModel{
				DisabledManagementGroups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("abc")}),
			},
		},
	}

	for _, c := range cases {
		var out DNSSettingsModel
		outDiag := dnsSettingsAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
