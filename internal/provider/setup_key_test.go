package provider

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_setupKeyAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.SetupKey
		expected SetupKeyModel
	}{
		{
			resource: &api.SetupKey{
				Id:                  "r1",
				AllowExtraDnsLabels: true,
				AutoGroups:          []string{"g1"},
				Ephemeral:           true,
				Expires:             timeNow,
				Key:                 "abc",
				LastUsed:            timeNow,
				Name:                "sk",
				Revoked:             true,
				State:               "active",
				Type:                "reusable",
				UpdatedAt:           timeNow,
				UsageLimit:          0,
				UsedTimes:           1,
				Valid:               true,
			},
			expected: SetupKeyModel{
				Id:                  types.StringValue("r1"),
				Key:                 types.StringNull(), // Key is added only in Create flow, so it's outside this method's scope
				Name:                types.StringValue("sk"),
				State:               types.StringValue("active"),
				Type:                types.StringValue("reusable"),
				AllowExtraDnsLabels: types.BoolValue(true),
				Ephemeral:           types.BoolValue(true),
				Revoked:             types.BoolValue(true),
				Valid:               types.BoolValue(true),
				Expires:             types.StringValue(timeNow.Format(time.RFC3339)),
				LastUsed:            types.StringValue(timeNow.Format(time.RFC3339)),
				UpdatedAt:           types.StringValue(timeNow.Format(time.RFC3339)),
				UsageLimit:          types.Int32Value(0),
				UsedTimes:           types.Int32Value(1),
				AutoGroups:          types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1")}),
			},
		},
	}

	for _, c := range cases {
		var out SetupKeyModel
		outDiag := setupKeyAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
