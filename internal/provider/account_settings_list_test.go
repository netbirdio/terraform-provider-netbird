package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

// Regression tests for the account settings resource and data source when the account list
// request fails or returns no account. Before the fix, Read indexed accounts[0] ahead of the
// error check and panicked with "index out of range [0] with length 0" on a revoked token.

// The body the management API returns for a revoked or expired personal access token.
const invalidTokenBody = `{"message":"invalid token: pat: 0123456789abcdef not found","code":404}`

// accountsClient returns a client whose /api/accounts answers with the given status and body.
func accountsClient(t *testing.T, status int, body string) *netbird.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return netbird.NewWithOptions(netbird.WithManagementURL(server.URL), netbird.WithPAT("test-token"))
}

// objectValue builds a value of the schema's object type with every attribute null except id.
func objectValue(t *testing.T, typ tftypes.Type, id *string) tftypes.Value {
	t.Helper()
	object, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, not an object", typ)
	}
	values := make(map[string]tftypes.Value, len(object.AttributeTypes))
	for name, attrType := range object.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	if id != nil {
		values["id"] = tftypes.NewValue(tftypes.String, *id)
	}
	return tftypes.NewValue(object, values)
}

func accountSettingsResource(t *testing.T, client *netbird.Client) (*AccountSettings, resource.SchemaResponse) {
	t.Helper()
	r := &AccountSettings{client: client}
	var schema resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
	if schema.Diagnostics.HasError() {
		t.Fatalf("schema: %v", schema.Diagnostics)
	}
	return r, schema
}

func TestAccountSettingsReadListErrorKeepsState(t *testing.T) {
	ctx := context.Background()
	r, schema := accountSettingsResource(t, accountsClient(t, http.StatusNotFound, invalidTokenBody))
	id := "account-1"
	state := tfsdk.State{Schema: schema.Schema, Raw: objectValue(t, schema.Schema.Type().TerraformType(ctx), &id)}

	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected a diagnostic for the failed list request")
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("a failed list request must not remove the resource from state")
	}
}

func TestAccountSettingsReadEmptyListRemovesState(t *testing.T) {
	ctx := context.Background()
	r, schema := accountSettingsResource(t, accountsClient(t, http.StatusOK, `[]`))
	id := "account-1"
	state := tfsdk.State{Schema: schema.Schema, Raw: objectValue(t, schema.Schema.Type().TerraformType(ctx), &id)}

	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("an empty account list must remove the resource from state")
	}
}

func TestAccountSettingsCreateEmptyListReportsError(t *testing.T) {
	ctx := context.Background()
	r, schema := accountSettingsResource(t, accountsClient(t, http.StatusOK, `[]`))
	plan := tfsdk.Plan{Schema: schema.Schema, Raw: objectValue(t, schema.Schema.Type().TerraformType(ctx), nil)}

	resp := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(ctx), nil)}}
	r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected a diagnostic for an empty account list")
	}
}

func TestAccountSettingsUpdateEmptyListReportsError(t *testing.T) {
	ctx := context.Background()
	r, schema := accountSettingsResource(t, accountsClient(t, http.StatusOK, `[]`))
	id := "account-1"
	value := objectValue(t, schema.Schema.Type().TerraformType(ctx), &id)
	plan := tfsdk.Plan{Schema: schema.Schema, Raw: value}
	state := tfsdk.State{Schema: schema.Schema, Raw: value}

	resp := resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected a diagnostic for an empty account list")
	}
}

func TestAccountSettingsDataSourceReadListFailures(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"invalid token": {status: http.StatusNotFound, body: invalidTokenBody},
		"empty list":    {status: http.StatusOK, body: `[]`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			d := &AccountSettingsDataSource{client: accountsClient(t, tc.status, tc.body)}
			var schema datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &schema)
			if schema.Diagnostics.HasError() {
				t.Fatalf("schema: %v", schema.Diagnostics)
			}
			config := tfsdk.Config{Schema: schema.Schema, Raw: objectValue(t, schema.Schema.Type().TerraformType(ctx), nil)}

			resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(ctx), nil)}}
			d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)

			if !resp.Diagnostics.HasError() {
				t.Fatal("expected a diagnostic instead of a panic or a silent result")
			}
		})
	}
}
