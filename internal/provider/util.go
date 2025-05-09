// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func boolDefault(a types.Bool, b bool) bool {
	if a.IsUnknown() {
		return b
	}
	return a.ValueBool()
}

func boolDefaultPointer(a types.Bool, b *bool) *bool {
	if a.IsUnknown() {
		return b
	}
	return a.ValueBoolPointer()
}

func stringDefaultPointer(a types.String, b *string) *string {
	if a.IsUnknown() {
		return b
	}
	return a.ValueStringPointer()
}

func stringListDefault(ctx context.Context, a types.List, b []string) []string {
	if a.IsUnknown() {
		return b
	}
	var ret []string
	a.ElementsAs(ctx, &ret, false)
	return ret
}

func int32Default(a types.Int32, b int32) int32 {
	if a.IsUnknown() {
		return b
	}
	return a.ValueInt32()
}

func stringListDefaultPointer(ctx context.Context, a types.List, b *[]string) *[]string {
	if a.IsUnknown() {
		return b
	}
	var ret []string
	a.ElementsAs(ctx, &ret, false)
	return &ret
}
