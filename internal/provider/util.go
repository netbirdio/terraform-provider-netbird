// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"slices"

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

func knownCount(vals ...attr.Value) int {
	ret := 0
	for _, v := range vals {
		if !v.IsNull() && !v.IsUnknown() {
			ret++
		}
	}
	return ret
}
