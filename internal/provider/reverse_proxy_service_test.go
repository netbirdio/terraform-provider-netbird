package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_reverseProxyServiceAPIToTerraform(t *testing.T) {
	cases := []struct {
		name    string
		service *api.Service
	}{
		{
			name: "basic service with password auth",
			service: &api.Service{
				Id:      "svc1",
				Name:    "test-service",
				Domain:  "test.example.com",
				Enabled: true,
				Targets: []api.ServiceTarget{
					{
						TargetId:   "peer1",
						TargetType: api.ServiceTargetTargetTypePeer,
						Port:       8080,
						Protocol:   api.ServiceTargetProtocolHttp,
						Enabled:    true,
					},
				},
				Auth: api.ServiceAuthConfig{
					PasswordAuth: &api.PasswordAuthConfig{
						Enabled:  true,
						Password: "secret",
					},
				},
			},
		},
		{
			name: "service with optional fields and bearer auth",
			service: &api.Service{
				Id:               "svc2",
				Name:             "advanced-service",
				Domain:           "app.example.com",
				Enabled:          false,
				PassHostHeader:   valPtr(true),
				RewriteRedirects: valPtr(true),
				Targets: []api.ServiceTarget{
					{
						TargetId:   "res1",
						TargetType: api.ServiceTargetTargetType("subnet"),
						Port:       443,
						Protocol:   api.ServiceTargetProtocolHttps,
						Enabled:    true,
						Host:       valPtr("backend.internal"),
						Path:       valPtr("/api"),
					},
				},
				Auth: api.ServiceAuthConfig{
					BearerAuth: &api.BearerAuthConfig{
						Enabled:            true,
						DistributionGroups: &[]string{"group1", "group2"},
					},
				},
			},
		},
		{
			name: "service with pin auth and nil optional pointers",
			service: &api.Service{
				Id:      "svc3",
				Name:    "pin-service",
				Domain:  "pin.example.com",
				Enabled: true,
				Targets: []api.ServiceTarget{
					{
						TargetId:   "peer2",
						TargetType: api.ServiceTargetTargetTypePeer,
						Port:       0,
						Protocol:   api.ServiceTargetProtocolHttp,
						Enabled:    false,
					},
				},
				Auth: api.ServiceAuthConfig{
					PinAuth: &api.PINAuthConfig{
						Enabled: true,
						Pin:     "1234",
					},
				},
			},
		},
		{
			name: "service with link auth",
			service: &api.Service{
				Id:      "svc4",
				Name:    "link-service",
				Domain:  "link.example.com",
				Enabled: true,
				Targets: []api.ServiceTarget{
					{
						TargetId:   "peer1",
						TargetType: api.ServiceTargetTargetTypePeer,
						Port:       80,
						Protocol:   api.ServiceTargetProtocolHttp,
						Enabled:    true,
					},
				},
				Auth: api.ServiceAuthConfig{
					LinkAuth: &api.LinkAuthConfig{
						Enabled: true,
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out ReverseProxyServiceModel
			ctx := context.Background()
			outDiag := reverseProxyServiceAPIToTerraform(ctx, c.service, &out)
			if outDiag.HasError() {
				t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
			}

			if out.Id.ValueString() != c.service.Id {
				t.Errorf("Id mismatch: expected %s, got %s", c.service.Id, out.Id.ValueString())
			}
			if out.Name.ValueString() != c.service.Name {
				t.Errorf("Name mismatch: expected %s, got %s", c.service.Name, out.Name.ValueString())
			}
			if out.Domain.ValueString() != c.service.Domain {
				t.Errorf("Domain mismatch: expected %s, got %s", c.service.Domain, out.Domain.ValueString())
			}
			if out.Enabled.ValueBool() != c.service.Enabled {
				t.Errorf("Enabled mismatch: expected %v, got %v", c.service.Enabled, out.Enabled.ValueBool())
			}

			expectedPassHost := false
			if c.service.PassHostHeader != nil {
				expectedPassHost = *c.service.PassHostHeader
			}
			if out.PassHostHeader.ValueBool() != expectedPassHost {
				t.Errorf("PassHostHeader mismatch: expected %v, got %v", expectedPassHost, out.PassHostHeader.ValueBool())
			}

			expectedRewrite := false
			if c.service.RewriteRedirects != nil {
				expectedRewrite = *c.service.RewriteRedirects
			}
			if out.RewriteRedirects.ValueBool() != expectedRewrite {
				t.Errorf("RewriteRedirects mismatch: expected %v, got %v", expectedRewrite, out.RewriteRedirects.ValueBool())
			}
		})
	}
}

func Test_reverseProxyServiceRoundtrip(t *testing.T) {
	ctx := context.Background()

	original := &api.Service{
		Id:               "svc-rt",
		Name:             "roundtrip-service",
		Domain:           "rt.example.com",
		Enabled:          true,
		PassHostHeader:   valPtr(true),
		RewriteRedirects: valPtr(false),
		Targets: []api.ServiceTarget{
			{
				TargetId:   "peer1",
				TargetType: api.ServiceTargetTargetTypePeer,
				Port:       8080,
				Protocol:   api.ServiceTargetProtocolHttp,
				Enabled:    true,
				Host:       valPtr("10.0.0.1"),
				Path:       valPtr("/v1"),
			},
		},
		Auth: api.ServiceAuthConfig{
			PasswordAuth: &api.PasswordAuthConfig{
				Enabled:  true,
				Password: "pass123",
			},
		},
	}

	// API -> Terraform
	var model ReverseProxyServiceModel
	d := reverseProxyServiceAPIToTerraform(ctx, original, &model)
	if d.HasError() {
		t.Fatalf("APIToTerraform failed with %d errors", d.ErrorsCount())
	}

	// Terraform -> API
	req, d := reverseProxyServiceTerraformToAPI(ctx, &model)
	if d.HasError() {
		t.Fatalf("TerraformToAPI failed with %d errors", d.ErrorsCount())
	}

	if req.Name != original.Name {
		t.Errorf("Name mismatch: expected %s, got %s", original.Name, req.Name)
	}
	if req.Domain != original.Domain {
		t.Errorf("Domain mismatch: expected %s, got %s", original.Domain, req.Domain)
	}
	if req.Enabled != original.Enabled {
		t.Errorf("Enabled mismatch: expected %v, got %v", original.Enabled, req.Enabled)
	}
	if req.PassHostHeader == nil || *req.PassHostHeader != *original.PassHostHeader {
		t.Errorf("PassHostHeader mismatch")
	}
	if req.RewriteRedirects == nil || *req.RewriteRedirects != *original.RewriteRedirects {
		t.Errorf("RewriteRedirects mismatch")
	}
	if len(req.Targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(req.Targets))
	}

	target := req.Targets[0]
	origTarget := original.Targets[0]
	if target.TargetId != origTarget.TargetId {
		t.Errorf("Target.TargetId mismatch: expected %s, got %s", origTarget.TargetId, target.TargetId)
	}
	if target.TargetType != origTarget.TargetType {
		t.Errorf("Target.TargetType mismatch")
	}
	if target.Port != origTarget.Port {
		t.Errorf("Target.Port mismatch: expected %d, got %d", origTarget.Port, target.Port)
	}
	if target.Protocol != origTarget.Protocol {
		t.Errorf("Target.Protocol mismatch")
	}
	if target.Host == nil || *target.Host != *origTarget.Host {
		t.Errorf("Target.Host mismatch")
	}
	if target.Path == nil || *target.Path != *origTarget.Path {
		t.Errorf("Target.Path mismatch")
	}

	if req.Auth.PasswordAuth == nil {
		t.Fatal("Expected PasswordAuth to be set")
	}
	if req.Auth.PasswordAuth.Enabled != original.Auth.PasswordAuth.Enabled {
		t.Errorf("PasswordAuth.Enabled mismatch")
	}
	if req.Auth.PasswordAuth.Password != original.Auth.PasswordAuth.Password {
		t.Errorf("PasswordAuth.Password mismatch")
	}
}

func Test_reverseProxyServiceTargetModelTFType(t *testing.T) {
	tfType := ReverseProxyServiceTargetModel{}.TFType()
	expectedKeys := []string{"target_id", "target_type", "host", "port", "protocol", "path", "enabled"}
	for _, key := range expectedKeys {
		if _, ok := tfType.AttrTypes[key]; !ok {
			t.Errorf("Expected key %s in TFType, not found", key)
		}
	}
	if len(tfType.AttrTypes) != len(expectedKeys) {
		t.Errorf("Expected %d keys in TFType, got %d", len(expectedKeys), len(tfType.AttrTypes))
	}
}

func Test_reverseProxyServiceAuthModelTFType(t *testing.T) {
	tfType := ReverseProxyServiceAuthModel{}.TFType()
	expectedKeys := []string{"password_auth", "pin_auth", "bearer_auth", "link_auth"}
	for _, key := range expectedKeys {
		if _, ok := tfType.AttrTypes[key]; !ok {
			t.Errorf("Expected key %s in TFType, not found", key)
		}
	}
}

func Test_reverseProxyServiceAPIToTerraform_nullAuth(t *testing.T) {
	ctx := context.Background()
	svc := &api.Service{
		Id:      "svc-null",
		Name:    "null-auth-service",
		Domain:  "null.example.com",
		Enabled: true,
		Targets: []api.ServiceTarget{
			{
				TargetId:   "peer1",
				TargetType: api.ServiceTargetTargetTypePeer,
				Port:       80,
				Protocol:   api.ServiceTargetProtocolHttp,
				Enabled:    true,
			},
		},
		Auth: api.ServiceAuthConfig{},
	}

	var out ReverseProxyServiceModel
	d := reverseProxyServiceAPIToTerraform(ctx, svc, &out)
	if d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	if out.Auth.IsNull() {
		t.Error("Auth should not be null")
	}

	authAttrs := out.Auth.Attributes()
	pwObj, ok := authAttrs["password_auth"].(types.Object)
	if !ok || !pwObj.IsNull() {
		t.Error("password_auth should be null when not set in API")
	}
	pinObj, ok := authAttrs["pin_auth"].(types.Object)
	if !ok || !pinObj.IsNull() {
		t.Error("pin_auth should be null when not set in API")
	}
	bearerObj, ok := authAttrs["bearer_auth"].(types.Object)
	if !ok || !bearerObj.IsNull() {
		t.Error("bearer_auth should be null when not set in API")
	}
	linkObj, ok := authAttrs["link_auth"].(types.Object)
	if !ok || !linkObj.IsNull() {
		t.Error("link_auth should be null when not set in API")
	}
}

func Test_reverseProxyServiceAPIToTerraform_bearerWithNilGroups(t *testing.T) {
	ctx := context.Background()
	svc := &api.Service{
		Id:      "svc-bearer-nil",
		Name:    "bearer-nil-groups",
		Domain:  "bearer.example.com",
		Enabled: true,
		Targets: []api.ServiceTarget{
			{
				TargetId:   "peer1",
				TargetType: api.ServiceTargetTargetTypePeer,
				Port:       80,
				Protocol:   api.ServiceTargetProtocolHttp,
				Enabled:    true,
			},
		},
		Auth: api.ServiceAuthConfig{
			BearerAuth: &api.BearerAuthConfig{
				Enabled:            true,
				DistributionGroups: nil,
			},
		},
	}

	var out ReverseProxyServiceModel
	d := reverseProxyServiceAPIToTerraform(ctx, svc, &out)
	if d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	authAttrs := out.Auth.Attributes()
	bearerObj, ok := authAttrs["bearer_auth"].(types.Object)
	if !ok {
		t.Fatal("bearer_auth is not types.Object")
	}
	if bearerObj.IsNull() {
		t.Fatal("bearer_auth should not be null")
	}
	bearerAttrs := bearerObj.Attributes()
	groupsList, ok := bearerAttrs["distribution_groups"].(types.List)
	if !ok {
		t.Fatal("distribution_groups is not types.List")
	}
	if !groupsList.IsNull() {
		t.Error("distribution_groups should be null when API returns nil")
	}
}

func Test_reverseProxyServiceAPIToTerraform_multipleTargets(t *testing.T) {
	ctx := context.Background()
	svc := &api.Service{
		Id:      "svc-multi",
		Name:    "multi-target",
		Domain:  "multi.example.com",
		Enabled: true,
		Targets: []api.ServiceTarget{
			{
				TargetId:   "peer1",
				TargetType: api.ServiceTargetTargetTypePeer,
				Port:       8080,
				Protocol:   api.ServiceTargetProtocolHttp,
				Enabled:    true,
			},
			{
				TargetId:   "res1",
				TargetType: api.ServiceTargetTargetType("subnet"),
				Port:       443,
				Protocol:   api.ServiceTargetProtocolHttps,
				Enabled:    false,
				Host:       valPtr("backend.local"),
				Path:       valPtr("/api/v2"),
			},
		},
		Auth: api.ServiceAuthConfig{
			PinAuth: &api.PINAuthConfig{
				Enabled: true,
				Pin:     "9999",
			},
		},
	}

	var out ReverseProxyServiceModel
	d := reverseProxyServiceAPIToTerraform(ctx, svc, &out)
	if d.HasError() {
		t.Fatalf("Expected no error diagnostics, found %d errors", d.ErrorsCount())
	}

	var targets []ReverseProxyServiceTargetModel
	d = out.Targets.ElementsAs(ctx, &targets, false)
	if d.HasError() {
		t.Fatalf("Failed to extract targets: %d errors", d.ErrorsCount())
	}
	if len(targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(targets))
	}

	if targets[0].TargetId.ValueString() != "peer1" {
		t.Errorf("First target ID mismatch: expected peer1, got %s", targets[0].TargetId.ValueString())
	}
	if targets[1].TargetId.ValueString() != "res1" {
		t.Errorf("Second target ID mismatch: expected res1, got %s", targets[1].TargetId.ValueString())
	}
	if targets[1].Host.ValueString() != "backend.local" {
		t.Errorf("Second target host mismatch")
	}
	if targets[1].Path.ValueString() != "/api/v2" {
		t.Errorf("Second target path mismatch")
	}
	if targets[1].Enabled.ValueBool() != false {
		t.Errorf("Second target should be disabled")
	}
}

