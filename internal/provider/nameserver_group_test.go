package provider

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

var nsObjAttrs = map[string]attr.Type{
	"ip":      types.StringType,
	"ns_type": types.StringType,
	"port":    types.Int32Type,
}

func Test_nameserverGroupAPIToTerraform(t *testing.T) {

	cases := []struct {
		resource *api.NameserverGroup
		expected NameserverGroupModel
	}{
		{
			resource: &api.NameserverGroup{
				Description: "Test",
				Domains:     []string{},
				Enabled:     false,
				Groups:      []string{},
				Id:          "c1",
				Name:        "DNS",
				Nameservers: []api.Nameserver{
					{
						Ip:     "8.8.8.8",
						NsType: api.NameserverNsTypeUdp,
						Port:   53,
					},
				},
				Primary:              false,
				SearchDomainsEnabled: false,
			},
			expected: NameserverGroupModel{
				Id:                   types.StringValue("c1"),
				Name:                 types.StringValue("DNS"),
				Description:          types.StringValue("Test"),
				Groups:               types.ListValueMust(types.StringType, []attr.Value{}),
				Domains:              types.ListValueMust(types.StringType, []attr.Value{}),
				Enabled:              types.BoolValue(false),
				Primary:              types.BoolValue(false),
				SearchDomainsEnabled: types.BoolValue(false),
				Nameservers: types.ListValueMust(types.ObjectType{
					AttrTypes: nsObjAttrs,
				}, []attr.Value{
					types.ObjectValueMust(nsObjAttrs, map[string]attr.Value{
						"ip":      types.StringValue("8.8.8.8"),
						"ns_type": types.StringValue("udp"),
						"port":    types.Int32Value(53),
					}),
				}),
			},
		},
		{
			resource: &api.NameserverGroup{
				Description: "",
				Domains:     []string{"google.com"},
				Enabled:     true,
				Groups:      []string{"g1", "g2"},
				Id:          "c2",
				Name:        "second",
				Nameservers: []api.Nameserver{
					{
						Ip:     "1.1.1.1",
						NsType: api.NameserverNsTypeUdp,
						Port:   5353,
					},
				},
				Primary:              true,
				SearchDomainsEnabled: true,
			},
			expected: NameserverGroupModel{
				Id:                   types.StringValue("c2"),
				Name:                 types.StringValue("second"),
				Description:          types.StringValue(""),
				Groups:               types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1"), types.StringValue("g2")}),
				Domains:              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("google.com")}),
				Enabled:              types.BoolValue(true),
				Primary:              types.BoolValue(true),
				SearchDomainsEnabled: types.BoolValue(true),
				Nameservers: types.ListValueMust(types.ObjectType{
					AttrTypes: nsObjAttrs,
				}, []attr.Value{
					types.ObjectValueMust(nsObjAttrs, map[string]attr.Value{
						"ip":      types.StringValue("1.1.1.1"),
						"ns_type": types.StringValue("udp"),
						"port":    types.Int32Value(5353),
					}),
				}),
			},
		},
	}

	for _, c := range cases {
		var out NameserverGroupModel
		outDiag := nameserverGroupAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_nameserverGroupTerraformToAPI(t *testing.T) {
	cases := []struct {
		input       *NameserverGroupModel
		expected    api.NameserverGroupRequest
		expectError bool
	}{
		{
			input: &NameserverGroupModel{
				Id:                   types.StringValue("c2"),
				Name:                 types.StringValue("second"),
				Description:          types.StringValue(""),
				Groups:               types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1"), types.StringValue("g2")}),
				Domains:              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("google.com")}),
				Enabled:              types.BoolValue(true),
				Primary:              types.BoolValue(false),
				SearchDomainsEnabled: types.BoolValue(false),
				Nameservers: types.ListValueMust(types.ObjectType{
					AttrTypes: nsObjAttrs,
				}, []attr.Value{
					types.ObjectValueMust(nsObjAttrs, map[string]attr.Value{
						"ip":      types.StringValue("1.1.1.1"),
						"ns_type": types.StringValue("udp"),
						"port":    types.Int32Value(5353),
					}),
				}),
			},
			expected: api.NameserverGroupRequest{
				Description: "",
				Domains:     []string{"google.com"},
				Enabled:     true,
				Groups:      []string{"g1", "g2"},
				Name:        "second",
				Nameservers: []api.Nameserver{
					{
						Ip:     "1.1.1.1",
						NsType: api.NameserverNsTypeUdp,
						Port:   5353,
					},
				},
				Primary:              false,
				SearchDomainsEnabled: false,
			},
		},
	}

	for _, c := range cases {
		out, outDiag := nameserverGroupTerraformToAPI(context.Background(), c.input)
		if outDiag.HasError() && !c.expectError {
			t.Fatalf("Expected no error diagnostics, found %d errors: %#v", outDiag.ErrorsCount(), outDiag)
		} else if !outDiag.HasError() && c.expectError {
			t.Fatal("Expected error diagnostics, found no errors")
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_fqdnRegex(t *testing.T) {
	cases := []struct {
		fqdn     string
		expected bool
	}{
		{
			fqdn:     "example.com",
			expected: true,
		},
		{
			fqdn:     "test-example.com",
			expected: true,
		},
		{
			fqdn:     "test-example.com.",
			expected: true,
		},
		{
			fqdn:     "test",
			expected: false,
		},
		{
			fqdn:     "company.internal",
			expected: true,
		},
		{
			fqdn:     "company.internal-dns",
			expected: true,
		},
		{
			fqdn:     "company,name.internal-dns",
			expected: false,
		},
	}

	for _, c := range cases {
		if regexp.MustCompile(fqdnRegex).MatchString(c.fqdn) != c.expected {
			t.Fatalf("Expected %t for fqdn %s", c.expected, c.fqdn)
		}
	}
}

func Test_NameserverGroup_Create(t *testing.T) {
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `1.1.1.1`, `udp`, `53`, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						nsGroup, err := testClient().DNS.GetNameserverGroup(context.Background(), gID)
						if err != nil {
							return err
						}
						if nsGroup.Name != rName {
							return fmt.Errorf("NameserverGroup name mismatch, expected %s, found %s on management server", rName, nsGroup.Name)
						}
						if len(nsGroup.Nameservers) != 1 {
							return fmt.Errorf("NameserverGroup Nameservers mismatch, expected 1, found %d", len(nsGroup.Nameservers))
						}
						if nsGroup.Nameservers[0].Ip != `1.1.1.1` {
							return fmt.Errorf("NameserverGroup Nameservers.0.Ip mismatch, expected 1.1.1.1, found %s", nsGroup.Nameservers[0].Ip)
						}
						if nsGroup.Nameservers[0].NsType != `udp` {
							return fmt.Errorf("NameserverGroup Nameservers.0.NsType mismatch, expected udp, found %s", nsGroup.Nameservers[0].NsType)
						}
						if nsGroup.Nameservers[0].Port != 53 {
							return fmt.Errorf("NameserverGroup Nameservers.0.Port mismatch, expected 53, found %d", nsGroup.Nameservers[0].Port)
						}
						if len(nsGroup.Groups) != 1 {
							return fmt.Errorf("NameserverGroup Groups mismatch, expected 1, found %d", len(nsGroup.Groups))
						}
						if nsGroup.Groups[0] != "group-all" {
							return fmt.Errorf("NameserverGroup Groups.0 mismatch, expected group-all, found %s", nsGroup.Groups[0])
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_NameserverGroup_Update(t *testing.T) {
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_nameserver_group." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `1.1.1.1`, `udp`, `53`, `["group-all"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testNameserverGroupResource(rName, `8.8.8.8`, `udp`, `5353`, `["group-notall"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					func(s *terraform.State) error {
						gID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						nsGroup, err := testClient().DNS.GetNameserverGroup(context.Background(), gID)
						if err != nil {
							return err
						}
						if nsGroup.Name != rName {
							return fmt.Errorf("NameserverGroup name mismatch, expected %s, found %s on management server", rName, nsGroup.Name)
						}
						if len(nsGroup.Nameservers) != 1 {
							return fmt.Errorf("NameserverGroup Nameservers mismatch, expected 1, found %d", len(nsGroup.Nameservers))
						}
						if nsGroup.Nameservers[0].Ip != `8.8.8.8` {
							return fmt.Errorf("NameserverGroup Nameservers.0.Ip mismatch, expected 8.8.8.8, found %s", nsGroup.Nameservers[0].Ip)
						}
						if nsGroup.Nameservers[0].NsType != `udp` {
							return fmt.Errorf("NameserverGroup Nameservers.0.NsType mismatch, expected udp, found %s", nsGroup.Nameservers[0].NsType)
						}
						if nsGroup.Nameservers[0].Port != 5353 {
							return fmt.Errorf("NameserverGroup Nameservers.0.Port mismatch, expected 5353, found %d", nsGroup.Nameservers[0].Port)
						}
						if len(nsGroup.Groups) != 1 {
							return fmt.Errorf("NameserverGroup Groups mismatch, expected 1, found %d", len(nsGroup.Groups))
						}
						if nsGroup.Groups[0] != "group-notall" {
							return fmt.Errorf("NameserverGroup Groups.0 mismatch, expected group-notall, found %s", nsGroup.Groups[0])
						}
						return nil
					},
				),
			},
		},
	})
}

func testNameserverGroupResource(rName, ip, nsType, port, groups string) string {
	return fmt.Sprintf(`resource "netbird_nameserver_group" "%s" {
	name = "%s"
	nameservers = [
		{
			ip = "%s"
			ns_type = "%s"
			port = %s
		}
	]
	groups = %s
}`, rName, rName, ip, nsType, port, groups)
}
