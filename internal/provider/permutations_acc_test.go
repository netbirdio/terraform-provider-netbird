// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Configuration shapes the CRUD tests never produced: optional attributes left
// out so the provider takes its null branches, and mutable fields changed one at
// a time so each conditional in an Update request builder is exercised. Each
// assertion reads the object back from management.

// The peer Update builds its request field by field, sending only what changed.
// Flipping the fields management lets a setup-key peer change covers those
// conditionals, and the API has to come back with them applied.
func Test_Peer_UpdateMutableFields(t *testing.T) {
	testE2E(t)
	peerID := testPeerID(t, "peer3")

	before, err := testClient().Peers.Get(context.Background(), peerID)
	if err != nil {
		t.Fatalf("read the peer: %v", err)
	}

	rName := "p" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_peer." + rName

	// Every flag flipped away from its current value, so no conditional is a
	// no-op, plus a rename.
	// Only name and ssh_enabled are flipped. Management gates the other two on how
	// the peer joined: both login_expiration_enabled and
	// inactivity_expiration_enabled are refused with "this peer hasn't been added
	// with the sso login", and these agents register with a setup key. Covering
	// those two conditionals needs an SSO-enrolled peer.
	config := func(name string, ssh bool) string {
		return fmt.Sprintf(`
resource "netbird_peer" %[1]q {
  id          = %[2]q
  name        = %[3]q
  ssh_enabled = %[4]t
}`, rName, peerID, name, ssh)
	}

	flipped := config(rName+"-renamed", !before.SshEnabled)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Start from the peer's current values, so this apply changes
				// nothing and the update conditionals all take their false arm.
				Config: config(before.Name, before.SshEnabled),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "id", peerID),
					resource.TestCheckResourceAttr(addr, "ssh_enabled", fmt.Sprint(before.SshEnabled)),
				),
			},
			{
				Config: flipped,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", rName+"-renamed"),
					resource.TestCheckResourceAttr(addr, "ssh_enabled", fmt.Sprint(!before.SshEnabled)),
					func(s *terraform.State) error {
						after, err := testClient().Peers.Get(context.Background(), peerID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":        {rName + "-renamed", after.Name},
							"ssh_enabled": {!before.SshEnabled, after.SshEnabled},
						})
					},
				),
			},
		},
	})
}

// A posture check with only the mandatory pieces: no description, and a single
// check. The provider has to leave the attributes it was not given null rather
// than inventing empty objects.
func Test_PostureCheck_OptionalAttributesLeftNull(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_posture_check." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// No description and no netbird_version_check; only a process
				// check, and only its Linux path — the empty mac and windows
				// paths must be dropped rather than sent as "".
				Config: fmt.Sprintf(`
resource "netbird_posture_check" %[1]q {
  name = %[1]q

  process_check {
    linux_path = "/usr/bin/netbird"
  }
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(addr, "description"),
					resource.TestCheckNoResourceAttr(addr, "netbird_version_check.min_version"),
					resource.TestCheckResourceAttr(addr, "process_check.#", "1"),
					resource.TestCheckResourceAttr(addr, "process_check.0.linux_path", "/usr/bin/netbird"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						pc, err := testClient().PostureChecks.Get(context.Background(), id)
						if err != nil {
							return err
						}
						if pc.Checks.NbVersionCheck != nil {
							return fmt.Errorf("management stored a version check that was never configured: %#v", pc.Checks.NbVersionCheck)
						}
						if pc.Checks.ProcessCheck == nil || len(pc.Checks.ProcessCheck.Processes) != 1 {
							return fmt.Errorf("process check not persisted: %#v", pc.Checks.ProcessCheck)
						}
						proc := pc.Checks.ProcessCheck.Processes[0]
						if valOr(proc.LinuxPath, "") != "/usr/bin/netbird" {
							return fmt.Errorf("linux_path not persisted, management reports %v", proc.LinuxPath)
						}
						// The unset mac and windows paths come back as "" from
						// management, which is indistinguishable from the provider
						// having sent "" — so there is nothing to assert about them
						// here. What matters is that the round trip is stable, and
						// the framework enforces that by failing on a non-empty plan
						// after this apply.
						return nil
					},
				),
			},
		},
	})
}

// A policy created without a description: management stores the empty string,
// and the provider must surface that as null rather than "" so the config and
// the state agree.
func Test_Policy_OmittedDescriptionStaysNull(t *testing.T) {
	testE2E(t)
	rName := "pol" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_policy." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_policy" %[1]q {
  name    = %[1]q
  enabled = true

  rule {
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    name          = %[1]q
    sources       = [%[2]q]
    destinations  = [%[3]q]
    ports         = ["443"]
  }
}`, rName, e2eGroupAllID(), e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(addr, "description"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						pol, err := testClient().Policies.Get(context.Background(), id)
						if err != nil {
							return err
						}
						if d := valOr(pol.Description, ""); d != "" {
							return fmt.Errorf("expected no description on the management server, found %q", d)
						}
						return nil
					},
				),
			},
		},
	})
}

// An agent-network provider carrying an explicit model list, which is a separate
// request-building branch from the default catalogue.
func Test_AgentNetworkProvider_ExplicitModels(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_agent_network_provider." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_agent_network_provider" %[1]q {
  provider_id       = "openai_api"
  name              = %[1]q
  upstream_url      = "https://api.openai.com"
  api_key           = "sk-acc-test"

  models = [
    {
      id            = "gpt-4.1"
      input_per_1k  = 0.01
      output_per_1k = 0.03
    },
  ]
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "models.#", "1"),
					resource.TestCheckResourceAttr(addr, "models.0.id", "gpt-4.1"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						p, err := testGetProvider(id)
						if err != nil {
							return err
						}
						if len(p.Models) != 1 {
							return fmt.Errorf("model list not persisted, management reports %v", p.Models)
						}
						if p.Models[0].Id != "gpt-4.1" {
							return fmt.Errorf("model id mismatch, management reports %q", p.Models[0].Id)
						}
						return nil
					},
				),
			},
		},
	})
}
