package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func tokenLimit(enabled attr.Value, groupCap, userCap, window attr.Value) types.Object {
	return types.ObjectValueMust(AgentNetworkTokenLimitModel{}.TFType().AttrTypes, map[string]attr.Value{
		"enabled":        enabled,
		"group_cap":      groupCap,
		"user_cap":       userCap,
		"window_seconds": window,
	})
}

func budgetLimit(enabled attr.Value, groupCap, userCap, window attr.Value) types.Object {
	return types.ObjectValueMust(AgentNetworkBudgetLimitModel{}.TFType().AttrTypes, map[string]attr.Value{
		"enabled":        enabled,
		"group_cap_usd":  groupCap,
		"user_cap_usd":   userCap,
		"window_seconds": window,
	})
}

// The server only enforces its limit rules when the limit is enabled, and
// rejects an enabled limit whose caps are both zero. Both must be caught at plan
// time rather than mid-apply, without over-restricting disabled limits.
func Test_validateLimit_tokenLimit(t *testing.T) {
	cases := []struct {
		name      string
		limit     types.Object
		wantError bool
	}{
		{
			name:      "disabled with zero caps is fine",
			limit:     tokenLimit(types.BoolValue(false), types.Int64Value(0), types.Int64Value(0), types.Int64Value(2592000)),
			wantError: false,
		},
		{
			// Regression: the schema's 0 defaults made this a mid-apply 422.
			name:      "enabled with both caps zero is rejected",
			limit:     tokenLimit(types.BoolValue(true), types.Int64Value(0), types.Int64Value(0), types.Int64Value(2592000)),
			wantError: true,
		},
		{
			name:      "enabled with omitted caps is rejected (they default to 0)",
			limit:     tokenLimit(types.BoolValue(true), types.Int64Null(), types.Int64Null(), types.Int64Value(2592000)),
			wantError: true,
		},
		{
			name:      "enabled with only group_cap set is fine",
			limit:     tokenLimit(types.BoolValue(true), types.Int64Value(1000), types.Int64Value(0), types.Int64Value(2592000)),
			wantError: false,
		},
		{
			name:      "enabled with only user_cap set is fine",
			limit:     tokenLimit(types.BoolValue(true), types.Int64Value(0), types.Int64Value(1000), types.Int64Value(2592000)),
			wantError: false,
		},
		{
			name:      "enabled below the 60s window floor is rejected",
			limit:     tokenLimit(types.BoolValue(true), types.Int64Value(1000), types.Int64Value(0), types.Int64Value(30)),
			wantError: true,
		},
		{
			// The server only applies the floor when enabled, so neither should we.
			name:      "disabled below the 60s window floor is fine",
			limit:     tokenLimit(types.BoolValue(false), types.Int64Value(0), types.Int64Value(0), types.Int64Value(30)),
			wantError: false,
		},
		{
			name:      "enabled with a negative cap is rejected",
			limit:     tokenLimit(types.BoolValue(true), types.Int64Value(-1), types.Int64Value(5), types.Int64Value(2592000)),
			wantError: true,
		},
		{
			name:      "null limit block is fine",
			limit:     types.ObjectNull(AgentNetworkTokenLimitModel{}.TFType().AttrTypes),
			wantError: false,
		},
		{
			// Values interpolated from other resources cannot be validated yet;
			// the server remains the backstop.
			name:      "unknown enabled is skipped",
			limit:     tokenLimit(types.BoolUnknown(), types.Int64Value(0), types.Int64Value(0), types.Int64Value(2592000)),
			wantError: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateLimit(c.limit, "token_limit", [2]string{"group_cap", "user_cap"}, &diags)

			if got := diags.HasError(); got != c.wantError {
				t.Fatalf("Expected error=%v, got error=%v (%v)", c.wantError, got, diags.Errors())
			}
		})
	}
}

func Test_validateLimit_budgetLimit(t *testing.T) {
	cases := []struct {
		name      string
		limit     types.Object
		wantError bool
	}{
		{
			name:      "enabled with both USD caps zero is rejected",
			limit:     budgetLimit(types.BoolValue(true), types.Float64Value(0), types.Float64Value(0), types.Int64Value(2592000)),
			wantError: true,
		},
		{
			name:      "enabled with a USD cap set is fine",
			limit:     budgetLimit(types.BoolValue(true), types.Float64Value(500), types.Float64Value(0), types.Int64Value(2592000)),
			wantError: false,
		},
		{
			name:      "enabled with a negative USD cap is rejected",
			limit:     budgetLimit(types.BoolValue(true), types.Float64Value(-1), types.Float64Value(50), types.Int64Value(2592000)),
			wantError: true,
		},
		{
			name:      "disabled is unconstrained",
			limit:     budgetLimit(types.BoolValue(false), types.Float64Value(0), types.Float64Value(0), types.Int64Value(1)),
			wantError: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateLimit(c.limit, "budget_limit", [2]string{"group_cap_usd", "user_cap_usd"}, &diags)

			if got := diags.HasError(); got != c.wantError {
				t.Fatalf("Expected error=%v, got error=%v (%v)", c.wantError, got, diags.Errors())
			}
		})
	}
}
