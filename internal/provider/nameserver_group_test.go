package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
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
