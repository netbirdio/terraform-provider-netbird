package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_groupAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Group
		expected GroupModel
	}{
		{
			resource: &api.Group{
				Id:             "abc",
				Issued:         valPtr(api.GroupIssuedApi),
				Name:           "Test",
				Peers:          []api.PeerMinimum{},
				PeersCount:     0,
				Resources:      []api.Resource{},
				ResourcesCount: 0,
			},
			expected: GroupModel{
				Id:        types.StringValue("abc"),
				Issued:    types.StringValue("api"),
				Name:      types.StringValue("Test"),
				Peers:     types.ListValueMust(types.StringType, []attr.Value{}),
				Resources: types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Group{
				Id:     "def",
				Issued: nil,
				Name:   "Meow",
				Peers: []api.PeerMinimum{
					{
						Id:   "c1",
						Name: "Useless",
					},
					{
						Id:   "c2",
						Name: "Also useless",
					},
				},
				PeersCount: 2,
				Resources: []api.Resource{
					{
						Id:   "r1",
						Type: api.ResourceTypeDomain,
					},
					{
						Id:   "r2",
						Type: api.ResourceTypeSubnet,
					},
				},
			},
			expected: GroupModel{
				Id:        types.StringValue("def"),
				Issued:    types.StringNull(),
				Name:      types.StringValue("Meow"),
				Peers:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("c1"), types.StringValue("c2")}),
				Resources: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("r1"), types.StringValue("r2")}),
			},
		},
	}

	for _, c := range cases {
		var out GroupModel
		outDiag := groupAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
