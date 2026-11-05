package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func stringList(vals ...string) types.List {
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

func Test_sortedStringList(t *testing.T) {
	cases := []struct {
		name     string
		prior    types.List
		values   []string
		expected types.List
	}{
		{
			name:     "no prior sorts",
			prior:    types.ListNull(types.StringType),
			values:   []string{"c", "a", "b"},
			expected: stringList("a", "b", "c"),
		},
		{
			name:     "unknown prior sorts",
			prior:    types.ListUnknown(types.StringType),
			values:   []string{"b", "a"},
			expected: stringList("a", "b"),
		},
		{
			name:     "zero value prior sorts",
			prior:    types.List{},
			values:   []string{"b", "a"},
			expected: stringList("a", "b"),
		},
		{
			name:     "prior with same elements keeps prior order",
			prior:    stringList("c", "a", "b"),
			values:   []string{"a", "b", "c"},
			expected: stringList("c", "a", "b"),
		},
		{
			name:     "prior with different elements sorts",
			prior:    stringList("c", "a"),
			values:   []string{"c", "b", "a"},
			expected: stringList("a", "b", "c"),
		},
		{
			name:     "duplicates must match in count",
			prior:    stringList("b", "a", "a"),
			values:   []string{"b", "b", "a"},
			expected: stringList("a", "b", "b"),
		},
		{
			name:     "empty prior and empty values keep prior",
			prior:    stringList(),
			values:   []string{},
			expected: stringList(),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, d := sortedStringList(context.Background(), c.prior, c.values)
			if d.HasError() {
				t.Fatalf("unexpected diagnostics: %v", d)
			}
			if !out.Equal(c.expected) {
				t.Errorf("expected %s, got %s", c.expected, out)
			}
		})
	}
}

func Test_sortedStringList_nilMatchesListValueFrom(t *testing.T) {
	var values []string
	expected, _ := types.ListValueFrom(context.Background(), types.StringType, values)
	out, d := sortedStringList(context.Background(), types.ListNull(types.StringType), values)
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !out.Equal(expected) {
		t.Errorf("nil slice should map like types.ListValueFrom: expected %s, got %s", expected, out)
	}
}

func Test_sortedStringListPointer(t *testing.T) {
	out, d := sortedStringListPointer(context.Background(), types.ListNull(types.StringType), nil)
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !out.IsNull() {
		t.Errorf("nil pointer should map to a null list, got %s", out)
	}

	out, d = sortedStringListPointer(context.Background(), types.ListNull(types.StringType), &[]string{"b", "a"})
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !out.Equal(stringList("a", "b")) {
		t.Errorf("expected sorted list, got %s", out)
	}
}

func Test_sortedList_objectsCompareNumerically(t *testing.T) {
	ctx := context.Background()
	rangeType := PolicyRulePortRangeModel{}.TFType()
	current, d := types.ListValueFrom(ctx, rangeType, []PolicyRulePortRangeModel{
		{Start: types.Int32Value(10), End: types.Int32Value(20)},
		{Start: types.Int32Value(9), End: types.Int32Value(15)},
		{Start: types.Int32Value(9), End: types.Int32Value(12)},
	})
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	expected, _ := types.ListValueFrom(ctx, rangeType, []PolicyRulePortRangeModel{
		{Start: types.Int32Value(9), End: types.Int32Value(12)},
		{Start: types.Int32Value(9), End: types.Int32Value(15)},
		{Start: types.Int32Value(10), End: types.Int32Value(20)},
	})

	out, d := sortedList(ctx, types.ListNull(rangeType), current)
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !out.Equal(expected) {
		t.Errorf("port ranges should sort by start then end: expected %s, got %s", expected, out)
	}
}

func Test_sortedList_nullCurrentStaysNull(t *testing.T) {
	out, d := sortedList(context.Background(), stringList("a"), types.ListNull(types.StringType))
	if d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !out.IsNull() {
		t.Errorf("null API value should stay null, got %s", out)
	}
}

