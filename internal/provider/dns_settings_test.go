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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_dnsSettingsAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.DNSSettings
		expected DNSSettingsModel
	}{
		{
			resource: &api.DNSSettings{
				DisabledManagementGroups: []string{},
			},
			expected: DNSSettingsModel{
				DisabledManagementGroups: types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.DNSSettings{
				DisabledManagementGroups: []string{"abc"},
			},
			expected: DNSSettingsModel{
				DisabledManagementGroups: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("abc")}),
			},
		},
	}

	for _, c := range cases {
		var out DNSSettingsModel
		outDiag := dnsSettingsAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_DNSSettings_Create(t *testing.T) {
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_dns_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, "[]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "0"),
				),
			},
		},
	})
}

func Test_DNSSettings_Update(t *testing.T) {
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_dns_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, "[]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "0"),
				),
			},
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.0", "group-all"),
				),
			},
		},
	})
}

func Test_DNSSettings_Delete(t *testing.T) {
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)
	rNameFull := "netbird_dns_settings." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "disabled_management_groups.0", "group-all"),
				),
			},
			{
				ResourceName: rName,
				Config:       testDNSSettingsResource(rName, `["group-all"]`),
				Destroy:      true,
				Check: func(s *terraform.State) error {
					settings, err := testClient().DNS.GetSettings(context.Background())
					if err != nil {
						return err
					}
					if len(settings.DisabledManagementGroups) != 1 || settings.DisabledManagementGroups[0] != "group-all" {
						return fmt.Errorf("shouldn't change anything on delete")
					}
					return nil
				},
			},
		},
	})

}

func testDNSSettingsResource(rName, disabledManagementGroups string) string {
	return fmt.Sprintf(`resource "netbird_dns_settings" "%s" {
		disabled_management_groups = %s
	}`, rName, disabledManagementGroups)
}
