package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_agentNetworkProviderAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.AgentNetworkProvider
		expected AgentNetworkProviderModel
	}{
		{
			resource: &api.AgentNetworkProvider{
				Id:                  "p1",
				ProviderId:          "openai_api",
				Name:                "OpenAI",
				UpstreamUrl:         "https://api.openai.com",
				Enabled:             true,
				SkipTlsVerification: false,
				MetadataDisabled:    true,
				ExtraValues:         &map[string]string{"x-portkey-config": "pc-abc123"},
				Models: []api.AgentNetworkProviderModel{
					{Id: "gpt-4o-mini", InputPer1k: 0.00015, OutputPer1k: 0.0006},
				},
			},
			expected: AgentNetworkProviderModel{
				Id:                  types.StringValue("p1"),
				ProviderId:          types.StringValue("openai_api"),
				Name:                types.StringValue("OpenAI"),
				UpstreamUrl:         types.StringValue("https://api.openai.com"),
				Enabled:             types.BoolValue(true),
				SkipTlsVerification: types.BoolValue(false),
				MetadataDisabled:    types.BoolValue(true),
				ExtraValues: types.MapValueMust(types.StringType, map[string]attr.Value{
					"x-portkey-config": types.StringValue("pc-abc123"),
				}),
				Models: types.ListValueMust(AgentNetworkProviderModelItem{}.TFType(), []attr.Value{
					types.ObjectValueMust(AgentNetworkProviderModelItem{}.TFType().AttrTypes, map[string]attr.Value{
						"id":            types.StringValue("gpt-4o-mini"),
						"input_per_1k":  types.Float64Value(0.00015),
						"output_per_1k": types.Float64Value(0.0006),
					}),
				}),
			},
		},
		{
			// No extra_values / identity headers in the response: extra_values
			// becomes a null map, not an empty one.
			resource: &api.AgentNetworkProvider{
				Id:          "p2",
				ProviderId:  "custom",
				Name:        "Custom",
				UpstreamUrl: "https://example.com",
			},
			expected: AgentNetworkProviderModel{
				Id:                  types.StringValue("p2"),
				ProviderId:          types.StringValue("custom"),
				Name:                types.StringValue("Custom"),
				UpstreamUrl:         types.StringValue("https://example.com"),
				Enabled:             types.BoolValue(false),
				MetadataDisabled:    types.BoolValue(false),
				SkipTlsVerification: types.BoolValue(false),
				ExtraValues:         types.MapNull(types.StringType),
				Models:              types.ListValueMust(AgentNetworkProviderModelItem{}.TFType(), []attr.Value{}),
			},
		},
	}

	for _, c := range cases {
		var out AgentNetworkProviderModel
		outDiag := agentNetworkProviderAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

// An empty identity-header value means "disable stamping for this dimension",
// but the API omits empty values from responses. Without preserving the
// configured "" the apply would fail with "provider produced inconsistent
// result after apply ... was cty.StringVal(\"\"), but now null".
func Test_agentNetworkProviderAPIToTerraform_preservesEmptyIdentityHeaders(t *testing.T) {
	out := AgentNetworkProviderModel{
		IdentityHeaderUserId: types.StringValue(""),
		IdentityHeaderGroups: types.StringValue(""),
	}

	res := &api.AgentNetworkProvider{Id: "p1", ProviderId: "custom", Name: "n", UpstreamUrl: "u"}
	if d := agentNetworkProviderAPIToTerraform(context.Background(), res, &out); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	if out.IdentityHeaderUserId != types.StringValue("") {
		t.Errorf("identity_header_user_id: expected preserved \"\", got %#v", out.IdentityHeaderUserId)
	}
	if out.IdentityHeaderGroups != types.StringValue("") {
		t.Errorf("identity_header_groups: expected preserved \"\", got %#v", out.IdentityHeaderGroups)
	}

	// An unset (null) header must stay null rather than becoming "".
	unset := AgentNetworkProviderModel{}
	if d := agentNetworkProviderAPIToTerraform(context.Background(), res, &unset); d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}
	if !unset.IdentityHeaderUserId.IsNull() {
		t.Errorf("unset identity_header_user_id: expected null, got %#v", unset.IdentityHeaderUserId)
	}
}

// The server rebuilds the provider row from the request on update, carrying over
// only the API key and session keys. metadata_disabled and extra_values must
// therefore be sent whenever known, or an update silently wipes them.
func Test_agentNetworkProviderTerraformToRequest_sendsMetadataAndExtraValues(t *testing.T) {
	data := AgentNetworkProviderModel{
		ProviderId:       types.StringValue("openai_api"),
		Name:             types.StringValue("OpenAI"),
		UpstreamUrl:      types.StringValue("https://api.openai.com"),
		MetadataDisabled: types.BoolValue(true),
		ExtraValues: types.MapValueMust(types.StringType, map[string]attr.Value{
			"x-portkey-config": types.StringValue("pc-abc123"),
		}),
	}

	req, d := agentNetworkProviderTerraformToRequest(context.Background(), &data)
	if d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	if req.MetadataDisabled == nil || !*req.MetadataDisabled {
		t.Errorf("expected metadata_disabled to be sent as true, got %v", req.MetadataDisabled)
	}
	if req.ExtraValues == nil {
		t.Fatal("expected extra_values to be sent, got nil (would wipe stored values)")
	}
	if got := (*req.ExtraValues)["x-portkey-config"]; got != "pc-abc123" {
		t.Errorf("expected extra_values passthrough, got %q", got)
	}

	// Unknown values must be omitted so a create does not send placeholders.
	unknown := AgentNetworkProviderModel{
		ProviderId:       types.StringValue("custom"),
		Name:             types.StringValue("n"),
		UpstreamUrl:      types.StringValue("u"),
		MetadataDisabled: types.BoolUnknown(),
		ExtraValues:      types.MapUnknown(types.StringType),
	}
	req, d = agentNetworkProviderTerraformToRequest(context.Background(), &unknown)
	if d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}
	if req.MetadataDisabled != nil {
		t.Errorf("expected unknown metadata_disabled to be omitted, got %v", *req.MetadataDisabled)
	}
	if req.ExtraValues != nil {
		t.Errorf("expected unknown extra_values to be omitted, got %v", *req.ExtraValues)
	}
}
