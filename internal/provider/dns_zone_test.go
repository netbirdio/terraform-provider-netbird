package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_dnsZoneAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.Zone
		expected DNSZoneModel
	}{
		{
			resource: &api.Zone{
				Id:                 "zone123",
				Name:               "Test Zone",
				Domain:             "example.com",
				Enabled:            true,
				EnableSearchDomain: false,
				DistributionGroups: []string{},
			},
			expected: DNSZoneModel{
				Id:                 types.StringValue("zone123"),
				Name:               types.StringValue("Test Zone"),
				Domain:             types.StringValue("example.com"),
				Enabled:            types.BoolValue(true),
				EnableSearchDomain: types.BoolValue(false),
				DistributionGroups: types.SetValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Zone{
				Id:                 "zone456",
				Name:               "Internal Zone",
				Domain:             "internal.local",
				Enabled:            false,
				EnableSearchDomain: true,
				DistributionGroups: []string{"grp1", "grp2"},
			},
			expected: DNSZoneModel{
				Id:                 types.StringValue("zone456"),
				Name:               types.StringValue("Internal Zone"),
				Domain:             types.StringValue("internal.local"),
				Enabled:            types.BoolValue(false),
				EnableSearchDomain: types.BoolValue(true),
				DistributionGroups: types.SetValueMust(types.StringType, []attr.Value{
					types.StringValue("grp1"),
					types.StringValue("grp2"),
				}),
			},
		},
	}

	for _, c := range cases {
		var out DNSZoneModel
		outDiag := dnsZoneAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_DNSZone_Create(t *testing.T) {
	rName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_dns_zone." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSZoneResource(rName, "test.local", true, false, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "domain", "test.local"),
					resource.TestCheckResourceAttr(rNameFull, "enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "enable_search_domain", "false"),
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func Test_DNSZone_Update(t *testing.T) {
	rName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_dns_zone." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSZoneResource(rName, "test.local", true, false, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "domain", "test.local"),
					resource.TestCheckResourceAttr(rNameFull, "enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "enable_search_domain", "false"),
				),
			},
			{
				ResourceName: rName,
				Config:       testDNSZoneResource(rName, "test.local", false, true, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "domain", "test.local"),
					resource.TestCheckResourceAttr(rNameFull, "enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "enable_search_domain", "true"),
				),
			},
		},
	})
}

func Test_DNSZone_DataSource(t *testing.T) {
	rName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	dsNameFull := "data.netbird_dns_zone." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDNSZoneResourceAndDataSource(rName, "datasource.local", true, false, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dsNameFull, "id"),
					resource.TestCheckResourceAttr(dsNameFull, "name", rName),
					resource.TestCheckResourceAttr(dsNameFull, "domain", "datasource.local"),
					resource.TestCheckResourceAttr(dsNameFull, "enabled", "true"),
					resource.TestCheckResourceAttr(dsNameFull, "enable_search_domain", "false"),
				),
			},
		},
	})
}

func testDNSZoneResource(name, domain string, enabled, enableSearchDomain bool, distributionGroups string) string {
	return fmt.Sprintf(`
resource "netbird_dns_zone" "%s" {
  name                 = "%s"
  domain               = "%s"
  enabled              = %t
  enable_search_domain = %t
  distribution_groups  = %s
}
`, name, name, domain, enabled, enableSearchDomain, distributionGroups)
}

func testDNSZoneResourceAndDataSource(name, domain string, enabled, enableSearchDomain bool, distributionGroups string) string {
	return testDNSZoneResource(name, domain, enabled, enableSearchDomain, distributionGroups) + fmt.Sprintf(`
data "netbird_dns_zone" "%s" {
  name = netbird_dns_zone.%s.name
}
`, name, name)
}
