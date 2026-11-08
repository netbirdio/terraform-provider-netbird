package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyPrivateRoundTrip(t *testing.T) {
	ctx := context.Background()
	groups := []string{"group-b", "group-a"}
	configured, diags := types.ListValueFrom(ctx, types.StringType, groups)
	require.False(t, diags.HasError(), "group configuration must decode")
	model := ReverseProxyServiceModel{AccessGroups: configured}
	svc := &api.Service{Private: valPtr(true), AccessGroups: &[]string{"group-a", "group-b"}}
	diags = reverseProxyServiceAPIToTerraform(ctx, svc, &model)
	require.False(t, diags.HasError(), "private response must decode")
	require.True(t, model.Private.ValueBool(), "refresh must retain private access")
	require.True(t, model.AccessGroups.Equal(configured), "refresh must preserve configured group order")
	request, diags := reverseProxyServiceTerraformToAPI(ctx, &model)
	require.False(t, diags.HasError(), "private request must encode")
	require.Equal(t, valPtr(true), request.Private, "an unrelated update must preserve private access")
	require.Equal(t, &groups, request.AccessGroups, "an unrelated update must retain allowed groups")

	// Import starts without prior state and must recover both access fields.
	imported := ReverseProxyServiceModel{}
	diags = reverseProxyServiceAPIToTerraform(ctx, svc, &imported)
	require.False(t, diags.HasError(), "import response must decode")
	require.True(t, imported.Private.ValueBool(), "import must recover private access")
	var importedGroups []string
	require.False(t, imported.AccessGroups.ElementsAs(ctx, &importedGroups, false).HasError(), "imported groups must decode")
	require.Equal(t, []string{"group-a", "group-b"}, importedGroups, "import must retain all group IDs")

	// Explicitly returning a service to public must send false and an empty array,
	// not omit either field through JSON omitempty handling.
	model.Private = types.BoolValue(false)
	model.AccessGroups = types.ListValueMust(types.StringType, nil)
	request, diags = reverseProxyServiceTerraformToAPI(ctx, &model)
	require.False(t, diags.HasError(), "public update must encode")
	payload, err := json.Marshal(request)
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &body))
	require.JSONEq(t, "false", string(body["private"]), "public transition must send false")
	require.JSONEq(t, "[]", string(body["access_groups"]), "public transition must clear groups")
}

func TestReverseProxyPrivateRefresh(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		private *bool
		groups  *[]string
	}{
		{name: "older server omits fields"},
		{name: "public service", private: valPtr(false)},
		{name: "changed private groups", private: valPtr(true), groups: &[]string{"replacement"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := ReverseProxyServiceModel{Private: types.BoolValue(true)}
			var diags = reverseProxyServiceAPIToTerraform(ctx, &api.Service{Private: tc.private, AccessGroups: tc.groups}, &model)
			require.False(t, diags.HasError(), "refresh must decode")
			require.Equal(t, tc.private != nil && *tc.private, model.Private.ValueBool(), "refresh must reflect server visibility")
			if tc.groups == nil {
				require.True(t, model.AccessGroups.Equal(types.ListValueMust(types.StringType, nil)), "omitted groups must normalize to an empty list")
			} else {
				var groups []string
				require.False(t, model.AccessGroups.ElementsAs(ctx, &groups, false).HasError(), "groups must decode")
				require.Equal(t, *tc.groups, groups, "refresh must expose changed groups")
			}
		})
	}
}

func TestReverseProxyPrivateUnconfiguredRequest(t *testing.T) {
	ctx := context.Background()
	for _, private := range []types.Bool{types.BoolNull(), types.BoolUnknown()} {
		for _, groups := range []types.List{types.ListNull(types.StringType), types.ListUnknown(types.StringType)} {
			model := ReverseProxyServiceModel{Private: private, AccessGroups: groups, Targets: types.ListNull(ReverseProxyServiceTargetModel{}.TFType())}
			request, diags := reverseProxyServiceTerraformToAPI(ctx, &model)
			require.False(t, diags.HasError(), "unset or unknown access fields must not cause conversion errors")
			require.Nil(t, request.Private, "unset visibility must be omitted")
			require.Nil(t, request.AccessGroups, "unset groups must be omitted")
		}
	}
}

func TestPrivateServiceValidation(t *testing.T) {
	groups := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("all")})
	bearer := func(enabled types.Bool) types.Object {
		b := types.ObjectValueMust(map[string]attr.Type{"enabled": types.BoolType}, map[string]attr.Value{"enabled": enabled})
		return types.ObjectValueMust(map[string]attr.Type{"bearer_auth": b.Type(context.Background())}, map[string]attr.Value{"bearer_auth": b})
	}
	for _, tc := range []struct {
		name      string
		mode      types.String
		groups    types.List
		auth      types.Object
		wantError bool
	}{
		{"valid", types.StringValue("http"), groups, types.ObjectNull(map[string]attr.Type{}), false},
		{"tcp", types.StringValue("tcp"), groups, types.ObjectNull(map[string]attr.Type{}), true},
		{"empty groups", types.StringValue("http"), types.ListValueMust(types.StringType, nil), types.ObjectNull(map[string]attr.Type{}), true},
		{"enabled bearer", types.StringValue("http"), groups, bearer(types.BoolValue(true)), true},
		{"disabled bearer", types.StringValue("http"), groups, bearer(types.BoolValue(false)), false},
		{"unknown bearer", types.StringValue("http"), groups, bearer(types.BoolUnknown()), false},
		{"unknown fields", types.StringUnknown(), types.ListUnknown(types.StringType), types.ObjectUnknown(map[string]attr.Type{}), false},
		{"imported groups", types.StringValue("http"), types.ListNull(types.StringType), types.ObjectNull(map[string]attr.Type{}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := ReverseProxyServiceModel{Private: types.BoolValue(true), Mode: tc.mode, AccessGroups: tc.groups, Auth: tc.auth}
			require.Equal(t, tc.wantError, validatePrivateService(&model).HasError())
			model.Private = types.BoolValue(false)
			require.False(t, validatePrivateService(&model).HasError())
			model.Private = types.BoolUnknown()
			require.False(t, validatePrivateService(&model).HasError())
		})
	}
}
