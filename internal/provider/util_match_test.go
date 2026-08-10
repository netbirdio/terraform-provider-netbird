// Copyright (c) HashiCorp, Inc.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The match* helpers score a candidate against a data source filter: 0 when the
// filter is absent, 1 on a match, and a large negative on a mismatch so that one
// failed filter outweighs any number of matching ones. The scoring is what makes
// a multi-filter data source lookup work, and it is pure — no API involved.

func Test_matchString(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    string
		filter   types.String
		expected int
	}{
		{name: "no filter", value: "abc", filter: types.StringNull(), expected: 0},
		{name: "unknown filter", value: "abc", filter: types.StringUnknown(), expected: 0},
		{name: "match", value: "abc", filter: types.StringValue("abc"), expected: 1},
		{name: "mismatch outweighs a match", value: "abc", filter: types.StringValue("def"), expected: -1000},
		{name: "empty filter is still a filter", value: "abc", filter: types.StringValue(""), expected: -1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchString(tc.value, tc.filter); got != tc.expected {
				t.Errorf("matchString(%q, %v) = %d, expected %d", tc.value, tc.filter, got, tc.expected)
			}
		})
	}
}

func Test_matchBool(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    bool
		filter   types.Bool
		expected int
	}{
		{name: "no filter", value: true, filter: types.BoolNull(), expected: 0},
		{name: "unknown filter", value: true, filter: types.BoolUnknown(), expected: 0},
		{name: "match true", value: true, filter: types.BoolValue(true), expected: 1},
		{name: "match false", value: false, filter: types.BoolValue(false), expected: 1},
		{name: "mismatch", value: true, filter: types.BoolValue(false), expected: -1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchBool(tc.value, tc.filter); got != tc.expected {
				t.Errorf("matchBool(%v, %v) = %d, expected %d", tc.value, tc.filter, got, tc.expected)
			}
		})
	}
}

func Test_matchInt32(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    int32
		filter   types.Int32
		expected int
	}{
		{name: "no filter", value: 7, filter: types.Int32Null(), expected: 0},
		{name: "unknown filter", value: 7, filter: types.Int32Unknown(), expected: 0},
		{name: "match", value: 7, filter: types.Int32Value(7), expected: 1},
		{name: "match zero", value: 0, filter: types.Int32Value(0), expected: 1},
		{name: "mismatch", value: 7, filter: types.Int32Value(8), expected: -1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchInt32(tc.value, tc.filter); got != tc.expected {
				t.Errorf("matchInt32(%d, %v) = %d, expected %d", tc.value, tc.filter, got, tc.expected)
			}
		})
	}
}

// A single mismatched filter has to sink a candidate no matter how many other
// filters matched — that is the reason for the -1000 rather than -1.
func Test_matchScoresCombineSoOneMismatchWins(t *testing.T) {
	score := matchString("abc", types.StringValue("abc")) +
		matchBool(true, types.BoolValue(true)) +
		matchInt32(7, types.Int32Value(7)) +
		matchString("abc", types.StringValue("nope"))
	if score > 0 {
		t.Errorf("three matches and one mismatch scored %d; a mismatch must not be outvoted", score)
	}
}

// stringListDefault and friends fall back to the supplied default when the
// configuration left the attribute out.
func Test_listDefaultsFallBackWhenUnset(t *testing.T) {
	ctx := t.Context()
	fallback := []string{"a", "b"}

	if got := stringListDefault(ctx, types.ListNull(types.StringType), fallback); len(got) != 2 {
		t.Errorf("stringListDefault on a null list should return the fallback, got %v", got)
	}
	if got := stringListDefault(ctx, types.ListUnknown(types.StringType), fallback); len(got) != 2 {
		t.Errorf("stringListDefault on an unknown list should return the fallback, got %v", got)
	}
	if got := stringSetDefault(ctx, types.SetNull(types.StringType), fallback); len(got) != 2 {
		t.Errorf("stringSetDefault on a null set should return the fallback, got %v", got)
	}
	if got := int32Default(types.Int32Null(), 42); got != 42 {
		t.Errorf("int32Default on a null value should return the fallback, got %d", got)
	}
	if got := int32Default(types.Int32Value(7), 42); got != 7 {
		t.Errorf("int32Default should prefer the configured value, got %d", got)
	}
	if got := stringListDefaultPointer(ctx, types.ListNull(types.StringType), &fallback); got == nil || len(*got) != 2 {
		t.Errorf("stringListDefaultPointer on a null list should return the fallback, got %v", got)
	}
}
