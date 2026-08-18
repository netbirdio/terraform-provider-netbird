//go:build e2e

// Replacement, attribute by attribute.
//
// Eighteen attributes across eight resources are marked RequiresReplace, which
// is a promise: change one and the object is destroyed and a new one takes its
// place. lifecycle_acc_test.go checks three of them. The rest are checked here,
// and the check is in two parts — the plan has to say it is replacing, and the
// server-assigned ID afterwards has to differ from the one before.
//
// Both halves matter. A provider that updates in place where the schema says
// replace looks identical in an apply, and the difference only shows up later,
// as an object that kept an attribute the API cannot actually change. A
// provider that replaces where it should update destroys infrastructure for no
// reason, which is the direction that costs a setup key or a token.

package provider

import (
	"context"
	"fmt"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// replaces asserts the plan destroys and recreates the resource rather than
// updating it. Terraform expresses that as two actions, in either order
// depending on whether the provider allows the new object to exist first.
type replaces struct {
	address string
}

func (c replaces) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address != c.address {
			continue
		}
		a := rc.Change.Actions
		if len(a) == 2 && ((a[0] == tfjson.ActionDelete && a[1] == tfjson.ActionCreate) ||
			(a[0] == tfjson.ActionCreate && a[1] == tfjson.ActionDelete)) {
			return
		}
		resp.Error = fmt.Errorf("%s: planned %v, which is not a replacement; the schema marks the attribute under test RequiresReplace",
			c.address, rc.Change.Actions)
		return
	}
	resp.Error = fmt.Errorf("%s is not in the plan", c.address)
}

func replacesResource(address string) resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{replaces{address: address}}}
}

// replaceCase applies one configuration, then a second that differs in exactly
// one replace-forcing attribute.
func replaceCase(t *testing.T, address, before, after string, then ...resource.TestCheckFunc) {
	t.Helper()
	var first string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: before,
				Check:  testRecordID(address, &first),
			},
			{
				Config:           after,
				ConfigPlanChecks: replacesResource(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					append([]resource.TestCheckFunc{testIDChanged(address, &first)}, then...)...),
			},
		},
	})
}

// Setup keys carry five of the eighteen, more than any other resource, and each
// one is a secret that a needless replacement invalidates.

// setupKeyConfig builds a reusable key. Reusable rather than one-off because a
// one-off key's usage limit is the server's to decide, and this file needs every
// attribute under test to be the configuration's.
func setupKeyConfig(rName, name, expiry, ephemeral, extraLabels, usageLimit string) string {
	return fmt.Sprintf(`resource "netbird_setup_key" "%[1]s" {
  name                   = "%[2]s"
  expiry_seconds         = %[3]s
  type                   = "reusable"
  ephemeral              = %[4]s
  allow_extra_dns_labels = %[5]s
  usage_limit            = %[6]s
  auto_groups            = []
}
`, rName, name, expiry, ephemeral, extraLabels, usageLimit)
}

func Test_Replace_SetupKeyName(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	replaceCase(t, address,
		setupKeyConfig(rName, rName, "1800", "false", "false", "5"),
		setupKeyConfig(rName, rName+"-renamed", "1800", "false", "false", "5"),
		resource.TestCheckResourceAttr(address, "name", rName+"-renamed"),
	)
}

func Test_Replace_SetupKeyExpiry(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	// expiry_seconds forces replacement through a provider-specific rule rather
	// than the framework's, because the API cannot report the value and an
	// imported key has to be able to adopt it. The rule still has to replace
	// when both values are known and differ.
	replaceCase(t, address,
		setupKeyConfig(rName, rName, "1800", "false", "false", "5"),
		setupKeyConfig(rName, rName, "3600", "false", "false", "5"),
		resource.TestCheckResourceAttr(address, "expiry_seconds", "3600"),
	)
}

func Test_Replace_SetupKeyUsageLimit(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	replaceCase(t, address,
		setupKeyConfig(rName, rName, "1800", "false", "false", "5"),
		setupKeyConfig(rName, rName, "1800", "false", "false", "10"),
		resource.TestCheckResourceAttr(address, "usage_limit", "10"),
	)
}

func Test_Replace_SetupKeyEphemeral(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	replaceCase(t, address,
		setupKeyConfig(rName, rName, "1800", "false", "false", "5"),
		setupKeyConfig(rName, rName, "1800", "true", "false", "5"),
		resource.TestCheckResourceAttr(address, "ephemeral", "true"),
	)
}

func Test_Replace_SetupKeyExtraDNSLabels(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	replaceCase(t, address,
		setupKeyConfig(rName, rName, "1800", "false", "false", "5"),
		setupKeyConfig(rName, rName, "1800", "false", "true", "5"),
		resource.TestCheckResourceAttr(address, "allow_extra_dns_labels", "true"),
	)
}

