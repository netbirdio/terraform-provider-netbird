package provider

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_dnsRecordAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.DNSRecord
		zoneId   string
		expected DNSRecordModel
	}{
		{
			resource: &api.DNSRecord{
				Id:      "rec123",
				Name:    "www.netbird.io",
				Type:    api.DNSRecordTypeA,
				Content: "192.168.1.1",
				Ttl:     300,
			},
			zoneId: "zone123",
			expected: DNSRecordModel{
				Id:      types.StringValue("rec123"),
				ZoneId:  types.StringValue("zone123"),
				Name:    types.StringValue("www.netbird.io"),
				Type:    types.StringValue("A"),
				Content: types.StringValue("192.168.1.1"),
				Ttl:     types.Int64Value(300),
			},
		},
		{
			resource: &api.DNSRecord{
				Id:      "rec456",
				Name:    "api.netbird.io",
				Type:    api.DNSRecordTypeAAAA,
				Content: "2001:db8::1",
				Ttl:     600,
			},
			zoneId: "zone456",
			expected: DNSRecordModel{
				Id:      types.StringValue("rec456"),
				ZoneId:  types.StringValue("zone456"),
				Name:    types.StringValue("api.netbird.io"),
				Type:    types.StringValue("AAAA"),
				Content: types.StringValue("2001:db8::1"),
				Ttl:     types.Int64Value(600),
			},
		},
		{
			resource: &api.DNSRecord{
				Id:      "rec789",
				Name:    "mail.netbird.io",
				Type:    api.DNSRecordTypeCNAME,
				Content: "mail.example.com",
				Ttl:     3600,
			},
			zoneId: "zone789",
			expected: DNSRecordModel{
				Id:      types.StringValue("rec789"),
				ZoneId:  types.StringValue("zone789"),
				Name:    types.StringValue("mail.netbird.io"),
				Type:    types.StringValue("CNAME"),
				Content: types.StringValue("mail.example.com"),
				Ttl:     types.Int64Value(3600),
			},
		},
	}

	for _, c := range cases {
		var out DNSRecordModel
		dnsRecordAPIToTerraform(c.resource, c.zoneId, &out)

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
