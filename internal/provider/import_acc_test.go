// Copyright (c) HashiCorp, Inc.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Import is the one code path a create/update test never exercises: Terraform
// throws the state away and asks the provider to rebuild it from an ID alone.
// Each test below creates a resource, re-imports it, and asserts the rebuilt
// state matches attribute for attribute — which also pins the documented import
// ID format for the resources that use a composite one.

// importStateIDPair builds "<first>/<second>" from two attributes of a resource,
// the format the composite-ID importers expect.
func importStateIDPair(addr, first, second string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return "", fmt.Errorf("not found: %s", addr)
		}
		a, b := rs.Primary.Attributes[first], rs.Primary.Attributes[second]
		if a == "" || b == "" {
			return "", fmt.Errorf("%s: %s=%q %s=%q, both must be set", addr, first, a, second, b)
		}
		return a + "/" + b, nil
	}
}

func Test_Group_Import(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_group." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testGroupResource(rName, "[]")},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_Network_Import(t *testing.T) {
	testE2E(t)
	rName := "n" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_network." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`resource "netbird_network" %[1]q {
  name        = %[1]q
  description = "import"
}`, rName)},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_NetworkResource_Import(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_network_resource." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_resource" %[1]q {
  network_id = netbird_network.%[1]s.id
  name       = %[1]q
  address    = "import.example.com"
  groups     = [%[2]q]
}`, rName, e2eGroupNotAllID())},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDPair(addr, "network_id", "id"),
			},
		},
	})
}

func Test_NetworkRouter_Import(t *testing.T) {
	testE2E(t)
	rName := "nro" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_network_router." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_router" %[1]q {
  network_id  = netbird_network.%[1]s.id
  peer_groups = [%[2]q]
  metric      = 9999
  masquerade  = true
  enabled     = true
}`, rName, e2eGroupNotAllID())},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDPair(addr, "network_id", "id"),
			},
		},
	})
}

func Test_Policy_Import(t *testing.T) {
	testE2E(t)
	rName := "pol" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_policy." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testPolicyResourceGroups(rName, rName, "import", "accept", "tcp", e2eGroupAllID(), e2eGroupNotAllID(), "443")},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_PostureCheck_Import(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_posture_check." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`resource "netbird_posture_check" %[1]q {
  name        = %[1]q
  description = "import"

  netbird_version_check {
    min_version = "0.35.0"
  }
}`, rName)},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_Route_Import(t *testing.T) {
	testE2E(t)
	rName := "rt" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_route." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testRouteResource(rName, e2eGroupAllID(), `null`, `import`, `null`,
				`["import.example.com"]`, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `null`)},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_SetupKey_Import(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_setup_key." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testSetupKeyResource(rName, `1800`, `one-off`, `true`,
				fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `false`, `1`)},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				// The plaintext key is returned once, at create; an import can
				// only ever see the stored metadata. Same for the requested
				// lifetime, which the API reports as an absolute expiry.
				ImportStateVerifyIgnore: []string{"key", "expiry_seconds"},
			},
		},
	})
}

func Test_Token_Import(t *testing.T) {
	env := testE2E(t)
	rName := "tok" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_token." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testTokenResource(rName, env.UserID, `180`)},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importStateIDPair(addr, "user_id", "id"),
				// The token itself is only returned at create, and the requested
				// lifetime is stored as an absolute expiration date.
				ImportStateVerifyIgnore: []string{"token", "expiration_days"},
			},
		},
	})
}

func Test_User_Import(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_user." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `user`)},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_NameserverGroup_Import(t *testing.T) {
	testE2E(t)
	rName := "ns" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_nameserver_group." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID()))},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_AccountSettings_Import(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_account_settings." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAccountResource(rName)},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     env.AccountID,
			},
		},
	})
}

func Test_DNSSettings_Import(t *testing.T) {
	testE2E(t)
	rName := "dns" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_dns_settings." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testDNSSettingsResource(rName, fmt.Sprintf("[%q]", e2eGroupAllID()))},
			{
				ResourceName: addr,
				ImportState:  true,
				// Account-wide singleton: the resource has no "id" attribute, so
				// the import ID is ignored and ImportStateVerify has nothing to
				// match instances on. Check the imported attributes directly.
				ImportStateId: "dns_settings",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 imported instance, got %d", len(states))
					}
					attrs := states[0].Attributes
					if attrs["disabled_management_groups.#"] != "1" {
						return fmt.Errorf("expected 1 disabled management group, got %q", attrs["disabled_management_groups.#"])
					}
					if got := attrs["disabled_management_groups.0"]; got != e2eGroupAllID() {
						return fmt.Errorf("expected group %s, got %s", e2eGroupAllID(), got)
					}
					return nil
				},
			},
		},
	})
}

func Test_AgentNetworkProvider_Import(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_agent_network_provider." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testAgentNetworkProviderResource(rName, rName, `{ "x-portkey-config" = "pc-import" }`, "false")},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
				// Write-only on the API, and only meaningful at create time.
				ImportStateVerifyIgnore: []string{"api_key", "bootstrap_cluster"},
			},
		},
	})
}

func Test_AgentNetworkGuardrail_Import(t *testing.T) {
	testE2E(t)
	rName := "ang" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_agent_network_guardrail." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`resource "netbird_agent_network_guardrail" %[1]q {
  name        = %[1]q
  description = "import"
  model_allowlist = {
    enabled = true
    models  = ["gpt-4.1"]
  }
  prompt_capture = {
    enabled    = true
    redact_pii = true
  }
}`, rName)},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_AgentNetworkPolicy_Import(t *testing.T) {
	testE2E(t)
	rName := "ann" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_agent_network_policy." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(`
resource "netbird_group" %[1]q {
  name = "%[1]s-group"
}

resource "netbird_agent_network_provider" %[1]q {
  provider_id       = "openai_api"
  name              = "%[1]s-provider"
  upstream_url      = "https://api.openai.com"
  api_key           = "sk-acc-test"
  bootstrap_cluster = %[2]q
}

resource "netbird_agent_network_guardrail" %[1]q {
  name = "%[1]s-guardrail"
  model_allowlist = {
    enabled = true
    models  = ["gpt-4.1"]
  }
  prompt_capture = {
    enabled = false
  }
}

resource "netbird_agent_network_policy" %[1]q {
  name                     = %[1]q
  source_groups            = [netbird_group.%[1]s.id]
  destination_provider_ids = [netbird_agent_network_provider.%[1]s.id]
  guardrail_ids            = [netbird_agent_network_guardrail.%[1]s.id]

  token_limit = {
    enabled        = true
    group_cap      = 1000000
    window_seconds = 86400
  }
}`, rName, testBootstrapCluster)},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}

func Test_Peer_Import(t *testing.T) {
	testE2E(t)
	peerID := testPeerID(t, "peer1")
	rName := "p" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_peer." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: testPeerResource(rName, peerID, "peer1")},
			{ResourceName: addr, ImportState: true, ImportStateVerify: true},
		},
	})
}
