// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

func valPtr[T any](s T) *T {
	return &s
}

// TestProviderUserAgent verifies that the provider sends the correct User-Agent header.
func TestProviderUserAgent(t *testing.T) {
	var capturedUserAgent string
	expectedVersion := "1.2.3"
	expectedTerraformVersion := "1.5.0"
	expectedUserAgent := fmt.Sprintf("terraform-provider-netbird/%s Terraform/%s", expectedVersion, expectedTerraformVersion)

	// Create a test HTTP server that captures the User-Agent header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	t.Setenv("NB_MANAGEMENT_URL", server.URL)
	t.Setenv("NB_PAT", "test-token")

	p, ok := New(expectedVersion)().(*NetBirdProvider)
	if !ok {
		t.Fatal("failed to cast to *NetBirdProvider")
	}

	schemaReq := provider.SchemaRequest{}
	schemaResp := provider.SchemaResponse{}
	p.Schema(context.Background(), schemaReq, &schemaResp)

	configValue := schemaResp.Schema.Type().TerraformType(context.Background())

	req := provider.ConfigureRequest{
		TerraformVersion: expectedTerraformVersion,
	}
	req.Config = tfsdk.Config{
		Raw: tftypes.NewValue(configValue, map[string]tftypes.Value{
			"management_url": tftypes.NewValue(tftypes.String, nil),
			"token":          tftypes.NewValue(tftypes.String, nil),
			"tenant_account": tftypes.NewValue(tftypes.String, nil),
		}),
		Schema: schemaResp.Schema,
	}

	resp := provider.ConfigureResponse{}
	p.Configure(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Failed to configure provider: %v", resp.Diagnostics.Errors())
	}

	client, ok := resp.ResourceData.(*netbird.Client)
	if !ok {
		t.Fatal("Failed to get client from provider response")
	}

	_, err := client.Accounts.List(context.Background())
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if capturedUserAgent != expectedUserAgent {
		t.Errorf("User-Agent mismatch:\nExpected: %s\nGot:      %s", expectedUserAgent, capturedUserAgent)
	}
}
