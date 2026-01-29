package provider

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

func Test_DNSRecord_Create(t *testing.T) {
	zoneName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_dns_record." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordResource(zoneName, "test.local", rName, "www", "A", "192.168.1.1", 300),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttrSet(rNameFull, "zone_id"),
					resource.TestCheckResourceAttr(rNameFull, "name", "www.test.local"),
					resource.TestCheckResourceAttr(rNameFull, "type", "A"),
					resource.TestCheckResourceAttr(rNameFull, "content", "192.168.1.1"),
					resource.TestCheckResourceAttr(rNameFull, "ttl", "300"),
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateIdFunc: testDNSRecordImportStateIdFunc(rNameFull),
				ImportStateVerify: true,
			},
		},
	})
}

func Test_DNSRecord_Update(t *testing.T) {
	zoneName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_dns_record." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordResource(zoneName, "test.local", rName, "www", "A", "192.168.1.1", 300),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "name", "www.test.local"),
					resource.TestCheckResourceAttr(rNameFull, "content", "192.168.1.1"),
					resource.TestCheckResourceAttr(rNameFull, "ttl", "300"),
				),
			},
			{
				Config: testDNSRecordResource(zoneName, "test.local", rName, "api", "A", "192.168.1.2", 600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "name", "api.test.local"),
					resource.TestCheckResourceAttr(rNameFull, "content", "192.168.1.2"),
					resource.TestCheckResourceAttr(rNameFull, "ttl", "600"),
				),
			},
		},
	})
}

func Test_DNSRecord_CNAME(t *testing.T) {
	zoneName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_dns_record." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordResource(zoneName, "test.local", rName, "mail", "CNAME", "mail.example.com", 300),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "name", "mail.test.local"),
					resource.TestCheckResourceAttr(rNameFull, "type", "CNAME"),
					resource.TestCheckResourceAttr(rNameFull, "content", "mail.example.com"),
				),
			},
		},
	})
}

func Test_DNSRecord_DataSource(t *testing.T) {
	zoneName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	dsNameFull := "data.netbird_dns_record." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSRecordResourceAndDataSource(zoneName, "datasource.local", rName, "test", "A", "10.0.0.1", 300),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dsNameFull, "id"),
					resource.TestCheckResourceAttrSet(dsNameFull, "zone_id"),
					resource.TestCheckResourceAttr(dsNameFull, "name", "test.datasource.local"),
					resource.TestCheckResourceAttr(dsNameFull, "type", "A"),
					resource.TestCheckResourceAttr(dsNameFull, "content", "10.0.0.1"),
				),
			},
		},
	})
}

func testDNSRecordResource(zoneName, domain, recordName, recordSubdomain, recordType, recordContent string, ttl int) string {
	return fmt.Sprintf(`
resource "netbird_dns_zone" "%s" {
  name                = "%s"
  domain              = "%s"
  distribution_groups = ["group-all"]
}

resource "netbird_dns_record" "%s" {
  zone_id = netbird_dns_zone.%s.id
  name    = "%s.%s"
  type    = "%s"
  content = "%s"
  ttl     = %d
}
`, zoneName, zoneName, domain, recordName, zoneName, recordSubdomain, domain, recordType, recordContent, ttl)
}

func testDNSRecordResourceAndDataSource(zoneName, domain, recordName, recordSubdomain, recordType, recordContent string, ttl int) string {
	return testDNSRecordResource(zoneName, domain, recordName, recordSubdomain, recordType, recordContent, ttl) + fmt.Sprintf(`
data "netbird_dns_record" "%s" {
  zone_id = netbird_dns_zone.%s.id
  name    = netbird_dns_record.%s.name
  type    = netbird_dns_record.%s.type
}
`, recordName, zoneName, recordName, recordName)
}

func testDNSRecordImportStateIdFunc(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("not found: %s", resourceName)
		}

		zoneId := rs.Primary.Attributes["zone_id"]
		id := rs.Primary.ID

		if zoneId == "" || id == "" {
			return "", fmt.Errorf("zone_id or id is not set")
		}

		return fmt.Sprintf("%s:%s", zoneId, id), nil
	}
}
