package provider

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_userAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.User
		expected UserModel
	}{
		{
			resource: &api.User{
				Id:            "r1",
				AutoGroups:    []string{"g1"},
				Name:          "sk",
				LastLogin:     &timeNow,
				Email:         "me@me.com",
				IsBlocked:     true,
				IsCurrent:     valPtr(true),
				IsServiceUser: valPtr(true),
				Issued:        valPtr("api"),
				Role:          "admin",
				Status:        api.UserStatusActive,
			},
			expected: UserModel{
				Id:            types.StringValue("r1"),
				Name:          types.StringValue("sk"),
				AutoGroups:    types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
				LastLogin:     types.StringValue(timeNow.Format(time.RFC3339)),
				IsBlocked:     types.BoolValue(true),
				IsCurrent:     types.BoolValue(true),
				IsServiceUser: types.BoolValue(true),
				Issued:        types.StringValue("api"),
				Email:         types.StringValue("me@me.com"),
				Role:          types.StringValue("admin"),
				Status:        types.StringValue(string(api.UserStatusActive)),
			},
		},
	}

	for _, c := range cases {
		var out UserModel
		outDiag := userAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_userAPIToTerraform_keepsConfiguredServiceUser(t *testing.T) {
	// is_service_user is Required on netbird_user: an API response without it
	// must leave the configured value in place, not null it.
	out := UserModel{IsServiceUser: types.BoolValue(true)}
	if d := userAPIToTerraform(context.Background(), &api.User{Id: "r1", Status: api.UserStatusActive}, &out); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}
	if !out.IsServiceUser.Equal(types.BoolValue(true)) {
		t.Fatalf("Expected the configured is_service_user to be kept, found %s", out.IsServiceUser)
	}
}
