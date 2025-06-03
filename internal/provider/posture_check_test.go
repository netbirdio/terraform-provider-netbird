package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/management/server/http/api"
)

func Test_postureCheckAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.PostureCheck
		expected PostureCheckModel
	}{
		{
			resource: &api.PostureCheck{
				Id:          "pc1",
				Description: valPtr("PC"),
				Name:        "PC",
				Checks: api.Checks{
					GeoLocationCheck: &api.GeoLocationCheck{
						Action: api.GeoLocationCheckActionAllow,
						Locations: []api.Location{
							{
								CityName:    valPtr(api.CityName("Cairo")),
								CountryCode: api.CountryCode("EG"),
							},
						},
					},
					NbVersionCheck: &api.MinVersionCheck{
						MinVersion: "0.40.0",
					},
					OsVersionCheck: &api.OSVersionCheck{
						Android: &api.MinVersionCheck{
							MinVersion: "0.0.1",
						},
						Darwin: &api.MinVersionCheck{
							MinVersion: "0.0.2",
						},
						Ios: &api.MinVersionCheck{
							MinVersion: "0.0.3",
						},
						Linux: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.4",
						},
						Windows: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.5",
						},
					},
					PeerNetworkRangeCheck: &api.PeerNetworkRangeCheck{
						Action: api.PeerNetworkRangeCheckActionAllow,
						Ranges: []string{"1.1.1.1/24"},
					},
					ProcessCheck: &api.ProcessCheck{
						Processes: []api.Process{
							{
								LinuxPath:   valPtr("/linux"),
								MacPath:     valPtr("/mac"),
								WindowsPath: valPtr("C:\\windows"),
							},
						},
					},
				},
			},
			expected: PostureCheckModel{
				Id:                  types.StringValue("pc1"),
				Name:                types.StringValue("PC"),
				Description:         types.StringValue("PC"),
				NetbirdVersionCheck: types.ObjectValueMust(map[string]attr.Type{"min_version": types.StringType}, map[string]attr.Value{"min_version": types.StringValue("0.40.0")}),
				OSVersionCheck: types.ObjectValueMust(map[string]attr.Type{
					"android_min_version":        types.StringType,
					"ios_min_version":            types.StringType,
					"darwin_min_version":         types.StringType,
					"linux_min_kernel_version":   types.StringType,
					"windows_min_kernel_version": types.StringType,
				}, map[string]attr.Value{
					"android_min_version":        types.StringValue("0.0.1"),
					"darwin_min_version":         types.StringValue("0.0.2"),
					"ios_min_version":            types.StringValue("0.0.3"),
					"linux_min_kernel_version":   types.StringValue("0.0.4"),
					"windows_min_kernel_version": types.StringValue("0.0.5"),
				}),
				GeoLocationCheck: types.ObjectValueMust(map[string]attr.Type{
					"locations": types.ListType{
						ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"country_code": types.StringType,
								"city_name":    types.StringType,
							},
						},
					},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"locations": types.ListValueMust(types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						},
					}, []attr.Value{
						types.ObjectValueMust(map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						}, map[string]attr.Value{
							"country_code": types.StringValue("EG"),
							"city_name":    types.StringValue("Cairo"),
						}),
					}),
				}),
				PeerNetworkRangeCheck: types.ObjectValueMust(map[string]attr.Type{
					"ranges": types.ListType{ElemType: types.StringType},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"ranges": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("1.1.1.1/24")}),
				}),
				ProcessCheck: types.ListValueMust(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					},
				}, []attr.Value{
					types.ObjectValueMust(map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					}, map[string]attr.Value{
						"linux_path":   types.StringValue("/linux"),
						"mac_path":     types.StringValue("/mac"),
						"windows_path": types.StringValue("C:\\windows"),
					}),
				}),
			},
		},
	}

	for _, c := range cases {
		var out PostureCheckModel
		outDiag := postureCheckAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_postureCheckTerraformToAPI(t *testing.T) {
	cases := []struct {
		resource PostureCheckModel
		expected api.PostureCheckUpdate
	}{
		{
			resource: PostureCheckModel{
				Id:                  types.StringValue("pc1"),
				Name:                types.StringValue("PC"),
				Description:         types.StringValue("desc"),
				NetbirdVersionCheck: types.ObjectValueMust(map[string]attr.Type{"min_version": types.StringType}, map[string]attr.Value{"min_version": types.StringValue("0.40.0")}),
				OSVersionCheck: types.ObjectValueMust(map[string]attr.Type{
					"android_min_version":        types.StringType,
					"ios_min_version":            types.StringType,
					"darwin_min_version":         types.StringType,
					"linux_min_kernel_version":   types.StringType,
					"windows_min_kernel_version": types.StringType,
				}, map[string]attr.Value{
					"android_min_version":        types.StringValue("0.0.1"),
					"darwin_min_version":         types.StringValue("0.0.2"),
					"ios_min_version":            types.StringValue("0.0.3"),
					"linux_min_kernel_version":   types.StringValue("0.0.4"),
					"windows_min_kernel_version": types.StringValue("0.0.5"),
				}),
				GeoLocationCheck: types.ObjectValueMust(map[string]attr.Type{
					"locations": types.ListType{
						ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"country_code": types.StringType,
								"city_name":    types.StringType,
							},
						},
					},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"locations": types.ListValueMust(types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						},
					}, []attr.Value{
						types.ObjectValueMust(map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						}, map[string]attr.Value{
							"country_code": types.StringValue("EG"),
							"city_name":    types.StringValue("Cairo"),
						}),
					}),
				}),
				PeerNetworkRangeCheck: types.ObjectValueMust(map[string]attr.Type{
					"ranges": types.ListType{ElemType: types.StringType},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"ranges": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("1.1.1.1/24")}),
				}),
				ProcessCheck: types.ListValueMust(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					},
				}, []attr.Value{
					types.ObjectValueMust(map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					}, map[string]attr.Value{
						"linux_path":   types.StringValue("/linux"),
						"mac_path":     types.StringValue("/mac"),
						"windows_path": types.StringValue("C:\\windows"),
					}),
				}),
			},
			expected: api.PostureCheckUpdate{
				Name:        "PC",
				Description: "desc",
				Checks: &api.Checks{
					GeoLocationCheck: &api.GeoLocationCheck{
						Action: api.GeoLocationCheckActionAllow,
						Locations: []api.Location{
							{
								CityName:    valPtr(api.CityName("Cairo")),
								CountryCode: api.CountryCode("EG"),
							},
						},
					},
					NbVersionCheck: &api.MinVersionCheck{
						MinVersion: "0.40.0",
					},
					OsVersionCheck: &api.OSVersionCheck{
						Android: &api.MinVersionCheck{
							MinVersion: "0.0.1",
						},
						Darwin: &api.MinVersionCheck{
							MinVersion: "0.0.2",
						},
						Ios: &api.MinVersionCheck{
							MinVersion: "0.0.3",
						},
						Linux: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.4",
						},
						Windows: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.5",
						},
					},
					PeerNetworkRangeCheck: &api.PeerNetworkRangeCheck{
						Action: api.PeerNetworkRangeCheckActionAllow,
						Ranges: []string{"1.1.1.1/24"},
					},
					ProcessCheck: &api.ProcessCheck{
						Processes: []api.Process{
							{
								LinuxPath:   valPtr("/linux"),
								MacPath:     valPtr("/mac"),
								WindowsPath: valPtr("C:\\windows"),
							},
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		out, outDiag := postureCheckTerraformToAPI(context.Background(), c.resource)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}
