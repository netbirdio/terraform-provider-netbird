package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strList(v ...string) types.List {
	e := make([]attr.Value, len(v))
	for i, s := range v {
		e[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, e)
}

func Test_preserveListOrder(t *testing.T) {
	cases := []struct {
		name           string
		prior, current types.List
		keepPrior      bool
	}{
		{"same order", strList("a", "b", "c"), strList("a", "b", "c"), true},
		{"reordered", strList("a", "b", "c"), strList("c", "a", "b"), true},
		{"both empty", strList(), strList(), true},
		{"member added", strList("a", "b"), strList("b", "a", "c"), false},
		{"member removed", strList("a", "b", "c"), strList("c", "a"), false},
		{"member replaced", strList("a", "b"), strList("b", "x"), false},
		{"duplicate differs", strList("a", "a", "b"), strList("a", "b", "b"), false},
		{"prior null", types.ListNull(types.StringType), strList("b", "a"), false},
		{"prior unknown", types.ListUnknown(types.StringType), strList("b", "a"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, d := preserveListOrder(context.Background(), c.prior, c.current)
			if d.HasError() {
				t.Fatalf("unexpected diagnostics: %v", d)
			}
			want := c.current
			if c.keepPrior {
				want = c.prior
			}
			if !got.Equal(want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}
