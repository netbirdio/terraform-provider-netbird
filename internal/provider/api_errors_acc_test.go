// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// The provider's failure paths, driven by real management responses rather than a
// stub: input the API rejects, and objects deleted behind Terraform's back. These
// are the diagnostics a user actually sees, so the message management produced has
// to reach them instead of being swallowed or turned into a panic.
//
// Failures that only a broken transport can produce — connection resets, truncated
// bodies, malformed JSON — are not reachable this way and belong in unit tests
// against a stub server.

// Deleting an object outside Terraform must show up as drift on the next refresh,
// not as a hard error: the provider is expected to drop it from state so the next
// apply recreates it.
func Test_Group_DeletedOutsideTerraformIsRecreated(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_group." + rName
	var firstID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGroupResource(rName, "[]"),
				Check: func(s *terraform.State) error {
					firstID = s.RootModule().Resources[addr].Primary.Attributes["id"]
					return nil
				},
			},
			{
				// Delete it behind Terraform's back, then let the framework
				// refresh: the group is gone, so the plan must want it back.
				PreConfig: func() {
					if err := testClient().Groups.Delete(context.Background(), firstID); err != nil {
						t.Fatalf("deleting the group out of band: %v", err)
					}
				},
				Config:             testGroupResource(rName, "[]"),
				RefreshState:       false,
				ExpectNonEmptyPlan: true,
				PlanOnly:           true,
			},
			{
				// And applying really does recreate it, under a new ID.
				Config: testGroupResource(rName, "[]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						if id == firstID {
							return fmt.Errorf("group kept the deleted ID %s", firstID)
						}
						if _, err := testClient().Groups.Get(context.Background(), id); err != nil {
							return fmt.Errorf("recreated group not on the management server: %w", err)
						}
						return nil
					},
				),
			},
		},
	})
}

// Same for a resource the provider reads through a nested path, where a
// not-found has to be recognised on the parent's collection rather than a
// top-level endpoint.
func Test_NetworkResource_DeletedOutsideTerraformIsRecreated(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_network_resource." + rName
	var networkID, resourceID string

	config := fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_resource" %[1]q {
  network_id = netbird_network.%[1]s.id
  name       = %[1]q
  address    = "drift.example.com"
  groups     = [%[2]q]
}`, rName, e2eGroupNotAllID())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(s *terraform.State) error {
					attrs := s.RootModule().Resources[addr].Primary.Attributes
					networkID, resourceID = attrs["network_id"], attrs["id"]
					return nil
				},
			},
			{
				PreConfig: func() {
					if err := testClient().Networks.Resources(networkID).Delete(context.Background(), resourceID); err != nil {
						t.Fatalf("deleting the network resource out of band: %v", err)
					}
				},
				Config:             config,
				ExpectNonEmptyPlan: true,
				PlanOnly:           true,
			},
			{
				// Recreate it, both to prove the drift is repairable and to leave
				// the state consistent for the framework's own destroy.
				Config: config,
				Check: func(s *terraform.State) error {
					attrs := s.RootModule().Resources[addr].Primary.Attributes
					if attrs["id"] == resourceID {
						return fmt.Errorf("network resource kept the deleted ID %s", resourceID)
					}
					_, err := testClient().Networks.Resources(attrs["network_id"]).Get(context.Background(), attrs["id"])
					return err
				},
			},
		},
	})
}

// Input the API refuses. Each case asserts the provider surfaces management's own
// complaint, so a user can act on it — the alternative, a generic "request
// failed", is what these guard against.
func Test_APIRejectedInput_SurfacesManagementDiagnostics(t *testing.T) {
	testE2E(t)

	for _, tc := range []struct {
		name   string
		config func(rName string) string
		expect *regexp.Regexp
	}{
		{
			name: "network resource with an unparseable address",
			config: func(rName string) string {
				return fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_resource" %[1]q {
  network_id = netbird_network.%[1]s.id
  name       = %[1]q
  address    = "not a host, domain or cidr"
  groups     = [%[2]q]
}`, rName, e2eGroupNotAllID())
			},
			expect: regexp.MustCompile(`(?s)Error creating networkResource`),
		},
		{
			name: "route with a network that is not a prefix",
			config: func(rName string) string {
				return fmt.Sprintf(`
resource "netbird_route" %[1]q {
  network_id = %[1]q
  network    = "999.999.999.999/99"
  groups     = [%[2]q]
  peer_groups = [%[3]q]
}`, rName, e2eGroupAllID(), e2eGroupNotAllID())
			},
			expect: regexp.MustCompile(`(?s)Error creating Route|invalid`),
		},
		{
			name: "policy referring to a group that does not exist",
			config: func(rName string) string {
				return fmt.Sprintf(`
resource "netbird_policy" %[1]q {
  name    = %[1]q
  enabled = true

  rule {
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    name          = %[1]q
    sources       = ["group-that-does-not-exist"]
    destinations  = [%[2]q]
    ports         = ["443"]
  }
}`, rName, e2eGroupAllID())
			},
			// Management accepts the unknown group and silently drops it from the
			// rule, so the apply fails with a consistency error rather than a
			// rejection. Either way the user is told; what must not happen is a
			// silent success with a rule that references nothing.
			expect: regexp.MustCompile(`(?s)inconsistent result after apply|Error creating Policy|not found`),
		},
		{
			name: "nameserver group with a nameserver that is not an ip",
			config: func(rName string) string {
				return fmt.Sprintf(`
resource "netbird_nameserver_group" %[1]q {
  name = %[1]q
  nameservers = [
    {
      ip      = "not-an-ip"
      ns_type = "udp"
      port    = 53
    }
  ]
  groups = [%[2]q]
}`, rName, e2eGroupAllID())
			},
			expect: regexp.MustCompile(`(?s)Error creating NameserverGroup|invalid`),
		},
		{
			name: "group name already taken",
			config: func(rName string) string {
				// Management rejects a duplicate group name with 409; the "All"
				// group exists on every account.
				return fmt.Sprintf(`
resource "netbird_group" %[1]q {
  name  = %[2]q
  peers = []
}`, rName, e2eGroupAll)
			},
			expect: regexp.MustCompile(`(?s)Error creating Group|already exists|conflict`),
		},
		{
			name: "group listing a peer that does not exist",
			config: func(rName string) string {
				return fmt.Sprintf(`
resource "netbird_group" %[1]q {
  name  = %[1]q
  peers = ["peer-that-does-not-exist"]
}`, rName)
			},
			// As above: the unknown peer is dropped by management rather than
			// rejected, and the apply fails on the resulting inconsistency.
			expect: regexp.MustCompile(`(?s)inconsistent result after apply|Error creating Group|not found`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rName := "err" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testEnsureManagementRunning(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      tc.config(rName),
						ExpectError: tc.expect,
					},
				},
			})
		})
	}
}