func Test_reverseProxyServiceTerraformToAPI_targets(t *testing.T) {
	ctx := context.Background()

	targetModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(8080),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringNull(),
			Path:       types.StringNull(),
		},
	}

	targetsList, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), targetModels)
	if d.HasError() {
		t.Fatalf("Failed to build targets list")
	}

	pwAuth, d := types.ObjectValueFrom(ctx, ReverseProxyPasswordAuthModel{}.TFType().AttrTypes, ReverseProxyPasswordAuthModel{
		Enabled:  types.BoolValue(false),
		Password: types.StringValue(""),
	})
	if d.HasError() {
		t.Fatalf("Failed to build password auth object")
	}

	authModel := ReverseProxyServiceAuthModel{
		PasswordAuth: pwAuth,
		PinAuth:      types.ObjectNull(ReverseProxyPinAuthModel{}.TFType().AttrTypes),
		BearerAuth:   types.ObjectNull(ReverseProxyBearerAuthModel{}.TFType().AttrTypes),
		LinkAuth:     types.ObjectNull(ReverseProxyLinkAuthModel{}.TFType().AttrTypes),
	}
	authObj, d := types.ObjectValueFrom(ctx, ReverseProxyServiceAuthModel{}.TFType().AttrTypes, authModel)
	if d.HasError() {
		t.Fatalf("Failed to build auth object")
	}

	model := &ReverseProxyServiceModel{
		Id:               types.StringValue("test-id"),
		Name:             types.StringValue("test"),
		Domain:           types.StringValue("test.com"),
		Enabled:          types.BoolValue(true),
		PassHostHeader:   types.BoolValue(false),
		RewriteRedirects: types.BoolValue(false),
		Targets:          targetsList,
		Auth:             authObj,
	}

	req, d := reverseProxyServiceTerraformToAPI(ctx, model)
	if d.HasError() {
		t.Fatalf("TerraformToAPI failed with %d errors", d.ErrorsCount())
	}

	if req.Name != "test" {
		t.Errorf("Name mismatch: expected test, got %s", req.Name)
	}
	if len(req.Targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(req.Targets))
	}
	if req.Targets[0].TargetId != "peer1" {
		t.Errorf("Target ID mismatch")
	}
	if req.Targets[0].Host != nil {
		t.Errorf("Target Host should be nil for null input")
	}
	if req.Targets[0].Path != nil {
		t.Errorf("Target Path should be nil for null input")
	}
	if req.Auth.PasswordAuth == nil {
		t.Fatal("Expected PasswordAuth to be set")
	}
	if req.Auth.PinAuth != nil {
		t.Error("Expected PinAuth to be nil")
	}
	if req.Auth.BearerAuth != nil {
		t.Error("Expected BearerAuth to be nil")
	}
	if req.Auth.LinkAuth != nil {
		t.Error("Expected LinkAuth to be nil")
	}
}

