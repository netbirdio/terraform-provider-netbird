//go:build e2e

package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_PostureCheck_Create(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_posture_check." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().PostureChecks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPostureCheckResource(rName, `posture_check`, `0.40.0`, `15`, `10`, `12`, `6.8.0`, `2531`, `EG`, `Cairo`, `allow`, `15.160.0.0/16`, `deny`, `/root`, `C:\\process.exe`, `/macpath`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
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
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func Test_PostureCheck_Update(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_posture_check." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().PostureChecks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPostureCheckResource(rName, `posture_check`, `0.40.0`, `15`, `10`, `12`, `6.8.0`, `2531`, `EG`, `Cairo`, `allow`, `15.160.0.0/16`, `deny`, `/root`, `C:\\process.exe`, `/macpath`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
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

// Test_PostureCheck_CertificateCheck walks the block through its lifecycle next to
// another check type: create, add and reorder CAs, import, and remove the block.
func Test_PostureCheck_CertificateCheck(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_posture_check." + rName
	caA, caB := testCAPEM(t, "acceptance CA A"), testCAPEM(t, "acceptance CA B")
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().PostureChecks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: testPostureCheckCertificateResource(rName, caA),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(address, &createdID),
					resource.TestCheckResourceAttr(address, "netbird_version_check.min_version", "0.40.0"),
					resource.TestCheckResourceAttr(address, "certificate_check.ca_certificates.#", "1"),
					// Byte for byte, trailing newline included, as file() reads it.
					resource.TestCheckResourceAttr(address, "certificate_check.ca_certificates.0", caA),
					testPostureCheckServerCAs(address, caA),
				),
			},
			{
				// B first: the configured order must survive the read.
				Config:           testPostureCheckCertificateResource(rName, caB, caA),
				ConfigPlanChecks: updatesInPlace(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(address, "id", &createdID),
					resource.TestCheckResourceAttr(address, "certificate_check.ca_certificates.#", "2"),
					resource.TestCheckResourceAttr(address, "certificate_check.ca_certificates.0", caB),
					resource.TestCheckResourceAttr(address, "certificate_check.ca_certificates.1", caA),
					testPostureCheckServerCAs(address, caA, caB),
				),
			},
			{
				ResourceName:      address,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:           testPostureCheckCertificateResource(rName),
				ConfigPlanChecks: updatesInPlace(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(address, "id", &createdID),
					resource.TestCheckNoResourceAttr(address, "certificate_check.ca_certificates.#"),
					resource.TestCheckResourceAttr(address, "netbird_version_check.min_version", "0.40.0"),
					testPostureCheckServerCAs(address),
				),
			},
		},
	})
}

// Test_PostureCheck_CertificateCheck_ImportExisting is the migration path for a
// check created in the dashboard: import it, and the configuration that
// describes it must plan no changes.
func Test_PostureCheck_CertificateCheck_ImportExisting(t *testing.T) {
	testE2E(t)
	testEnsureManagementRunning(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_posture_check." + rName
	ca := testCAPEM(t, "dashboard CA")

	existing, err := testClient().PostureChecks.Create(context.Background(), api.PostureCheckUpdate{
		Name: rName,
		Checks: &api.Checks{
			NbVersionCheck:   &api.MinVersionCheck{MinVersion: "0.40.0"},
			CertificateCheck: &api.CertificateCheck{CaCertificates: []string{ca}},
		},
	})
	if err != nil {
		t.Fatalf("create posture check through the API: %v", err)
	}
	createdID := existing.Id
	// The import step never runs if the case fails early; do not leak the check.
	t.Cleanup(func() { _ = testClient().PostureChecks.Delete(context.Background(), createdID) })

	cfg := testPostureCheckCertificateResource(rName, ca)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().PostureChecks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config:             cfg,
				ResourceName:       address,
				ImportState:        true,
				ImportStateId:      createdID,
				ImportStatePersist: true,
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "id", createdID),
					resource.TestCheckResourceAttr(address, "certificate_check.ca_certificates.0", ca),
				),
			},
		},
	})
}

// testPostureCheckCertificateResource renders a posture check with a NetBird
// version check and, when CAs are given, a certificate_check in that order.
func testPostureCheckCertificateResource(rName string, caCertificates ...string) string {
	certificateCheck := ""
	if len(caCertificates) > 0 {
		quoted := make([]string, 0, len(caCertificates))
		for _, ca := range caCertificates {
			quoted = append(quoted, fmt.Sprintf("%q", ca))
		}
		certificateCheck = fmt.Sprintf(`
  certificate_check {
    ca_certificates = [%s]
  }
`, strings.Join(quoted, ", "))
	}
	return fmt.Sprintf(`resource "netbird_posture_check" "%[1]s" {
  name = "%[1]s"

  netbird_version_check {
    min_version = "0.40.0"
  }
%[2]s}
`, rName, certificateCheck)
}

// testPostureCheckServerCAs asserts the CAs the server holds for the posture
// check at address, in any order. No CAs means the server holds no
// certificate check at all.
func testPostureCheckServerCAs(address string, want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[address]
		if !ok {
			return fmt.Errorf("%s not in state", address)
		}
		check, err := testClient().PostureChecks.Get(context.Background(), rs.Primary.ID)
		if err != nil {
			return err
		}
		if len(want) == 0 {
			if check.Checks.CertificateCheck != nil {
				return fmt.Errorf("server still holds a certificate check with %d CAs", len(check.Checks.CertificateCheck.CaCertificates))
			}
			return nil
		}
		if check.Checks.CertificateCheck == nil {
			return fmt.Errorf("server holds no certificate check, want %d CAs", len(want))
		}
		got := slices.Clone(check.Checks.CertificateCheck.CaCertificates)
		want = slices.Clone(want)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			return fmt.Errorf("server holds %d CAs that differ from the %d configured", len(got), len(want))
		}
		return nil
	}
}

// Test_PostureCheck_WithoutDescription covers the most common configuration:
// no description at all. The server stores it as "", which must not surface as
// an inconsistent result on create or as a diff afterwards.
func Test_PostureCheck_WithoutDescription(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_posture_check." + rName
	cfg := fmt.Sprintf(`resource "netbird_posture_check" "%[1]s" {
  name = "%[1]s"

  netbird_version_check {
    min_version = "0.40.0"
  }
}`, rName)
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().PostureChecks.Get, &createdID),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(address, &createdID),
					resource.TestCheckNoResourceAttr(address, "description"),
				),
			},
			{
				ResourceName:      address,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