func Test_objectListAttr(t *testing.T) {
	inner := types.ObjectValueMust(
		map[string]attr.Type{"groups": types.ListType{ElemType: types.StringType}},
		map[string]attr.Value{"groups": stringList("x")},
	)
	outer := types.ObjectValueMust(
		map[string]attr.Type{"auth": inner.Type(context.Background())},
		map[string]attr.Value{"auth": inner},
	)

	if got := objectListAttr(outer, "auth", "groups"); !got.Equal(stringList("x")) {
		t.Errorf("expected nested list, got %s", got)
	}
	if got := objectListAttr(outer, "auth", "missing"); !got.IsNull() {
		t.Errorf("missing attribute should give a null list, got %s", got)
	}
	if got := objectListAttr(types.ObjectNull(outer.AttributeTypes(context.Background())), "auth", "groups"); !got.IsNull() {
		t.Errorf("null object should give a null list, got %s", got)
	}
	if got := objectListAttr(outer, "auth"); !got.IsNull() {
		t.Errorf("non-list attribute should give a null list, got %s", got)
	}
}

func Test_groupAPIToTerraform_listOrder(t *testing.T) {
	group := &api.Group{
		Id:        "g1",
		Name:      "g",
		Peers:     []api.PeerMinimum{{Id: "p3"}, {Id: "p1"}, {Id: "p2"}},
		Resources: []api.Resource{},
	}

	// Without prior state (import, or computed), the API order is replaced by a sorted one
	var imported GroupModel
	if d := groupAPIToTerraform(context.Background(), group, &imported); d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !imported.Peers.Equal(stringList("p1", "p2", "p3")) {
		t.Errorf("peers should be sorted without prior state, got %s", imported.Peers)
	}

	// A configured order with the same members survives the round trip
	configured := GroupModel{Peers: stringList("p2", "p3", "p1")}
	if d := groupAPIToTerraform(context.Background(), group, &configured); d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !configured.Peers.Equal(stringList("p2", "p3", "p1")) {
		t.Errorf("configured peer order should be kept, got %s", configured.Peers)
	}
}

func Test_policyAPIToTerraform_listOrder(t *testing.T) {
	ruleID, policyID := "r1", "p1"
	policy := &api.Policy{
		Id:      &policyID,
		Name:    "p",
		Enabled: true,
		Rules: []api.PolicyRule{{
			Id:           &ruleID,
			Name:         "r",
			Action:       api.PolicyRuleActionAccept,
			Protocol:     api.PolicyRuleProtocolTcp,
			Enabled:      true,
			Sources:      &[]api.GroupMinimum{{Id: "s2"}, {Id: "s1"}},
			Destinations: &[]api.GroupMinimum{{Id: "d2"}, {Id: "d1"}},
			Ports:        &[]string{"80", "443"},
		}},
		SourcePostureChecks: []string{"pc2", "pc1"},
	}

	var imported PolicyModel
	if d := policyAPIToTerraform(context.Background(), policy, &imported); d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if !imported.SourcePostureChecks.Equal(stringList("pc1", "pc2")) {
		t.Errorf("posture checks should be sorted, got %s", imported.SourcePostureChecks)
	}
	rule := listElementObject(imported.Rules, 0)
	if got := objectListAttr(rule, "sources"); !got.Equal(stringList("s1", "s2")) {
		t.Errorf("rule sources should be sorted, got %s", got)
	}
	if got := objectListAttr(rule, "ports"); !got.Equal(stringList("443", "80")) {
		t.Errorf("rule ports should be sorted, got %s", got)
	}

	// Mapping again with the state as prior must keep the destinations order a user configured
	priorRule := rule.Attributes()
	priorRule["destinations"] = stringList("d2", "d1")
	ruleObj := types.ObjectValueMust(PolicyRuleModel{}.TFType().AttrTypes, priorRule)
	imported.Rules = types.ListValueMust(PolicyRuleModel{}.TFType(), []attr.Value{ruleObj})
	if d := policyAPIToTerraform(context.Background(), policy, &imported); d.HasError() {
		t.Fatalf("unexpected diagnostics: %v", d)
	}
	if got := objectListAttr(listElementObject(imported.Rules, 0), "destinations"); !got.Equal(stringList("d2", "d1")) {
		t.Errorf("configured destination order should be kept, got %s", got)
	}
}