func Test_reverseProxyServiceSchemaValidation(t *testing.T) {
	r := &ReverseProxyService{}

	var metaResp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "netbird"}, &metaResp)
	if metaResp.TypeName != "netbird_reverse_proxy_service" {
		t.Errorf("Expected type name netbird_reverse_proxy_service, got %s", metaResp.TypeName)
	}

	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors")
	}

	attrs := schemaResp.Schema.Attributes
	requiredAttrs := []string{"name", "domain", "targets", "auth"}
	for _, a := range requiredAttrs {
		if _, ok := attrs[a]; !ok {
			t.Errorf("Expected required attribute %s in schema", a)
		}
	}

	computedAttrs := []string{"id", "proxy_cluster"}
	for _, a := range computedAttrs {
		if _, ok := attrs[a]; !ok {
			t.Errorf("Expected computed attribute %s in schema", a)
		}
	}

	var _ resource.Resource = &ReverseProxyService{}
	var _ resource.ResourceWithImportState = &ReverseProxyService{}
}

func Test_reverseProxyServiceTFTypeConsistency(t *testing.T) {
	targetType := ReverseProxyServiceTargetModel{}.TFType()
	authType := ReverseProxyServiceAuthModel{}.TFType()
	pwType := ReverseProxyPasswordAuthModel{}.TFType()
	pinType := ReverseProxyPinAuthModel{}.TFType()
	bearerType := ReverseProxyBearerAuthModel{}.TFType()
	linkType := ReverseProxyLinkAuthModel{}.TFType()

	if !reflect.DeepEqual(authType.AttrTypes["password_auth"], pwType) {
		t.Error("Auth password_auth type doesn't match PasswordAuth TFType")
	}
	if !reflect.DeepEqual(authType.AttrTypes["pin_auth"], pinType) {
		t.Error("Auth pin_auth type doesn't match PinAuth TFType")
	}
	if !reflect.DeepEqual(authType.AttrTypes["bearer_auth"], bearerType) {
		t.Error("Auth bearer_auth type doesn't match BearerAuth TFType")
	}
	if !reflect.DeepEqual(authType.AttrTypes["link_auth"], linkType) {
		t.Error("Auth link_auth type doesn't match LinkAuth TFType")
	}

	if len(targetType.AttrTypes) != 7 {
		t.Errorf("Expected 7 target attributes, got %d", len(targetType.AttrTypes))
	}
}

