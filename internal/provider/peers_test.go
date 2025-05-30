package provider

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_filterPeers(t *testing.T) {
	cases := []struct {
		peers    []api.Peer
		filter   PeersModel
		expected []string
	}{
		{
			peers: []api.Peer{
				{
					Groups: []api.GroupMinimum{
						{
							Id: "g1",
						},
					},
					Id: "p1",
				},
				{
					Groups: []api.GroupMinimum{
						{
							Id: "g1",
						},
						{
							Id: "g2",
						},
					},
					Id: "p2",
				},
				{
					Groups: []api.GroupMinimum{
						{
							Id: "g2",
						},
					},
					Id: "p3",
				},
			},
			filter: PeersModel{
				Groups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
			},
			expected: []string{"p1", "p2"},
		},
	}

	for _, c := range cases {
		out, outDiag := filterPeers(context.Background(), c.peers, c.filter)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !slices.Equal(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
