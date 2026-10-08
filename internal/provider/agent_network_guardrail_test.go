package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func guardrailChecks(allowEnabled bool, models []string, captureEnabled, redactPii bool) api.AgentNetworkGuardrailChecks {
	var c api.AgentNetworkGuardrailChecks
	c.ModelAllowlist.Enabled = allowEnabled
	c.ModelAllowlist.Models = models
	c.PromptCapture.Enabled = captureEnabled
	c.PromptCapture.RedactPii = redactPii
	return c
}

func Test_agentNetworkGuardrailAPIToTerraform(t *testing.T) {
	cases := []struct {
		name     string
		resource *api.AgentNetworkGuardrail
		expected AgentNetworkGuardrailModel
	}{
		{
			name: "populated",
			resource: &api.AgentNetworkGuardrail{
				Id:          "g1",
				Name:        "strict",
				Description: "tight allowlist",
				Checks:      guardrailChecks(true, []string{"gpt-4o-mini", "gpt-4o"}, true, true),
			},
			expected: AgentNetworkGuardrailModel{
				Id:          types.StringValue("g1"),
				Name:        types.StringValue("strict"),
				Description: types.StringValue("tight allowlist"),
				ModelAllowlist: types.ObjectValueMust(AgentNetworkModelAllowlistModel{}.TFType().AttrTypes, map[string]attr.Value{
					"enabled": types.BoolValue(true),
					"models": types.ListValueMust(types.StringType, []attr.Value{
						types.StringValue("gpt-4o-mini"), types.StringValue("gpt-4o"),
					}),
				}),
				PromptCapture: types.ObjectValueMust(AgentNetworkPromptCaptureModel{}.TFType().AttrTypes, map[string]attr.Value{
					"enabled":    types.BoolValue(true),
					"redact_pii": types.BoolValue(true),
				}),
			},
		},
		{
			// The server currently normalizes nil to []string{}, but a nil slice
			// would convert to a NULL list rather than an empty one. Pinning the
			// empty-slice case documents the boundary that keeps `models = []`
			// from tripping the plan/apply consistency check.
			name: "empty models list stays empty, not null",
			resource: &api.AgentNetworkGuardrail{
				Id:     "g2",
				Name:   "loose",
				Checks: guardrailChecks(false, []string{}, false, false),
			},
			expected: AgentNetworkGuardrailModel{
				Id:          types.StringValue("g2"),
				Name:        types.StringValue("loose"),
				Description: types.StringValue(""),
				ModelAllowlist: types.ObjectValueMust(AgentNetworkModelAllowlistModel{}.TFType().AttrTypes, map[string]attr.Value{
					"enabled": types.BoolValue(false),
					"models":  types.ListValueMust(types.StringType, []attr.Value{}),
				}),
				PromptCapture: types.ObjectValueMust(AgentNetworkPromptCaptureModel{}.TFType().AttrTypes, map[string]attr.Value{
					"enabled":    types.BoolValue(false),
					"redact_pii": types.BoolValue(false),
				}),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out AgentNetworkGuardrailModel
			outDiag := agentNetworkGuardrailAPIToTerraform(context.Background(), c.resource, &out)
			if outDiag.HasError() {
				t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
			}

			if !reflect.DeepEqual(out, c.expected) {
				t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
			}
		})
	}
}

// A nil Models slice from the API yields a null list, which does not round-trip
// as an empty list. If the server ever stops normalizing nil to []string{} this
// test documents the resulting difference instead of it surfacing as a confusing
// "inconsistent result after apply" for users.
func Test_agentNetworkGuardrailAPIToTerraform_nilModelsIsNullList(t *testing.T) {
	res := &api.AgentNetworkGuardrail{Id: "g3", Name: "n", Checks: guardrailChecks(false, nil, false, false)}

	var out AgentNetworkGuardrailModel
	if d := agentNetworkGuardrailAPIToTerraform(context.Background(), res, &out); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	models, ok := out.ModelAllowlist.Attributes()["models"].(types.List)
	if !ok {
		t.Fatal("models attribute is not a list")
	}
	if !models.IsNull() {
		t.Errorf("expected a nil API slice to produce a null list, got %#v", models)
	}
}

func Test_agentNetworkGuardrailTerraformToRequest(t *testing.T) {
	data := AgentNetworkGuardrailModel{
		Name:        types.StringValue("strict"),
		Description: types.StringValue("desc"),
		ModelAllowlist: types.ObjectValueMust(AgentNetworkModelAllowlistModel{}.TFType().AttrTypes, map[string]attr.Value{
			"enabled": types.BoolValue(true),
			"models":  types.ListValueMust(types.StringType, []attr.Value{types.StringValue("gpt-4o")}),
		}),
		PromptCapture: types.ObjectValueMust(AgentNetworkPromptCaptureModel{}.TFType().AttrTypes, map[string]attr.Value{
			"enabled":    types.BoolValue(true),
			"redact_pii": types.BoolValue(false),
		}),
	}

	req, d := agentNetworkGuardrailTerraformToRequest(context.Background(), &data)
	if d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	if req.Name != "strict" {
		t.Errorf("Name mismatch: got %q", req.Name)
	}
	if req.Description == nil || *req.Description != "desc" {
		t.Errorf("Description mismatch: got %v", req.Description)
	}
	if !req.Checks.ModelAllowlist.Enabled {
		t.Error("expected model_allowlist.enabled true")
	}
	if got := req.Checks.ModelAllowlist.Models; len(got) != 1 || got[0] != "gpt-4o" {
		t.Errorf("Models mismatch: got %v", got)
	}
	if !req.Checks.PromptCapture.Enabled || req.Checks.PromptCapture.RedactPii {
		t.Errorf("PromptCapture mismatch: %+v", req.Checks.PromptCapture)
	}
}