func Test_preserveAuthSecrets(t *testing.T) {
	ctx := context.Background()

	// Build prior state with real password and pin
	priorAuthModel := ReverseProxyServiceAuthModel{
		PasswordAuth: mustObjectValue(ctx, ReverseProxyPasswordAuthModel{}.TFType().AttrTypes, ReverseProxyPasswordAuthModel{
			Enabled:  types.BoolValue(true),
			Password: types.StringValue("real-password"),
		}),
		PinAuth: mustObjectValue(ctx, ReverseProxyPinAuthModel{}.TFType().AttrTypes, ReverseProxyPinAuthModel{
			Enabled: types.BoolValue(true),
			Pin:     types.StringValue("1234"),
		}),
		BearerAuth: types.ObjectNull(ReverseProxyBearerAuthModel{}.TFType().AttrTypes),
		LinkAuth:   types.ObjectNull(ReverseProxyLinkAuthModel{}.TFType().AttrTypes),
	}
	priorAuth := mustObjectValue(ctx, ReverseProxyServiceAuthModel{}.TFType().AttrTypes, priorAuthModel)

	// Build current state with redacted (empty) password and pin from API
	currentAuthModel := ReverseProxyServiceAuthModel{
		PasswordAuth: mustObjectValue(ctx, ReverseProxyPasswordAuthModel{}.TFType().AttrTypes, ReverseProxyPasswordAuthModel{
			Enabled:  types.BoolValue(true),
			Password: types.StringValue(""),
		}),
		PinAuth: mustObjectValue(ctx, ReverseProxyPinAuthModel{}.TFType().AttrTypes, ReverseProxyPinAuthModel{
			Enabled: types.BoolValue(true),
			Pin:     types.StringValue(""),
		}),
		BearerAuth: types.ObjectNull(ReverseProxyBearerAuthModel{}.TFType().AttrTypes),
		LinkAuth:   types.ObjectNull(ReverseProxyLinkAuthModel{}.TFType().AttrTypes),
	}
	currentAuth := mustObjectValue(ctx, ReverseProxyServiceAuthModel{}.TFType().AttrTypes, currentAuthModel)

	result, d := preserveAuthSecrets(priorAuth, currentAuth)
	if d.HasError() {
		t.Fatalf("preserveAuthSecrets failed with %d errors", d.ErrorsCount())
	}

	authAttrs := result.Attributes()
	pwObj, ok := authAttrs["password_auth"].(types.Object)
	if !ok {
		t.Fatal("password_auth is not types.Object")
	}
	pwAttrs := pwObj.Attributes()
	password, ok := pwAttrs["password"].(types.String)
	if !ok {
		t.Fatal("password is not types.String")
	}
	if password.ValueString() != "real-password" {
		t.Errorf("Expected password to be preserved as 'real-password', got %q", password.ValueString())
	}

	pinObj, ok := authAttrs["pin_auth"].(types.Object)
	if !ok {
		t.Fatal("pin_auth is not types.Object")
	}
	pinAttrs := pinObj.Attributes()
	pin, ok := pinAttrs["pin"].(types.String)
	if !ok {
		t.Fatal("pin is not types.String")
	}
	if pin.ValueString() != "1234" {
		t.Errorf("Expected pin to be preserved as '1234', got %q", pin.ValueString())
	}
}

