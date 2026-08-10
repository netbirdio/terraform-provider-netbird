// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

// unsetEnv removes an environment variable for the duration of the test and
// restores it afterwards, which t.Setenv cannot express.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			//nolint:usetesting // t.Setenv is not allowed inside Cleanup, and only a real unset exercises the provider's LookupEnv fallback
			_ = os.Setenv(key, prev)
			return
		}
		_ = os.Unsetenv(key)
	})
}

// configureProvider runs Configure with the given provider block values. A nil
// entry means the attribute was omitted from the configuration, which is what
// makes the environment-variable fallbacks apply.
func configureProvider(t *testing.T, managementURL, token, tenantAccount *string) provider.ConfigureResponse {
	t.Helper()

	p, ok := New("test")().(*NetBirdProvider)
	if !ok {
		t.Fatal("failed to cast to *NetBirdProvider")
	}

	schemaResp := provider.SchemaResponse{}
	p.Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)

	value := func(s *string) tftypes.Value {
		if s == nil {
			return tftypes.NewValue(tftypes.String, nil)
		}
		return tftypes.NewValue(tftypes.String, *s)
	}

	req := provider.ConfigureRequest{
		TerraformVersion: "1.5.0",
		Config: tfsdk.Config{
			Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{
				"management_url": value(managementURL),
				"token":          value(token),
				"tenant_account": value(tenantAccount),
			}),
			Schema: schemaResp.Schema,
		},
	}

	resp := provider.ConfigureResponse{}
	p.Configure(context.Background(), req, &resp)
	return resp
}

// recordedRequest is what the stub management server saw.
type recordedRequest struct {
	Header http.Header
	Query  url.Values
}

// recordingServer answers any request with an empty JSON array and records what
// it saw, so a test can tell which URL, which token and which impersonation the
// configured client actually used. Impersonation rides as the "account" query
// parameter rather than a header.
func recordingServer(t *testing.T, seen *recordedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = recordedRequest{Header: r.Header.Clone(), Query: r.URL.Query()}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(server.Close)
	return server
}

// A token in the provider block is used even when NB_PAT names a different one.
func TestProviderConfigure_TokenFromConfigWinsOverEnv(t *testing.T) {
	var seen recordedRequest
	server := recordingServer(t, &seen)

	t.Setenv("NB_PAT", "env-token")
	t.Setenv("NB_MANAGEMENT_URL", server.URL)

	resp := configureProvider(t, nil, valPtr("config-token"), nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
	}

	client, ok := resp.ResourceData.(*netbird.Client)
	if !ok {
		t.Fatal("provider did not produce a client")
	}
	if _, err := client.Accounts.List(context.Background()); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if got := seen.Header.Get("Authorization"); got != "Token config-token" {
		t.Errorf("Authorization header mismatch: %q", got)
	}
}

// With no token in the block and no NB_PAT, the provider must refuse rather than
// build a client that will 401 on every call.
func TestProviderConfigure_MissingTokenIsAnError(t *testing.T) {
	t.Setenv("NB_MANAGEMENT_URL", "http://127.0.0.1:1")
	unsetEnv(t, "NB_PAT")

	resp := configureProvider(t, nil, nil, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an error diagnostic when no token is configured")
	}
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Error("provider should not hand out a client when configuration failed")
	}

	var found bool
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Missing required argument" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a \"Missing required argument\" diagnostic, got %v", resp.Diagnostics.Errors())
	}
}

// The management URL in the block wins over NB_MANAGEMENT_URL.
func TestProviderConfigure_ManagementURLFromConfigWinsOverEnv(t *testing.T) {
	var seen recordedRequest
	configured := recordingServer(t, &seen)
	fromEnv := recordingServer(t, nil)

	t.Setenv("NB_MANAGEMENT_URL", fromEnv.URL)
	t.Setenv("NB_PAT", "token")

	resp := configureProvider(t, valPtr(configured.URL), nil, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
	}
	client, ok := resp.ResourceData.(*netbird.Client)
	if !ok {
		t.Fatal("provider did not produce a client")
	}
	if _, err := client.Accounts.List(context.Background()); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if seen.Header.Get("Authorization") == "" {
		t.Error("the request did not reach the server named in the provider block")
	}
}

