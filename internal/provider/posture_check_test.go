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

func Test_PostureCheck_Create(t *testing.T) {
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_posture_check." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPostureCheckResource(rName, `posture_check`, `0.40.0`, `15`, `10`, `12`, `6.8.0`, `2531`, `EG`, `Cairo`, `allow`, `15.160.0.0/16`, `deny`, `/root`, `C:\\process.exe`, `/macpath`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "posture_check"),
					resource.TestCheckResourceAttr(rNameFull, "netbird_version_check.min_version", "0.40.0"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.android_min_version", "15"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.ios_min_version", "10"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.darwin_min_version", "12"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.linux_min_kernel_version", "6.8.0"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.windows_min_kernel_version", "2531"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.locations.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.locations.0.country_code", "EG"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.locations.0.city_name", "Cairo"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.action", "allow"),
					resource.TestCheckResourceAttr(rNameFull, "peer_network_range_check.ranges.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "peer_network_range_check.ranges.0", "15.160.0.0/16"),
					resource.TestCheckResourceAttr(rNameFull, "peer_network_range_check.action", "deny"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.0.linux_path", "/root"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.0.windows_path", "C:\\process.exe"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.0.mac_path", "/macpath"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						pCheck, err := testClient().PostureChecks.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"name":                                        {rName, pCheck.Name},
							"netbird_version_check.min_version":           {"0.40.0", pCheck.Checks.NbVersionCheck.MinVersion},
							"os_version_check.android_min_version":        {"15", pCheck.Checks.OsVersionCheck.Android.MinVersion},
							"os_version_check.ios_min_version":            {"10", pCheck.Checks.OsVersionCheck.Ios.MinVersion},
							"os_version_check.darwin_min_version":         {"12", pCheck.Checks.OsVersionCheck.Darwin.MinVersion},
							"os_version_check.linux_min_kernel_version":   {"6.8.0", pCheck.Checks.OsVersionCheck.Linux.MinKernelVersion},
							"os_version_check.windows_min_kernel_version": {"2531", pCheck.Checks.OsVersionCheck.Windows.MinKernelVersion},
							"geo_location_check.locations.#":              {int(1), len(pCheck.Checks.GeoLocationCheck.Locations)},
							"geo_location_check.locations.0.country_code": {"EG", pCheck.Checks.GeoLocationCheck.Locations[0].CountryCode},
							"geo_location_check.locations.0.city_name":    {"Cairo", *pCheck.Checks.GeoLocationCheck.Locations[0].CityName},
							"geo_location_check.action":                   {"allow", string(pCheck.Checks.GeoLocationCheck.Action)},
							"peer_network_range_check.ranges.#":           {int(1), len(pCheck.Checks.PeerNetworkRangeCheck.Ranges)},
							"peer_network_range_check.ranges.0":           {"15.160.0.0/16", pCheck.Checks.PeerNetworkRangeCheck.Ranges[0]},
							"peer_network_range_check.action":             {"deny", string(pCheck.Checks.PeerNetworkRangeCheck.Action)},
							"process_check.#":                             {int(1), len(pCheck.Checks.ProcessCheck.Processes)},
							"process_check.0.linux_path":                  {"/root", pCheck.Checks.ProcessCheck.Processes[0].LinuxPath},
							"process_check.0.windows_path":                {"C:\\process.exe", pCheck.Checks.ProcessCheck.Processes[0].WindowsPath},
							"process_check.0.mac_path":                    {"/macpath", pCheck.Checks.ProcessCheck.Processes[0].MacPath},
						})
					},
				),
			},
		},
	})
}