func Test_preserveAuthSecrets_nullPrior(t *testing.T) {
	ctx := context.Background()

	currentAuthModel := ReverseProxyServiceAuthModel{
		PasswordAuth: mustObjectValue(ctx, ReverseProxyPasswordAuthModel{}.TFType().AttrTypes, ReverseProxyPasswordAuthModel{
			Enabled:  types.BoolValue(true),
			Password: types.StringValue("new-password"),
		}),
		PinAuth:    types.ObjectNull(ReverseProxyPinAuthModel{}.TFType().AttrTypes),
		BearerAuth: types.ObjectNull(ReverseProxyBearerAuthModel{}.TFType().AttrTypes),
		LinkAuth:   types.ObjectNull(ReverseProxyLinkAuthModel{}.TFType().AttrTypes),
	}
	currentAuth := mustObjectValue(ctx, ReverseProxyServiceAuthModel{}.TFType().AttrTypes, currentAuthModel)

	result, d := preserveAuthSecrets(types.ObjectNull(ReverseProxyServiceAuthModel{}.TFType().AttrTypes), currentAuth)
	if d.HasError() {
		t.Fatalf("preserveAuthSecrets failed with %d errors", d.ErrorsCount())
	}

	if result.IsNull() {
		t.Error("Result should not be null when only prior is null")
	}

	authAttrs := result.Attributes()
	pwObj, ok := authAttrs["password_auth"].(types.Object)
	if !ok {
		t.Fatal("password_auth is not types.Object")
	}
	pwAttrs := pwObj.Attributes()
	password, ok := pwAttrs["password"].(types.String)
	if !ok {
		t.Fatal("password is not types.String")
	}
	if password.ValueString() != "new-password" {
		t.Errorf("Expected password to remain 'new-password', got %q", password.ValueString())
	}
}

