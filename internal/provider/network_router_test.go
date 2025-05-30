package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_networkRouterAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.NetworkRouter
		expected NetworkRouterModel
	}{
		{
			resource: &api.NetworkRouter{
				Enabled:    false,
				Id:         "ro1",
				Masquerade: false,
				Metric:     1,
				Peer:       nil,
				PeerGroups: nil,
			},
			expected: NetworkRouterModel{
				Id:         types.StringValue("ro1"),
				Enabled:    types.BoolValue(false),
				PeerGroups: types.ListNull(types.StringType),
				Peer:       types.StringNull(),
				Metric:     types.Int32Value(1),
				Masquerade: types.BoolValue(false),
			},
		},
		{
			resource: &api.NetworkRouter{
				Enabled:    true,
				Id:         "ro2",
				Masquerade: true,
				Metric:     9999,
				Peer:       valPtr(""),
				PeerGroups: &[]string{"g1", "g2"},
			},
			expected: NetworkRouterModel{
				Id:         types.StringValue("ro2"),
				Enabled:    types.BoolValue(true),
				PeerGroups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1"), types.StringValue("g2")}),
				Peer:       types.StringNull(),
				Metric:     types.Int32Value(9999),
				Masquerade: types.BoolValue(true),
			},
		},
		{
			resource: &api.NetworkRouter{
				Enabled:    true,
				Id:         "ro3",
				Masquerade: true,
				Metric:     9999,
				Peer:       valPtr("p1"),
				PeerGroups: nil,
			},
			expected: NetworkRouterModel{
				Id:         types.StringValue("ro3"),
				Enabled:    types.BoolValue(true),
				PeerGroups: types.ListNull(types.StringType),
				Peer:       types.StringValue("p1"),
				Metric:     types.Int32Value(9999),
				Masquerade: types.BoolValue(true),
			},
		},
	}

	for _, c := range cases {
		var out NetworkRouterModel
		outDiag := networkRouterAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