func Test_Replace_TokenName(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_token." + rName
	userID := mustE2E().UserID
	cfg := func(name string) string {
		return fmt.Sprintf(`resource "netbird_token" "%[1]s" {
  name            = "%[2]s"
  user_id         = %[3]q
  expiration_days = 90
}
`, rName, name, userID)
	}
	replaceCase(t, address, cfg(rName), cfg(rName+"-renamed"),
		resource.TestCheckResourceAttr(address, "name", rName+"-renamed"),
	)
}

// Test_Replace_TokenUser moves a token from one service user to another. The
// token belongs to the user, so this is a different object by definition, and
// the second user has to exist before the token can point at it.
func Test_Replace_TokenUser(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_token." + rName
	cfg := func(owner string) string {
		return fmt.Sprintf(`
resource "netbird_user" "%[1]sa" {
  name            = "%[1]sa"
  is_service_user = true
  role            = "user"
  auto_groups     = []
}

resource "netbird_user" "%[1]sb" {
  name            = "%[1]sb"
  is_service_user = true
  role            = "user"
  auto_groups     = []
}

resource "netbird_token" "%[1]s" {
  name            = "%[1]s"
  user_id         = netbird_user.%[2]s.id
  expiration_days = 90
}
`, rName, owner)
	}
	replaceCase(t, address, cfg(rName+"a"), cfg(rName+"b"),
		resource.TestCheckResourceAttrPair(address, "user_id", "netbird_user."+rName+"b", "id"),
	)
}

// Test_Replace_DNSRecordZone moves a record between zones. A record is
// addressed as zone plus record, so it cannot be the same object afterwards.
//
// The name moves with it: a record is named by its fully qualified name and the
// server refuses one that does not sit inside its zone's domain, so the two
// cannot be varied independently. zone_id is what forces the replacement either
// way, since name does not.
func Test_Replace_DNSRecordZone(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_dns_record." + rName
	cfg := func(zone string) string {
		return fmt.Sprintf(`
resource "netbird_dns_zone" "%[1]sa" {
  name                = "%[1]sa"
  domain              = "%[1]sa.local"
  distribution_groups = [%[3]q]
}

resource "netbird_dns_zone" "%[1]sb" {
  name                = "%[1]sb"
  domain              = "%[1]sb.local"
  distribution_groups = [%[3]q]
}

resource "netbird_dns_record" "%[1]s" {
  zone_id = netbird_dns_zone.%[2]s.id
  name    = "www.%[2]s.local"
  type    = "A"
  content = "10.0.0.1"
  ttl     = 300
}
`, rName, zone, e2eGroupNotAllID())
	}
	replaceCase(t, address, cfg(rName+"a"), cfg(rName+"b"),
		resource.TestCheckResourceAttrPair(address, "zone_id", "netbird_dns_zone."+rName+"b", "id"),
	)
}

// Test_Replace_NetworkResourceNetwork and Test_Replace_NetworkRouterNetwork
// both move a child between networks, which the API models as a new object
// under a different parent rather than a move.

func Test_Replace_NetworkResourceNetwork(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_network_resource." + rName
	cfg := func(network string) string {
		return fmt.Sprintf(`
resource "netbird_network" "%[1]sa" {
  name = "%[1]sa"
}

resource "netbird_network" "%[1]sb" {
  name = "%[1]sb"
}

resource "netbird_network_resource" "%[1]s" {
  network_id = netbird_network.%[2]s.id
  name       = "%[1]s"
  address    = "example.com"
  groups     = [%[3]q]
}
`, rName, network, e2eGroupNotAllID())
	}
	replaceCase(t, address, cfg(rName+"a"), cfg(rName+"b"),
		resource.TestCheckResourceAttrPair(address, "network_id", "netbird_network."+rName+"b", "id"),
	)
}

func Test_Replace_NetworkRouterNetwork(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_network_router." + rName
	cfg := func(network string) string {
		return fmt.Sprintf(`
resource "netbird_network" "%[1]sa" {
  name = "%[1]sa"
}

resource "netbird_network" "%[1]sb" {
  name = "%[1]sb"
}

resource "netbird_network_router" "%[1]s" {
  network_id  = netbird_network.%[2]s.id
  peer_groups = [%[3]q]
  masquerade  = true
  metric      = 9999
}
`, rName, network, e2eGroupNotAllID())
	}
	replaceCase(t, address, cfg(rName+"a"), cfg(rName+"b"),
		resource.TestCheckResourceAttrPair(address, "network_id", "netbird_network."+rName+"b", "id"),
	)
}