func Test_preserveTargetPlanValues(t *testing.T) {
	ctx := context.Background()

	// Plan has explicit host and path
	planModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(8080),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("10.0.0.1"),
			Path:       types.StringValue("/custom"),
		},
	}
	planTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), planModels)
	if d.HasError() {
		t.Fatal("Failed to build plan targets")
	}

	// API returns different host (resolved from peer IP) and path
	apiModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(8080),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("100.0.219.86"),
			Path:       types.StringValue("/"),
		},
	}
	apiTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), apiModels)
	if d.HasError() {
		t.Fatal("Failed to build API targets")
	}

	result, d := preserveTargetPlanValues(ctx, planTargets, apiTargets)
	if d.HasError() {
		t.Fatalf("preserveTargetPlanValues failed with %d errors", d.ErrorsCount())
	}

	var resultModels []ReverseProxyServiceTargetModel
	d = result.ElementsAs(ctx, &resultModels, false)
	if d.HasError() {
		t.Fatal("Failed to extract result targets")
	}

	if len(resultModels) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(resultModels))
	}
	if resultModels[0].Host.ValueString() != "10.0.0.1" {
		t.Errorf("Expected host to be preserved as '10.0.0.1', got %q", resultModels[0].Host.ValueString())
	}
	if resultModels[0].Path.ValueString() != "/custom" {
		t.Errorf("Expected path to be preserved as '/custom', got %q", resultModels[0].Path.ValueString())
	}
	// Non-preserved fields should come from API
	if resultModels[0].Port.ValueInt64() != 8080 {
		t.Errorf("Expected port 8080, got %d", resultModels[0].Port.ValueInt64())
	}
}