func Test_PostureCheck_Update(t *testing.T) {
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_posture_check." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPostureCheckResource(rName, `posture_check`, `0.40.0`, `15`, `10`, `12`, `6.8.0`, `2531`, `EG`, `Cairo`, `allow`, `15.160.0.0/16`, `deny`, `/root`, `C:\\process.exe`, `/macpath`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testPostureCheckResource(rName, `posture_check_updated`, `0.45.0`, `16`, `11`, `13`, `6.9.0`, `2532`, `US`, `Florida`, `deny`, `16.160.0.0/16`, `allow`, `/rootnt`, `C:\\processnt.exe`, `/macpathnt`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "description", "posture_check_updated"),
					resource.TestCheckResourceAttr(rNameFull, "netbird_version_check.min_version", "0.45.0"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.android_min_version", "16"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.ios_min_version", "11"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.darwin_min_version", "13"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.linux_min_kernel_version", "6.9.0"),
					resource.TestCheckResourceAttr(rNameFull, "os_version_check.windows_min_kernel_version", "2532"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.locations.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.locations.0.country_code", "US"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.locations.0.city_name", "Florida"),
					resource.TestCheckResourceAttr(rNameFull, "geo_location_check.action", "deny"),
					resource.TestCheckResourceAttr(rNameFull, "peer_network_range_check.ranges.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "peer_network_range_check.ranges.0", "16.160.0.0/16"),
					resource.TestCheckResourceAttr(rNameFull, "peer_network_range_check.action", "allow"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.0.linux_path", "/rootnt"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.0.windows_path", "C:\\processnt.exe"),
					resource.TestCheckResourceAttr(rNameFull, "process_check.0.mac_path", "/macpathnt"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						pCheck, err := testClient().PostureChecks.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":                                        {rName, pCheck.Name},
							"netbird_version_check.min_version":           {"0.45.0", pCheck.Checks.NbVersionCheck.MinVersion},
							"os_version_check.android_min_version":        {"16", pCheck.Checks.OsVersionCheck.Android.MinVersion},
							"os_version_check.ios_min_version":            {"11", pCheck.Checks.OsVersionCheck.Ios.MinVersion},
							"os_version_check.darwin_min_version":         {"13", pCheck.Checks.OsVersionCheck.Darwin.MinVersion},
							"os_version_check.linux_min_kernel_version":   {"6.9.0", pCheck.Checks.OsVersionCheck.Linux.MinKernelVersion},
							"os_version_check.windows_min_kernel_version": {"2532", pCheck.Checks.OsVersionCheck.Windows.MinKernelVersion},
							"geo_location_check.locations.#":              {int(1), len(pCheck.Checks.GeoLocationCheck.Locations)},
							"geo_location_check.locations.0.country_code": {"US", pCheck.Checks.GeoLocationCheck.Locations[0].CountryCode},
							"geo_location_check.locations.0.city_name":    {"Florida", *pCheck.Checks.GeoLocationCheck.Locations[0].CityName},
							"geo_location_check.action":                   {"deny", string(pCheck.Checks.GeoLocationCheck.Action)},
							"peer_network_range_check.ranges.#":           {int(1), len(pCheck.Checks.PeerNetworkRangeCheck.Ranges)},
							"peer_network_range_check.ranges.0":           {"16.160.0.0/16", pCheck.Checks.PeerNetworkRangeCheck.Ranges[0]},
							"peer_network_range_check.action":             {"allow", string(pCheck.Checks.PeerNetworkRangeCheck.Action)},
							"process_check.#":                             {int(1), len(pCheck.Checks.ProcessCheck.Processes)},
							"process_check.0.linux_path":                  {"/rootnt", pCheck.Checks.ProcessCheck.Processes[0].LinuxPath},
							"process_check.0.windows_path":                {"C:\\processnt.exe", pCheck.Checks.ProcessCheck.Processes[0].WindowsPath},
							"process_check.0.mac_path":                    {"/macpathnt", pCheck.Checks.ProcessCheck.Processes[0].MacPath},
						})
					},
				),
			},
		},
	})
}

func testPostureCheckResource(rName, desc, nbVersion, andVersion, iosVersion, macVersion, linuxVersion, winVersion, country, city, geoAction, netRange, netRangeAction, processLinux, processWindows, processMac string) string {
	return fmt.Sprintf(`resource "netbird_posture_check" "%s" {
  name        = "%s"
  description = "%s"

  netbird_version_check {
    min_version = "%s"
  }

  os_version_check {
    android_min_version        = "%s"
    ios_min_version            = "%s"
    darwin_min_version         = "%s"
    linux_min_kernel_version   = "%s"
    windows_min_kernel_version = "%s"
  }

  geo_location_check {
    locations = [
      {
        country_code = "%s"
				city_name    = "%s"
      }
    ]
    action = "%s"
  }

  peer_network_range_check {
    ranges = [
      "%s"
    ]

    action = "%s"
  }

  process_check {
    linux_path   = "%s"
    windows_path = "%s"
		mac_path     = "%s"
  }
}`, rName, rName, desc, nbVersion, andVersion, iosVersion, macVersion, linuxVersion, winVersion, country, city, geoAction, netRange, netRangeAction, processLinux, processWindows, processMac)
}
