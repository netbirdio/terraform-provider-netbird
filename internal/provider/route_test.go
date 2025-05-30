package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_routeAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Route
		expected RouteModel
	}{
		{
			resource: &api.Route{
				Id:          "r1",
				Description: "desc",
				Enabled:     true,
				Domains:     &[]string{"example.com"},
				Groups:      []string{"g1"},
				KeepRoute:   true,
				Masquerade:  true,
				Metric:      9999,
				NetworkId:   "route",
				NetworkType: "domain",
				PeerGroups:  &[]string{"g2"},
			},
			expected: RouteModel{
				Id:                  types.StringValue("r1"),
				Description:         types.StringValue("desc"),
				Enabled:             types.BoolValue(true),
				KeepRoute:           types.BoolValue(true),
				Masquerade:          types.BoolValue(true),
				NetworkId:           types.StringValue("route"),
				NetworkType:         types.StringValue("domain"),
				PeerGroups:          types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g2")}),
				Domains:             types.ListValueMust(types.StringType, []attr.Value{types.StringValue("example.com")}),
				Metric:              types.Int32Value(9999),
				Groups:              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				Peer:                types.StringNull(),
				Network:             types.StringNull(),
				AccessControlGroups: types.ListNull(types.StringType),
			},
		},
	}

	for _, c := range cases {
		var out RouteModel
		outDiag := routeAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