func Test_preserveTargetPlanValues_nullPlanHost(t *testing.T) {
	ctx := context.Background()

	// Plan has null host — should accept API's value
	planModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(80),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringNull(),
			Path:       types.StringNull(),
		},
	}
	planTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), planModels)
	if d.HasError() {
		t.Fatal("Failed to build plan targets")
	}

	apiModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(80),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("100.0.219.86"),
			Path:       types.StringValue("/"),
		},
	}
	apiTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), apiModels)
	if d.HasError() {
		t.Fatal("Failed to build API targets")
	}

	result, d := preserveTargetPlanValues(ctx, planTargets, apiTargets)
	if d.HasError() {
		t.Fatalf("preserveTargetPlanValues failed with %d errors", d.ErrorsCount())
	}

	var resultModels []ReverseProxyServiceTargetModel
	d = result.ElementsAs(ctx, &resultModels, false)
	if d.HasError() {
		t.Fatal("Failed to extract result targets")
	}

	if resultModels[0].Host.ValueString() != "100.0.219.86" {
		t.Errorf("Expected API host '100.0.219.86' when plan is null, got %q", resultModels[0].Host.ValueString())
	}
	if resultModels[0].Path.ValueString() != "/" {
		t.Errorf("Expected API path '/' when plan is null, got %q", resultModels[0].Path.ValueString())
	}
}

func Test_preserveTargetPlanValues_extraAPITarget(t *testing.T) {
	ctx := context.Background()

	planModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(80),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("10.0.0.1"),
			Path:       types.StringNull(),
		},
	}
	planTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), planModels)
	if d.HasError() {
		t.Fatal("Failed to build plan targets")
	}

	// API returned 2 targets — peer1 matches plan, peer2 is new
	apiModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(80),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("100.0.219.86"),
			Path:       types.StringValue("/"),
		},
		{
			TargetId:   types.StringValue("peer2"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(443),
			Protocol:   types.StringValue("https"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("100.0.34.47"),
			Path:       types.StringValue("/"),
		},
	}
	apiTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), apiModels)
	if d.HasError() {
		t.Fatal("Failed to build API targets")
	}

	result, d := preserveTargetPlanValues(ctx, planTargets, apiTargets)
	if d.HasError() {
		t.Fatalf("preserveTargetPlanValues failed with %d errors", d.ErrorsCount())
	}

	var resultModels []ReverseProxyServiceTargetModel
	d = result.ElementsAs(ctx, &resultModels, false)
	if d.HasError() {
		t.Fatal("Failed to extract result targets")
	}

	if len(resultModels) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(resultModels))
	}
	// peer1 matched by ID — plan host preserved
	if resultModels[0].Host.ValueString() != "10.0.0.1" {
		t.Errorf("Expected peer1 host preserved as '10.0.0.1', got %q", resultModels[0].Host.ValueString())
	}
	// peer2 not in plan — API host kept
	if resultModels[1].Host.ValueString() != "100.0.34.47" {
		t.Errorf("Expected peer2 API host '100.0.34.47', got %q", resultModels[1].Host.ValueString())
	}
}

