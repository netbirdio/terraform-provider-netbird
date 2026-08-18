package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// object is the shape of a raw state or plan value. Only its null-ness is read
// by the modifier, so one attribute is enough to make it a value rather than
// nothing.
var rawObject = tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}}

func rawNull() tftypes.Value { return tftypes.NewValue(rawObject, nil) }
func rawKnown() tftypes.Value {
	return tftypes.NewValue(rawObject, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "k1")})
}

func Test_expirySecondsPlanModifier(t *testing.T) {
	cases := []struct {
		name        string
		state, plan tftypes.Value
		stateValue  types.Int32
		planValue   types.Int32
		wantReplace bool
	}{
		{
			name:  "creating",
			state: rawNull(), plan: rawKnown(),
			stateValue: types.Int32Null(), planValue: types.Int32Value(1800),
		},
		{
			name:  "destroying",
			state: rawKnown(), plan: rawNull(),
			stateValue: types.Int32Value(1800), planValue: types.Int32Null(),
		},
		{
			// The import case: the API never reported the value, so state has
			// none. Replacing here would destroy the key that was just adopted.
			name:  "adopting an imported key",
			state: rawKnown(), plan: rawKnown(),
			stateValue: types.Int32Null(), planValue: types.Int32Value(1800),
		},
		{
			name:  "unchanged",
			state: rawKnown(), plan: rawKnown(),
			stateValue: types.Int32Value(1800), planValue: types.Int32Value(1800),
		},
		{
			// The server cannot move an expiry date, so this is a new key.
			name:  "changed",
			state: rawKnown(), plan: rawKnown(),
			stateValue: types.Int32Value(1800), planValue: types.Int32Value(3600),
			wantReplace: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := &planmodifier.Int32Response{PlanValue: c.planValue}
			expirySecondsPlanModifier{}.PlanModifyInt32(context.Background(), planmodifier.Int32Request{
				State:      tfsdk.State{Raw: c.state},
				Plan:       tfsdk.Plan{Raw: c.plan},
				StateValue: c.stateValue,
				PlanValue:  c.planValue,
			}, resp)

			if resp.RequiresReplace != c.wantReplace {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, c.wantReplace)
			}
		})
	}
}
