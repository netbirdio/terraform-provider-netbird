// Copyright (c) HashiCorp, Inc.

package provider

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func boolDefault(a types.Bool, b bool) bool {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	return a.ValueBool()
}

func boolDefaultPointer(a types.Bool, b *bool) *bool {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	return a.ValueBoolPointer()
}

func stringDefaultPointer(a types.String, b *string) *string {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	return a.ValueStringPointer()
}

func stringListDefault(ctx context.Context, a types.List, b []string) []string {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	var ret []string
	a.ElementsAs(ctx, &ret, false)
	return ret
}

func stringSetDefault(ctx context.Context, a types.Set, b []string) []string {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	var ret []string
	a.ElementsAs(ctx, &ret, false)
	return ret
}

func int32Default(a types.Int32, b int32) int32 {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	return a.ValueInt32()
}

func stringListDefaultPointer(ctx context.Context, a types.List, b *[]string) *[]string {
	if a.IsUnknown() || a.IsNull() {
		return b
	}
	var ret []string
	a.ElementsAs(ctx, &ret, false)
	return &ret
}

func matchString(a string, b types.String) int {
	if b.IsNull() || b.IsUnknown() {
		return 0
	}
	if b.ValueString() == a {
		return 1
	}
	return -1000
}

func matchBool(a bool, b types.Bool) int {
	if b.IsNull() || b.IsUnknown() {
		return 0
	}
	if b.ValueBool() == a {
		return 1
	}
	return -1000
}

func matchInt32(a int32, b types.Int32) int {
	if b.IsNull() || b.IsUnknown() {
		return 0
	}
	if b.ValueInt32() == a {
		return 1
	}
	return -1000
}

func matchListString(ctx context.Context, a []string, b types.List) (int, diag.Diagnostics) {
	if b.IsNull() || b.IsUnknown() {
		return 0, nil
	}
	var ba []string
	d := b.ElementsAs(ctx, &ba, false)
	if d.HasError() {
		return 0, d
	}
	for _, i := range ba {
		if !slices.Contains(a, i) {
			return -1000, d
		}
	}

	return 1, d
}

// sortedStringList builds a list from values with the semantics of sortedList.
func sortedStringList(ctx context.Context, prior types.List, values []string) (types.List, diag.Diagnostics) {
	l, d := types.ListValueFrom(ctx, types.StringType, values)
	if d.HasError() {
		return l, d
	}
	return sortedList(ctx, prior, l)
}

// sortedStringListPointer is sortedStringList for optional API slices; a nil pointer maps
// the same way types.ListValueFrom maps it.
func sortedStringListPointer(ctx context.Context, prior types.List, values *[]string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListValueFrom(ctx, types.StringType, values)
	}
	return sortedStringList(ctx, prior, *values)
}

// sortedList returns current with its elements sorted, so that the order the API happens to
// return does not leak into state. When prior holds the same elements in any order it is
// returned unchanged, which keeps a configured order stable and avoids inconsistent results.
func sortedList(ctx context.Context, prior, current types.List) (types.List, diag.Diagnostics) {
	if current.IsNull() || current.IsUnknown() {
		return current, nil
	}
	if sameElements(prior, current) {
		return prior, nil
	}
	elems := slices.Clone(current.Elements())
	slices.SortStableFunc(elems, compareAttrValues)
	return types.ListValue(current.ElementType(ctx), elems)
}

func sameElements(a, b types.List) bool {
	if a.IsNull() || a.IsUnknown() || b.IsNull() || b.IsUnknown() {
		return false
	}
	ae, be := a.Elements(), b.Elements()
	if len(ae) != len(be) {
		return false
	}
	used := make([]bool, len(be))
	for _, x := range ae {
		found := false
		for i, y := range be {
			if !used[i] && x.Equal(y) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// compareAttrValues orders attribute values of the same type, nulls first. Objects compare
// attribute by attribute in name order.
func compareAttrValues(a, b attr.Value) int {
	if a.IsNull() || b.IsNull() {
		return cmp.Compare(boolRank(!a.IsNull()), boolRank(!b.IsNull()))
	}
	switch av := a.(type) {
	case types.String:
		if bv, ok := b.(types.String); ok {
			return strings.Compare(av.ValueString(), bv.ValueString())
		}
	case types.Int32:
		if bv, ok := b.(types.Int32); ok {
			return cmp.Compare(av.ValueInt32(), bv.ValueInt32())
		}
	case types.Int64:
		if bv, ok := b.(types.Int64); ok {
			return cmp.Compare(av.ValueInt64(), bv.ValueInt64())
		}
	case types.Float64:
		if bv, ok := b.(types.Float64); ok {
			return cmp.Compare(av.ValueFloat64(), bv.ValueFloat64())
		}
	case types.Bool:
		if bv, ok := b.(types.Bool); ok {
			return cmp.Compare(boolRank(av.ValueBool()), boolRank(bv.ValueBool()))
		}
	case types.Object:
		if bv, ok := b.(types.Object); ok {
			return compareObjects(av, bv)
		}
	}
	return strings.Compare(a.String(), b.String())
}

func compareObjects(a, b types.Object) int {
	aa, ba := a.Attributes(), b.Attributes()
	names := slices.Sorted(maps.Keys(aa))
	for _, name := range names {
		bv, ok := ba[name]
		if !ok {
			continue
		}
		if c := compareAttrValues(aa[name], bv); c != 0 {
			return c
		}
	}
	return 0
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// objectListAttr walks nested object attributes by name and returns the list found at the
// end of the path, or a null list when any step is missing, null or unknown.
func objectListAttr(obj types.Object, names ...string) types.List {
	null := types.ListNull(types.StringType)
	var cur attr.Value = obj
	for _, name := range names {
		o, ok := cur.(types.Object)
		if !ok || o.IsNull() || o.IsUnknown() {
			return null
		}
		if cur, ok = o.Attributes()[name]; !ok {
			return null
		}
	}
	if l, ok := cur.(types.List); ok {
		return l
	}
	return null
}

// listElementObject returns the i-th element of l as an object, or a null object when l is
// not known or shorter than i.
func listElementObject(l types.List, i int) types.Object {
	null := types.ObjectNull(map[string]attr.Type{})
	if l.IsNull() || l.IsUnknown() || i >= len(l.Elements()) {
		return null
	}
	if o, ok := l.Elements()[i].(types.Object); ok {
		return o
	}
	return null
}

func knownCount(vals ...attr.Value) int {
	ret := 0
	for _, v := range vals {
		if !v.IsNull() && !v.IsUnknown() {
			ret++
		}
	}
	return ret
}
