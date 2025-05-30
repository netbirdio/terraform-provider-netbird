package provider

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_tokenAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.PersonalAccessToken
		expected TokenModel
	}{
		{
			resource: &api.PersonalAccessToken{
				Id:             "r1",
				CreatedAt:      timeNow,
				CreatedBy:      "me",
				ExpirationDate: timeNow,
				LastUsed:       &timeNow,
				Name:           "test",
			},
			expected: TokenModel{
				Id:             types.StringValue("r1"),
				UserID:         types.StringValue("me"),
				Name:           types.StringValue("test"),
				CreatedAt:      types.StringValue(timeNow.Format(time.RFC3339)),
				ExpirationDate: types.StringValue(timeNow.Format(time.RFC3339)),
				LastUsed:       types.StringValue(timeNow.Format(time.RFC3339)),
			},
		},
	}

	for _, c := range cases {
		var out TokenModel
		tokenAPIToTerraform(c.resource, &out)

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
