// Copyright (c) HashiCorp, Inc.

package provider

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Assertions and helpers the acceptance tests share. They talk to whatever
// management server the suite was pointed at rather than starting one, so they
// stay outside the e2e build tag: the tests that call them compile either way,
// and only the deployment behind them is gated.

// The fixture the harness provisions, named rather than identified: IDs are
// assigned by the server at creation time and differ on every bootstrap, while
// the names are what both the harness and the tests can agree on ahead of time.
// They live here rather than with the harness because both halves of the build
// need them.
const (
	e2eAdminEmail    = "admin@netbird.test"
	e2eAdminName     = "E2E Admin"
	e2eAdminPassword = "Netbird-e2e-Passw0rd!" //nolint:gosec // throwaway credential for a disposable test deployment

	e2eGroupAll     = "All"
	e2eGroupNotAll  = "NotAll"
	e2eNetworkName  = "tfaccnetwork"
	e2eResourceHost = "resource-host"
	e2eResourceNet  = "resource-subnet"
	e2eResourceDom  = "resource-domain"
)

// checkAttrsMatchAPI asserts that the given attributes of a resource or data
// source hold exactly what the management API reports. Comparing Terraform state
// against Terraform state only shows the provider is self-consistent; the
// contract that matters is between the provider and the API, so `want` reads the
// object back from management and returns the values it holds.
func checkAttrsMatchAPI(addr string, want func(attrs map[string]string) (map[string]string, error)) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("not found in state: %s", addr)
		}
		expected, err := want(rs.Primary.Attributes)
		if err != nil {
			return fmt.Errorf("reading %s back from the management API: %w", addr, err)
		}
		for attr, wantValue := range expected {
			got := rs.Primary.Attributes[attr]
			// A null list has no "#" entry at all, which is the same zero items
			// as an empty one. See apiListCount for why that is not uniform.
			if strings.HasSuffix(attr, ".#") && got == "" && wantValue == "0" {
				continue
			}
			if got != wantValue {
				return fmt.Errorf("%s: %s is %q in Terraform state but %q on the management server",
					addr, attr, got, wantValue)
			}
		}
		return nil
	}
}

// apiListCount is the number of items the API reported for a collection. A nil
// collection counts as zero: neither management nor the provider is consistent
// about null versus empty, so the count is the part worth asserting.
//
// Management returns "routers": null from GET /api/networks/{id} but
// "routers": [] from LIST /api/networks for the same network, and the provider
// mirrors whichever it read — the resource uses Get, the data source uses List.
// On top of that the provider's own conversions differ: the ones that build a
// list element by element (groups) turn null into an empty list, while the ones
// that hand the raw slice to types.ListValueFrom (networks) keep it null. So
// length() works on some attributes and errors on others. Tests here assert the
// item count and leave the representation alone.
func apiListCount[T any](items []T) string {
	return fmt.Sprint(len(items))
}
