package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func Test_reverseProxyClustersDataSourceSchema(t *testing.T) {
	d := &ReverseProxyClustersDataSource{}

	var metaResp datasource.MetadataResponse
	d.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "netbird"}, &metaResp)
	if metaResp.TypeName != "netbird_reverse_proxy_clusters" {
		t.Errorf("Expected type name netbird_reverse_proxy_clusters, got %s", metaResp.TypeName)
	}

	var schemaResp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors")
	}

	attrs := schemaResp.Schema.Attributes
	if _, ok := attrs["clusters"]; !ok {
		t.Error("Expected clusters attribute in schema")
	}

	var _ datasource.DataSource = &ReverseProxyClustersDataSource{}
}