// tenant_account impersonates another account; NB_ACCOUNT is the fallback. Both
// must reach the wire as the impersonation header.
func TestProviderConfigure_TenantAccountImpersonation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		attr     *string
		env      string
		expected string
	}{
		{name: "from config", attr: valPtr("acct-from-config"), env: "acct-from-env", expected: "acct-from-config"},
		{name: "from env", attr: nil, env: "acct-from-env", expected: "acct-from-env"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var seen recordedRequest
			server := recordingServer(t, &seen)

			t.Setenv("NB_MANAGEMENT_URL", server.URL)
			t.Setenv("NB_PAT", "token")
			t.Setenv("NB_ACCOUNT", tc.env)

			resp := configureProvider(t, nil, nil, tc.attr)
			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
			}
			client, ok := resp.ResourceData.(*netbird.Client)
			if !ok {
				t.Fatal("provider did not produce a client")
			}
			if _, err := client.Accounts.List(context.Background()); err != nil {
				t.Fatalf("request failed: %v", err)
			}

			if got := seen.Query.Get("account"); got != tc.expected {
				t.Errorf("impersonated account mismatch: expected %q, got %q (query %v)", tc.expected, got, seen.Query)
			}
		})
	}
}

// Without tenant_account or NB_ACCOUNT the client must not impersonate anyone.
func TestProviderConfigure_NoImpersonationByDefault(t *testing.T) {
	var seen recordedRequest
	server := recordingServer(t, &seen)

	t.Setenv("NB_MANAGEMENT_URL", server.URL)
	t.Setenv("NB_PAT", "token")
	unsetEnv(t, "NB_ACCOUNT")

	resp := configureProvider(t, nil, nil, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics.Errors())
	}
	client, ok := resp.ResourceData.(*netbird.Client)
	if !ok {
		t.Fatal("provider did not produce a client")
	}
	if _, err := client.Accounts.List(context.Background()); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if got := seen.Query.Get("account"); got != "" {
		t.Errorf("client impersonated %q without tenant_account or NB_ACCOUNT", got)
	}
}

// The provider must advertise every resource, data source, function and
// ephemeral resource without duplicate type names — a duplicate would make
// Terraform reject the provider at load time.
func TestProviderSurfaceIsWellFormed(t *testing.T) {
	ctx := context.Background()
	p, ok := New("test")().(*NetBirdProvider)
	if !ok {
		t.Fatal("failed to cast to *NetBirdProvider")
	}

	metaResp := provider.MetadataResponse{}
	p.Metadata(ctx, provider.MetadataRequest{}, &metaResp)
	if metaResp.TypeName != "netbird" {
		t.Errorf("provider type name mismatch: %q", metaResp.TypeName)
	}
	if metaResp.Version != "test" {
		t.Errorf("provider version mismatch: %q", metaResp.Version)
	}

	seen := map[string]string{}
	resources := p.Resources(ctx)
	if len(resources) == 0 {
		t.Fatal("provider advertises no resources")
	}
	for _, newResource := range resources {
		r := newResource()
		resp := resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: metaResp.TypeName}, &resp)
		if prev, dup := seen[resp.TypeName]; dup {
			t.Errorf("duplicate resource type name %s (also %s)", resp.TypeName, prev)
		}
		seen[resp.TypeName] = "resource"

		schemaResp := resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Errorf("%s schema has errors: %v", resp.TypeName, schemaResp.Diagnostics.Errors())
		}
		if len(schemaResp.Schema.Attributes) == 0 && len(schemaResp.Schema.Blocks) == 0 {
			t.Errorf("%s has an empty schema", resp.TypeName)
		}
	}

	dataSources := p.DataSources(ctx)
	if len(dataSources) == 0 {
		t.Fatal("provider advertises no data sources")
	}
	for _, newDataSource := range dataSources {
		d := newDataSource()
		resp := datasource.MetadataResponse{}
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: metaResp.TypeName}, &resp)
		if prev, dup := seen[resp.TypeName]; dup && prev == "data source" {
			t.Errorf("duplicate data source type name %s", resp.TypeName)
		}
		seen[resp.TypeName] = "data source"

		schemaResp := datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Errorf("%s schema has errors: %v", resp.TypeName, schemaResp.Diagnostics.Errors())
		}
		if len(schemaResp.Schema.Attributes) == 0 && len(schemaResp.Schema.Blocks) == 0 {
			t.Errorf("%s has an empty schema", resp.TypeName)
		}
	}

	// Declared but intentionally empty today; calling them keeps the provider
	// interface honest.
	if got := p.Functions(ctx); len(got) != 0 {
		t.Errorf("expected no provider functions, got %d", len(got))
	}
	if got := p.EphemeralResources(ctx); len(got) != 0 {
		t.Errorf("expected no ephemeral resources, got %d", len(got))
	}
}
