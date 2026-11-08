package provider

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Test_filterUsers checks each users filter in isolation and combined.
func Test_filterUsers(t *testing.T) {
	yes, no := true, false
	apiIssued, integration := "api", "integration"
	users := []api.User{
		{Id: "u1", Name: "Alice", Email: "alice@example.com", Role: "admin", Status: api.UserStatusActive,
			Issued: &apiIssued, IsServiceUser: &no, AutoGroups: []string{"g1", "g2"}},
		{Id: "u2", Name: "Bob", Email: "bob@example.com", Role: "user", Status: api.UserStatusBlocked,
			Issued: &integration, IsServiceUser: &no, IsBlocked: true, AutoGroups: []string{"g1"}},
		{Id: "u3", Name: "ci", Role: "user", Status: api.UserStatusActive, Issued: &apiIssued, IsServiceUser: &yes},
		// Optional pointers left unset must not crash the filter.
		{Id: "u4", Name: "Carol", Role: "user", Status: api.UserStatusInvited},
	}

	cases := []struct {
		name     string
		filter   UsersModel
		expected []string
	}{
		{
			name:     "no filter returns every user",
			filter:   UsersModel{},
			expected: []string{"u1", "u2", "u3", "u4"},
		},
		{
			name:     "service users",
			filter:   UsersModel{IsServiceUser: types.BoolValue(true)},
			expected: []string{"u3"},
		},
		{
			name:     "regular users count an unset is_service_user as false",
			filter:   UsersModel{IsServiceUser: types.BoolValue(false)},
			expected: []string{"u1", "u2", "u4"},
		},
		{
			name:     "role and is_blocked are both required",
			filter:   UsersModel{Role: types.StringValue("user"), IsBlocked: types.BoolValue(false)},
			expected: []string{"u3", "u4"},
		},
		{
			name:     "status and issued",
			filter:   UsersModel{Status: types.StringValue("active"), Issued: types.StringValue("api")},
			expected: []string{"u1", "u3"},
		},
		{
			name: "auto_groups contains all listed ids",
			filter: UsersModel{
				AutoGroups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
			},
			expected: []string{"u1", "u2"},
		},
		{
			name:     "name",
			filter:   UsersModel{Name: types.StringValue("Bob")},
			expected: []string{"u2"},
		},
		{
			name:     "email",
			filter:   UsersModel{Email: types.StringValue("alice@example.com")},
			expected: []string{"u1"},
		},
		{
			name: "auto_groups requires every listed id",
			filter: UsersModel{
				AutoGroups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1"), types.StringValue("g2")}),
			},
			expected: []string{"u1"},
		},
		{
			name:     "no match is an empty result",
			filter:   UsersModel{Email: types.StringValue("nobody@example.com")},
			expected: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, d := filterUsers(context.Background(), users, c.filter)
			if d.HasError() {
				t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
			}
			ids := make([]string, len(out))
			for i, u := range out {
				ids[i] = u.Id
			}
			if !slices.Equal(ids, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, ids)
			}
		})
	}
}

// Test_usersAPIToTerraform checks API users map to the Terraform list.
func Test_usersAPIToTerraform(t *testing.T) {
	yes, no := true, false
	issued := "api"
	login := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	users := []api.User{
		{Id: "u1", Name: "Alice", Email: "alice@example.com", Role: "admin", Status: api.UserStatusActive,
			Issued: &issued, IsServiceUser: &no, IsCurrent: &yes, LastLogin: &login, AutoGroups: []string{"g1"}},
	}

	var data UsersModel
	if d := usersAPIToTerraform(context.Background(), users, &data); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	var ids []string
	data.Ids.ElementsAs(context.Background(), &ids, false)
	if !slices.Equal(ids, []string{"u1"}) {
		t.Fatalf("Expected ids [u1], found %v", ids)
	}
	if len(data.Users) != 1 || data.Users[0].Email.ValueString() != "alice@example.com" ||
		data.Users[0].LastLogin.ValueString() != "2026-09-30T08:00:00Z" || !data.Users[0].IsCurrent.ValueBool() {
		t.Fatalf("Unexpected user mapping: %#v", data.Users)
	}
}

// Test_usersAPIToTerraform_absentOptionalFields checks absent optional user fields become null.
func Test_usersAPIToTerraform_absentOptionalFields(t *testing.T) {
	// The optional pointers are omitempty in the API types: a user the filter
	// accepts without them must map to nulls, not crash the provider.
	users := []api.User{{Id: "u4", Name: "Carol", Role: "user", Status: api.UserStatusInvited}}

	var data UsersModel
	if d := usersAPIToTerraform(context.Background(), users, &data); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	u := data.Users[0]
	if !u.LastLogin.IsNull() || !u.IsCurrent.IsNull() || !u.IsServiceUser.IsNull() || !u.Issued.IsNull() {
		t.Fatalf("Expected nulls for the absent fields, found %#v", u)
	}
	if u.Name.ValueString() != "Carol" || u.Status.ValueString() != "invited" {
		t.Fatalf("Unexpected user mapping: %#v", u)
	}
}
