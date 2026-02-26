package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_reverseProxyDomainAPIToTerraform(t *testing.T) {
	cases := []struct {
		name     string
		domain   *api.ReverseProxyDomain
		expected ReverseProxyDomainModel
	}{
		{
			name: "custom domain with target cluster",
			domain: &api.ReverseProxyDomain{
				Id:            "dom1",
				Domain:        "app.example.com",
				TargetCluster: valPtr("cluster1"),
				Type:          api.ReverseProxyDomainTypeCustom,
				Validated:     true,
			},
			expected: ReverseProxyDomainModel{
				Id:            types.StringValue("dom1"),
				Domain:        types.StringValue("app.example.com"),
				TargetCluster: types.StringValue("cluster1"),
				Type:          types.StringValue("custom"),
				Validated:     types.BoolValue(true),
			},
		},
		{
			name: "free domain without target cluster",
			domain: &api.ReverseProxyDomain{
				Id:            "dom2",
				Domain:        "myapp.netbird.app",
				TargetCluster: nil,
				Type:          api.ReverseProxyDomainTypeFree,
				Validated:     false,
			},
			expected: ReverseProxyDomainModel{
				Id:            types.StringValue("dom2"),
				Domain:        types.StringValue("myapp.netbird.app"),
				TargetCluster: types.StringNull(),
				Type:          types.StringValue("free"),
				Validated:     types.BoolValue(false),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out ReverseProxyDomainModel
			reverseProxyDomainAPIToTerraform(c.domain, &out)

			if out.Id.ValueString() != c.expected.Id.ValueString() {
				t.Errorf("Id mismatch: expected %s, got %s", c.expected.Id.ValueString(), out.Id.ValueString())
			}
			if out.Domain.ValueString() != c.expected.Domain.ValueString() {
				t.Errorf("Domain mismatch: expected %s, got %s", c.expected.Domain.ValueString(), out.Domain.ValueString())
			}
			if out.Type.ValueString() != c.expected.Type.ValueString() {
				t.Errorf("Type mismatch: expected %s, got %s", c.expected.Type.ValueString(), out.Type.ValueString())
			}
			if out.Validated.ValueBool() != c.expected.Validated.ValueBool() {
				t.Errorf("Validated mismatch: expected %v, got %v", c.expected.Validated.ValueBool(), out.Validated.ValueBool())
			}
			if out.TargetCluster.IsNull() != c.expected.TargetCluster.IsNull() {
				t.Errorf("TargetCluster null mismatch: expected null=%v, got null=%v", c.expected.TargetCluster.IsNull(), out.TargetCluster.IsNull())
			}
			if !c.expected.TargetCluster.IsNull() && out.TargetCluster.ValueString() != c.expected.TargetCluster.ValueString() {
				t.Errorf("TargetCluster mismatch: expected %s, got %s", c.expected.TargetCluster.ValueString(), out.TargetCluster.ValueString())
			}
		})
	}
}

func Test_reverseProxyDomainResourceSchema(t *testing.T) {
	r := &ReverseProxyDomain{}

	var metaResp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "netbird"}, &metaResp)
	if metaResp.TypeName != "netbird_reverse_proxy_domain" {
		t.Errorf("Expected type name netbird_reverse_proxy_domain, got %s", metaResp.TypeName)
	}

	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors")
	}

	attrs := schemaResp.Schema.Attributes
	requiredAttrs := []string{"domain", "target_cluster"}
	for _, attr := range requiredAttrs {
		if _, ok := attrs[attr]; !ok {
			t.Errorf("Expected required attribute %s in schema", attr)
		}
	}

	computedAttrs := []string{"id", "type", "validated"}
	for _, attr := range computedAttrs {
		if _, ok := attrs[attr]; !ok {
			t.Errorf("Expected computed attribute %s in schema", attr)
		}
	}

	var _ resource.Resource = &ReverseProxyDomain{}
	var _ resource.ResourceWithImportState = &ReverseProxyDomain{}
}

func Test_reverseProxyDomainDataSourceSchema(t *testing.T) {
	d := &ReverseProxyDomainDataSource{}

	var metaResp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "netbird"}, &metaResp)
	if metaResp.TypeName != "netbird_reverse_proxy_domain" {
		t.Errorf("Expected type name netbird_reverse_proxy_domain, got %s", metaResp.TypeName)
	}

	var schemaResp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors")
	}

	attrs := schemaResp.Schema.Attributes
	expectedAttrs := []string{"id", "domain", "target_cluster", "type", "validated"}
	for _, attr := range expectedAttrs {
		if _, ok := attrs[attr]; !ok {
			t.Errorf("Expected attribute %s in schema", attr)
		}
	}

	var _ datasource.DataSource = &ReverseProxyDomainDataSource{}
}

func Test_reverseProxyDomainResourceUpdate(t *testing.T) {
	r := &ReverseProxyDomain{}

	var resp resource.UpdateResponse
	r.Update(context.Background(), resource.UpdateRequest{}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("Expected Update to return an error diagnostic")
	}
	found := false
	for _, d := range resp.Diagnostics.Errors() {
		if d.Summary() == "Update not supported" {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'Update not supported' error diagnostic")
	}
}
