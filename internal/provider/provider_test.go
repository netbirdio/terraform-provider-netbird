// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func valPtr[T any](s T) *T {
	return &s
}

// valOr dereferences an optional API field, falling back to a zero-ish default
// when the server omitted it.
func valOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// GetProjectDir will return the directory where the project is.
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.ReplaceAll(wd, "/internal/provider", "")
	return wd, nil
}

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"netbird": providerserver.NewProtocol6WithError(New("test")()),
}

// testClient talks to the deployment the suite bootstrapped. It is called from
// Check functions that have no *testing.T to hand, so it reads the stack that
// testEnsureManagementRunning already brought up rather than starting one.
func testClient() *netbird.Client {
	if e2eEnv == nil {
		panic("testClient called before the e2e stack was bootstrapped; the test's PreCheck must call testEnsureManagementRunning")
	}
	return netbird.New(e2eEnv.ManagementURL, e2eEnv.Token)
}

// sameIDSet reports whether the group references the server returned are exactly
// the wanted IDs, regardless of order — the API does not promise one.
func sameIDSet(got []api.GroupMinimum, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	remaining := make(map[string]int, len(want))
	for _, w := range want {
		remaining[w]++
	}
	for _, g := range got {
		if remaining[g.Id] == 0 {
			return false
		}
		remaining[g.Id]--
	}
	return true
}

func matchPairs(pairs map[string][]any) error {
	for name, p := range pairs {
		if reflect.ValueOf(p[1]).Kind() == reflect.Pointer {
			if reflect.ValueOf(p[0]).Kind() == reflect.Pointer {
				if reflect.ValueOf(p[1]).IsNil() != reflect.ValueOf(p[0]).IsNil() {
					return fmt.Errorf("Mistmatch, expected field %s to be %v, found nil", name, p[0])
				}
				if reflect.ValueOf(p[0]).IsNil() {
					continue
				}
			}
			if !reflect.ValueOf(p[0]).IsValid() && reflect.ValueOf(p[1]).IsNil() {
				continue
			} else if !reflect.ValueOf(p[0]).IsValid() {
				return fmt.Errorf("Mismatch, expected field %s to be nil, found %v", name, reflect.ValueOf(p[1]).Elem().Interface())
			}
			if v := reflect.ValueOf(p[1]).Elem().Interface(); v != p[0] {
				return fmt.Errorf("Mismatch, expected field %s to be %v, found %v", name, p[0], v)
			}
		} else if p[0] != p[1] {
			return fmt.Errorf("Mismatch, expected field %s to be %v, found %v", name, p[0], p[1])
		}
	}
	return nil
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