func Test_preserveTargetPlanValues_nullPlan(t *testing.T) {
	ctx := context.Background()

	apiModels := []ReverseProxyServiceTargetModel{
		{
			TargetId:   types.StringValue("peer1"),
			TargetType: types.StringValue("peer"),
			Port:       types.Int64Value(80),
			Protocol:   types.StringValue("http"),
			Enabled:    types.BoolValue(true),
			Host:       types.StringValue("100.0.219.86"),
			Path:       types.StringValue("/"),
		},
	}
	apiTargets, d := types.ListValueFrom(ctx, ReverseProxyServiceTargetModel{}.TFType(), apiModels)
	if d.HasError() {
		t.Fatal("Failed to build API targets")
	}

	result, d := preserveTargetPlanValues(ctx, types.ListNull(ReverseProxyServiceTargetModel{}.TFType()), apiTargets)
	if d.HasError() {
		t.Fatalf("preserveTargetPlanValues failed with %d errors", d.ErrorsCount())
	}

	// Should return API targets unmodified
	var resultModels []ReverseProxyServiceTargetModel
	d = result.ElementsAs(ctx, &resultModels, false)
	if d.HasError() {
		t.Fatal("Failed to extract result targets")
	}

	if resultModels[0].Host.ValueString() != "100.0.219.86" {
		t.Errorf("Expected API host when plan is null, got %q", resultModels[0].Host.ValueString())
	}
}

func Test_reverseProxyServiceRoundtrip_bearerAuth(t *testing.T) {
	ctx := context.Background()

	original := &api.Service{
		Id:      "svc-bearer-rt",
		Name:    "bearer-roundtrip",
		Domain:  "bearer-rt.example.com",
		Enabled: true,
		Targets: []api.ServiceTarget{
			{
				TargetId:   "peer1",
				TargetType: api.ServiceTargetTargetTypePeer,
				Port:       8080,
				Protocol:   api.ServiceTargetProtocolHttp,
				Enabled:    true,
			},
		},
		Auth: api.ServiceAuthConfig{
			BearerAuth: &api.BearerAuthConfig{
				Enabled:            true,
				DistributionGroups: &[]string{"group1", "group2"},
			},
		},
	}

	// API -> Terraform
	var model ReverseProxyServiceModel
	d := reverseProxyServiceAPIToTerraform(ctx, original, &model)
	if d.HasError() {
		t.Fatalf("APIToTerraform failed with %d errors", d.ErrorsCount())
	}

	// Terraform -> API
	req, d := reverseProxyServiceTerraformToAPI(ctx, &model)
	if d.HasError() {
		t.Fatalf("TerraformToAPI failed with %d errors", d.ErrorsCount())
	}

	if req.Auth.BearerAuth == nil {
		t.Fatal("Expected BearerAuth to be set")
	}
	if req.Auth.BearerAuth.Enabled != true {
		t.Error("BearerAuth.Enabled should be true")
	}
	if req.Auth.BearerAuth.DistributionGroups == nil {
		t.Fatal("Expected DistributionGroups to be set")
	}
	groups := *req.Auth.BearerAuth.DistributionGroups
	if len(groups) != 2 {
		t.Fatalf("Expected 2 distribution groups, got %d", len(groups))
	}
	if groups[0] != "group1" || groups[1] != "group2" {
		t.Errorf("Distribution groups mismatch: got %v", groups)
	}
	if req.Auth.PasswordAuth != nil {
		t.Error("PasswordAuth should be nil")
	}
	if req.Auth.PinAuth != nil {
		t.Error("PinAuth should be nil")
	}
}

func mustObjectValue(ctx context.Context, attrTypes map[string]attr.Type, val any) types.Object {
	obj, d := types.ObjectValueFrom(ctx, attrTypes, val)
	if d.HasError() {
		panic("failed to create object value in test helper")
	}
	return obj
}
