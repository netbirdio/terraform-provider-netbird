package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

const twoProvidersJSON = `[
  {"id":"p1","provider_id":"openai_api","name":"OpenAI","upstream_url":"https://api.openai.com","enabled":true,"skip_tls_verification":false,"metadata_disabled":false,"models":[]},
  {"id":"p2","provider_id":"anthropic_api","name":"Anthropic","upstream_url":"https://api.anthropic.com","enabled":true,"skip_tls_verification":false,"metadata_disabled":false,"models":[]}
]`

const oneProviderJSON = `{"id":"p1","provider_id":"openai_api","name":"OpenAI","upstream_url":"https://api.openai.com","enabled":true,"skip_tls_verification":false,"metadata_disabled":false,"models":[]}`

// readProviderDataSource drives the data source's Read end to end against a stub
// API. This is the only unit-level way to exercise the selector logic together
// with the schema/model round-trip: a model field missing from the schema fails
// here as a conversion error even though it compiles fine.
func readProviderDataSource(t *testing.T, serverURL, id, name string) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()

	d := &AgentNetworkProviderDataSource{client: newAgentNetworkClient(netbird.New(serverURL, "test-token"))}

	schemaResp := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema error: %v", schemaResp.Diagnostics.Errors())
	}

	objType, ok := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatal("schema type is not an object")
	}

	vals := map[string]tftypes.Value{}
	for attrName, attrType := range objType.AttributeTypes {
		var v any
		switch attrName {
		case "id":
			v = nullableString(id)
		case "name":
			v = nullableString(name)
		}
		vals[attrName] = tftypes.NewValue(attrType, v)
	}

	resp := datasource.ReadResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(objType, nil), Schema: schemaResp.Schema},
	}
	d.Read(ctx, datasource.ReadRequest{
		Config: tfsdk.Config{Raw: tftypes.NewValue(objType, vals), Schema: schemaResp.Schema},
	}, &resp)

	return resp.Diagnostics
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func Test_AgentNetworkProviderDataSource_Read(t *testing.T) {
	cases := []struct {
		name       string
		id         string
		filterName string
		wantErr    string // empty means success expected
		wantNoList bool   // by-id lookups must not hit the list endpoint
	}{
		{name: "by name selects the matching provider", filterName: "Anthropic"},
		{name: "by id uses the direct GET", id: "p1", wantNoList: true},
		{name: "by id and matching name", id: "p1", filterName: "OpenAI", wantNoList: true},
		// Regression: an inverted match filter previously let every
		// non-matching row count as a match.
		{name: "unknown name reports no match", filterName: "Nope", wantErr: "No Match"},
		{name: "id and mismatched name reports no match", id: "p1", filterName: "Anthropic", wantErr: "No Match"},
		{name: "no selector is rejected", wantErr: "No selector"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var listed bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if strings.HasPrefix(r.URL.Path, "/api/agent-network/providers/") {
					_, _ = w.Write([]byte(oneProviderJSON))
					return
				}
				listed = true
				_, _ = w.Write([]byte(twoProvidersJSON))
			}))
			defer server.Close()

			diags := readProviderDataSource(t, server.URL, c.id, c.filterName)

			if c.wantErr != "" {
				if !diags.HasError() {
					t.Fatalf("Expected error %q, got none", c.wantErr)
				}
				var found bool
				for _, e := range diags.Errors() {
					if strings.Contains(e.Summary(), c.wantErr) {
						found = true
					}
				}
				if !found {
					t.Fatalf("Expected an error mentioning %q, got: %v", c.wantErr, diags.Errors())
				}
				return
			}

			if diags.HasError() {
				t.Fatalf("Expected no error, got: %v", diags.Errors())
			}
			if c.wantNoList && listed {
				t.Error("a known id should be fetched directly, not by listing every provider")
			}
		})
	}
}
