package provider

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_filterGroups(t *testing.T) {
	jwt := api.GroupIssuedJwt
	apiIssued := api.GroupIssuedApi
	groups := []api.Group{
		{Id: "g1", Name: "All", Issued: &apiIssued},
		{Id: "g2", Name: "NetbirdUsersInfraDyn", Issued: &jwt},
		{Id: "g3", Name: "NetbirdUsersRrdDyn", Issued: &jwt},
		// A group read back without an issuer must not crash the filter.
		{Id: "g4", Name: "Legacy"},
	}

	cases := []struct {
		name     string
		filter   GroupsModel
		expected []string
	}{
		{
			name:     "no filter returns every group",
			filter:   GroupsModel{},
			expected: []string{"g1", "g2", "g3", "g4"},
		},
		{
			name:     "issued",
			filter:   GroupsModel{Issued: types.StringValue("jwt")},
			expected: []string{"g2", "g3"},
		},
		{
			name:     "name and issued are both required",
			filter:   GroupsModel{Name: types.StringValue("All"), Issued: types.StringValue("jwt")},
			expected: []string{},
		},
		{
			// The reason this data source exists: an OIDC group nobody logged in with
			// yet is an empty result, not an error.
			name:     "missing name is an empty result",
			filter:   GroupsModel{Name: types.StringValue("NetbirdUsersNotYetDyn")},
			expected: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := filterGroups(groups, c.filter)
			ids := make([]string, len(out))
			for i, g := range out {
				ids[i] = g.Id
			}
			if !slices.Equal(ids, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, ids)
			}
		})
	}
}

func Test_groupsAPIToTerraform(t *testing.T) {
	jwt := api.GroupIssuedJwt
	groups := []api.Group{
		{Id: "g2", Name: "NetbirdUsersInfraDyn", Issued: &jwt, Peers: []api.PeerMinimum{{Id: "p1"}}},
	}

	var data GroupsModel
	if d := groupsAPIToTerraform(context.Background(), groups, &data); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	var ids []string
	data.Ids.ElementsAs(context.Background(), &ids, false)
	if !slices.Equal(ids, []string{"g2"}) {
		t.Fatalf("Expected ids [g2], found %v", ids)
	}
	if len(data.Groups) != 1 || data.Groups[0].Name.ValueString() != "NetbirdUsersInfraDyn" ||
		data.Groups[0].Issued.ValueString() != "jwt" || len(data.Groups[0].Peers.Elements()) != 1 {
		t.Fatalf("Unexpected group mapping: %#v", data.Groups)
	}
}
