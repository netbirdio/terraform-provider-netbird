package provider

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_tokenAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		name     string
		resource *api.PersonalAccessToken
		in       TokenModel
		expected TokenModel
	}{
		{
			name: "maps the response",
			resource: &api.PersonalAccessToken{
				Id:             "r1",
				CreatedAt:      timeNow,
				ExpirationDate: timeNow,
				LastUsed:       &timeNow,
				Name:           "test",
			},
			expected: TokenModel{
				Id:             types.StringValue("r1"),
				UserID:         types.StringNull(),
				Name:           types.StringValue("test"),
				CreatedAt:      types.StringValue(timeNow.Format(time.RFC3339)),
				ExpirationDate: types.StringValue(timeNow.Format(time.RFC3339)),
				LastUsed:       types.StringValue(timeNow.Format(time.RFC3339)),
			},
		},
		{
			// The import case: nothing but the two timestamps to work from.
			name: "recovers the lifetime when state has none",
			resource: &api.PersonalAccessToken{
				Id:             "r2",
				CreatedAt:      timeNow,
				ExpirationDate: timeNow.AddDate(0, 0, 180),
				Name:           "test",
			},
			expected: TokenModel{
				Id:             types.StringValue("r2"),
				UserID:         types.StringNull(),
				Name:           types.StringValue("test"),
				CreatedAt:      types.StringValue(timeNow.Format(time.RFC3339)),
				ExpirationDate: types.StringValue(timeNow.AddDate(0, 0, 180).Format(time.RFC3339)),
				ExpirationDays: types.Int32Value(180),
				LastUsed:       types.StringNull(),
			},
		},
		{
			// A configured lifetime is what the user asked for, and it is the
			// value the token was created with. Recomputing it from timestamps
			// could only introduce a difference, and a difference on this
			// attribute recreates the token.
			name: "leaves a known lifetime alone",
			resource: &api.PersonalAccessToken{
				Id:             "r3",
				CreatedAt:      timeNow,
				ExpirationDate: timeNow.AddDate(0, 0, 180),
				Name:           "test",
			},
			in: TokenModel{ExpirationDays: types.Int32Value(30)},
			expected: TokenModel{
				Id:             types.StringValue("r3"),
				UserID:         types.StringNull(),
				Name:           types.StringValue("test"),
				CreatedAt:      types.StringValue(timeNow.Format(time.RFC3339)),
				ExpirationDate: types.StringValue(timeNow.AddDate(0, 0, 180).Format(time.RFC3339)),
				ExpirationDays: types.Int32Value(30),
				LastUsed:       types.StringNull(),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := c.in
			tokenAPIToTerraform(c.resource, &out)

			if !reflect.DeepEqual(out, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
			}
		})
	}
}

func Test_tokenExpirationDays(t *testing.T) {
	// AddDate is how the server derives the expiry, and it is called on a UTC
	// timestamp, where a calendar day and 24 hours are the same span.
	created := time.Date(2026, 3, 8, 23, 30, 0, 0, time.UTC)
	for _, days := range []int{1, 30, 180, 365} {
		if got := tokenExpirationDays(created, created.AddDate(0, 0, days)); got != int32(days) {
			t.Errorf("expected %d days, got %d", days, got)
		}
	}

	// Whatever a round trip through storage and JSON does to the sub-second
	// part, it must not change the number of days.
	drifted := created.AddDate(0, 0, 90).Add(-400 * time.Millisecond)
	if got := tokenExpirationDays(created, drifted); got != 90 {
		t.Errorf("expected 90 days despite drift, got %d", got)
	}
}
