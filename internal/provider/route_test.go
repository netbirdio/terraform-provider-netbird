package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
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
				SkipAutoApply:       types.BoolNull(),
			},
		},
		{
			resource: &api.Route{
				Id:            "r2",
				Description:   "exit node",
				Enabled:       true,
				Network:       valPtr("0.0.0.0/0"),
				Groups:        []string{"g1"},
				KeepRoute:     false,
				Masquerade:    true,
				Metric:        100,
				NetworkId:     "exit",
				NetworkType:   "IPv4",
				Peer:          valPtr("peer1"),
				SkipAutoApply: valPtr(true),
			},
			expected: RouteModel{
				Id:                  types.StringValue("r2"),
				Description:         types.StringValue("exit node"),
				Enabled:             types.BoolValue(true),
				KeepRoute:           types.BoolValue(false),
				Masquerade:          types.BoolValue(true),
				NetworkId:           types.StringValue("exit"),
				NetworkType:         types.StringValue("IPv4"),
				PeerGroups:          types.ListNull(types.StringType),
				Domains:             types.ListNull(types.StringType),
				Metric:              types.Int32Value(100),
				Groups:              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				Peer:                types.StringValue("peer1"),
				Network:             types.StringValue("0.0.0.0/0"),
				AccessControlGroups: types.ListNull(types.StringType),
				SkipAutoApply:       types.BoolValue(true),
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
