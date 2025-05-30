package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_networkAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Network
		expected NetworkModel
	}{
		{
			resource: &api.Network{
				Id:                "n1",
				Description:       valPtr("Test"),
				Name:              "Network",
				Policies:          []string{},
				Resources:         []string{},
				Routers:           []string{},
				RoutingPeersCount: 0,
			},
			expected: NetworkModel{
				Id:          types.StringValue("n1"),
				Name:        types.StringValue("Network"),
				Description: types.StringValue("Test"),
				Resources:   types.ListValueMust(types.StringType, []attr.Value{}),
				Routers:     types.ListValueMust(types.StringType, []attr.Value{}),
				Policies:    types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Network{
				Id:                "n2",
				Description:       valPtr("test2"),
				Name:              "Network2",
				Policies:          []string{"p1"},
				Resources:         []string{"r1"},
				Routers:           []string{"ro1"},
				RoutingPeersCount: 3,
			},
			expected: NetworkModel{
				Id:          types.StringValue("n2"),
				Name:        types.StringValue("Network2"),
				Description: types.StringValue("test2"),
				Resources:   types.ListValueMust(types.StringType, []attr.Value{types.StringValue("r1")}),
				Routers:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ro1")}),
				Policies:    types.ListValueMust(types.StringType, []attr.Value{types.StringValue("p1")}),
			},
		},
	}

	for _, c := range cases {
		var out NetworkModel
		outDiag := networkAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
