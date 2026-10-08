package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_dnsZoneAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Zone
		expected DNSZoneModel
	}{
		{
			resource: &api.Zone{
				Id:                 "zone123",
				Name:               "Test Zone",
				Domain:             "example.com",
				Enabled:            true,
				EnableSearchDomain: false,
				DistributionGroups: []string{},
			},
			expected: DNSZoneModel{
				Id:                 types.StringValue("zone123"),
				Name:               types.StringValue("Test Zone"),
				Domain:             types.StringValue("example.com"),
				Enabled:            types.BoolValue(true),
				EnableSearchDomain: types.BoolValue(false),
				DistributionGroups: types.SetValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Zone{
				Id:                 "zone456",
				Name:               "Internal Zone",
				Domain:             "internal.local",
				Enabled:            false,
				EnableSearchDomain: true,
				DistributionGroups: []string{"grp1", "grp2"},
			},
			expected: DNSZoneModel{
				Id:                 types.StringValue("zone456"),
				Name:               types.StringValue("Internal Zone"),
				Domain:             types.StringValue("internal.local"),
				Enabled:            types.BoolValue(false),
				EnableSearchDomain: types.BoolValue(true),
				DistributionGroups: types.SetValueMust(types.StringType, []attr.Value{
					types.StringValue("grp1"),
					types.StringValue("grp2"),
				}),
			},
		},
	}

	for _, c := range cases {
		var out DNSZoneModel
		outDiag := dnsZoneAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