// An update the API refuses has to fail the apply and leave the object as it was,
// rather than half-writing it or dropping it from state.
func Test_APIRejectedUpdate_LeavesTheObjectIntact(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_network_resource." + rName

	config := func(address string) string {
		return fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_resource" %[1]q {
  network_id = netbird_network.%[1]s.id
  name       = %[1]q
  address    = %[2]q
  groups     = [%[3]q]
}`, rName, address, e2eGroupNotAllID())
	}

	var networkID, resourceID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("valid.example.com"),
				Check: func(s *terraform.State) error {
					attrs := s.RootModule().Resources[addr].Primary.Attributes
					networkID, resourceID = attrs["network_id"], attrs["id"]
					return nil
				},
			},
			{
				Config:      config("still not a valid address"),
				ExpectError: regexp.MustCompile(`(?s)Error updating NetworkResource|invalid`),
			},
			{
				// The rejected update must not have changed anything.
				Config: config("valid.example.com"),
				Check: func(s *terraform.State) error {
					r, err := testClient().Networks.Resources(networkID).Get(context.Background(), resourceID)
					if err != nil {
						return fmt.Errorf("the resource is gone after a rejected update: %w", err)
					}
					if r.Address != "valid.example.com" {
						return fmt.Errorf("address changed despite the update being rejected, management reports %q", r.Address)
					}
					return nil
				},
			},
		},
	})
}

// Reading a data source that matches nothing is an error, not an empty result —
// otherwise a typo silently yields a resource wired to nothing.
func Test_DataSources_NoMatchIsAnError(t *testing.T) {
	testE2E(t)

	for _, tc := range []struct {
		name   string
		config string
		expect *regexp.Regexp
	}{
		{
			name:   "group",
			config: `data "netbird_group" "missing" { name = "no-such-group-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "network",
			config: `data "netbird_network" "missing" { name = "no-such-network-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "peer",
			config: `data "netbird_peer" "missing" { name = "no-such-peer-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "policy",
			config: `data "netbird_policy" "missing" { name = "no-such-policy-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "setup key",
			config: `data "netbird_setup_key" "missing" { name = "no-such-key-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "posture check",
			config: `data "netbird_posture_check" "missing" { name = "no-such-check-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "nameserver group",
			config: `data "netbird_nameserver_group" "missing" { name = "no-such-nsgroup-anywhere" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
		{
			name:   "user",
			config: `data "netbird_user" "missing" { email = "nobody@netbird.invalid" }`,
			expect: regexp.MustCompile(`(?s)No match|not found`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testEnsureManagementRunning(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: tc.config, ExpectError: tc.expect},
				},
			})
		})
	}
}

// A data source given no filter at all cannot pick a record, and says so rather
// than returning an arbitrary one.
func Test_DataSources_MissingSelectorIsAnError(t *testing.T) {
	testE2E(t)

	for _, tc := range []struct{ name, config string }{
		{name: "group", config: `data "netbird_group" "nofilter" {}`},
		{name: "network", config: `data "netbird_network" "nofilter" {}`},
		{name: "peer", config: `data "netbird_peer" "nofilter" {}`},
		{name: "policy", config: `data "netbird_policy" "nofilter" {}`},
		{name: "posture check", config: `data "netbird_posture_check" "nofilter" {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testEnsureManagementRunning(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: tc.config, ExpectError: regexp.MustCompile(`(?s)No selector|Must (add|set) at least one`)},
				},
			})
		})
	}
}

// A filter matching more than one record is ambiguous, and the provider must
// refuse rather than silently pick one. Management allows duplicate network
// names, which makes this reachable.
func Test_Network_DataSource_MultipleMatchesIsAnError(t *testing.T) {
	testE2E(t)
	ctx := context.Background()
	shared := "dup" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)

	var ids []string
	for i := 0; i < 2; i++ {
		n, err := testClient().Networks.Create(ctx, api.PostApiNetworksJSONRequestBody{Name: shared})
		if err != nil {
			t.Fatalf("creating fixture network %d: %v", i, err)
		}
		ids = append(ids, n.Id)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			_ = testClient().Networks.Delete(ctx, id)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(`data "netbird_network" "dup" { name = %q }`, shared),
				ExpectError: regexp.MustCompile(`(?s)Multiple Matches|cannot match multiple`),
			},
		},
	})
}
