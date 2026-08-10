// Copyright (c) HashiCorp, Inc.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Data sources are read against the live management server, so each test here
// creates the object with the resource, reads it back through the data source in
// the same configuration, and compares the two. Referencing a resource attribute
// in the data source's lookup argument is what orders the read after the create.

func Test_AccountSettings_DataSource(t *testing.T) {
	env := testE2E(t)
	rName := "acc" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_account_settings." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`data "netbird_account_settings" %[1]q {}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dsAddr, "id", env.AccountID),
					resource.TestCheckResourceAttrSet(dsAddr, "peer_login_expiration"),
					resource.TestCheckResourceAttrSet(dsAddr, "peer_login_expiration_enabled"),
				),
			},
		},
	})
}

func Test_DNSSettings_DataSource(t *testing.T) {
	testE2E(t)
	rName := "dns" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_dns_settings." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_dns_settings" %[1]q {
  disabled_management_groups = [%[2]q]
}

data "netbird_dns_settings" %[1]q {
  depends_on = [netbird_dns_settings.%[1]s]
}`, rName, e2eGroupAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dsAddr, "disabled_management_groups.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "disabled_management_groups.0", e2eGroupAllID()),
				),
			},
		},
	})
}

func Test_Group_DataSource(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_group." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_group" %[1]q {
  name  = %[1]q
  peers = []
}

data "netbird_group" %[1]q {
  name = netbird_group.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_group" "%[1]s_by_id" {
  id = netbird_group.%[1]s.id
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_group."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "name", rName),
					resource.TestCheckResourceAttr(dsAddr, "issued", "api"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "name", dsAddr, "name"),
				),
			},
		},
	})
}

func Test_Network_DataSource(t *testing.T) {
	testE2E(t)
	rName := "n" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_network." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name        = %[1]q
  description = "data source lookup"
}

data "netbird_network" %[1]q {
  name = netbird_network.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_network" "%[1]s_by_id" {
  id = netbird_network.%[1]s.id
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_network."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "name", rName),
					resource.TestCheckResourceAttr(dsAddr, "description", "data source lookup"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "name", dsAddr, "name"),
				),
			},
		},
	})
}

func Test_NetworkResource_DataSource(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_network_resource." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_resource" %[1]q {
  network_id = netbird_network.%[1]s.id
  name       = %[1]q
  address    = "datasource.example.com"
  groups     = [%[2]q]
}

data "netbird_network_resource" %[1]q {
  network_id = netbird_network.%[1]s.id
  name       = netbird_network_resource.%[1]s.name
}

data "netbird_network_resource" "%[1]s_by_id" {
  network_id = netbird_network.%[1]s.id
  id         = netbird_network_resource.%[1]s.id
}`, rName, e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_network_resource."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "address", "datasource.example.com"),
					resource.TestCheckResourceAttr(dsAddr, "enabled", "true"),
					resource.TestCheckResourceAttr(dsAddr, "groups.#", "1"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "address", dsAddr, "address"),
				),
			},
		},
	})
}

func Test_NetworkRouter_DataSource(t *testing.T) {
	testE2E(t)
	rName := "nro" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_network_router." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_router" %[1]q {
  network_id  = netbird_network.%[1]s.id
  peer_groups = [%[2]q]
  metric      = 9999
  masquerade  = true
  enabled     = true
}

data "netbird_network_router" %[1]q {
  network_id = netbird_network.%[1]s.id
  id         = netbird_network_router.%[1]s.id
}`, rName, e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_network_router."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "metric", "9999"),
					resource.TestCheckResourceAttr(dsAddr, "masquerade", "true"),
					resource.TestCheckResourceAttr(dsAddr, "peer_groups.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "peer_groups.0", e2eGroupNotAllID()),
				),
			},
		},
	})
}

func Test_Peer_DataSource(t *testing.T) {
	testE2E(t)
	peerID := testPeerID(t, "peer1")
	rName := "p" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_peer." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "netbird_peer" %[1]q {
  id = %[2]q
}`, rName, peerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dsAddr, "id", peerID),
					resource.TestCheckResourceAttr(dsAddr, "hostname", "peer1"),
					resource.TestCheckResourceAttrSet(dsAddr, "ip"),
					resource.TestCheckResourceAttrSet(dsAddr, "dns_label"),
				),
			},
		},
	})
}

func Test_Policy_DataSource(t *testing.T) {
	testE2E(t)
	rName := "pol" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_policy." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_policy" %[1]q {
  name        = %[1]q
  description = "data source lookup"
  enabled     = true

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
}

data "netbird_policy" %[1]q {
  name = netbird_policy.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_policy" "%[1]s_by_id" {
  id = netbird_policy.%[1]s.id
}`, rName, e2eGroupAllID(), e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_policy."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "description", "data source lookup"),
					resource.TestCheckResourceAttr(dsAddr, "rule.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "rule.0.action", "accept"),
					resource.TestCheckResourceAttr(dsAddr, "rule.0.sources.0", e2eGroupAllID()),
					resource.TestCheckResourceAttr(dsAddr, "rule.0.destinations.0", e2eGroupNotAllID()),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "description", dsAddr, "description"),
				),
			},
		},
	})
}

func Test_PostureCheck_DataSource(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_posture_check." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Version checks only: geo_location_check needs the GeoLite
				// database, which is a property of the deployment rather than of
				// the data source under test.
				Config: fmt.Sprintf(`
resource "netbird_posture_check" %[1]q {
  name        = %[1]q
  description = "data source lookup"

  netbird_version_check {
    min_version = "0.35.0"
  }
}

data "netbird_posture_check" %[1]q {
  name = netbird_posture_check.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_posture_check" "%[1]s_by_id" {
  id = netbird_posture_check.%[1]s.id
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_posture_check."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "description", "data source lookup"),
					resource.TestCheckResourceAttr(dsAddr, "netbird_version_check.min_version", "0.35.0"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "description", dsAddr, "description"),
				),
			},
		},
	})
}

func Test_Route_DataSource(t *testing.T) {
	testE2E(t)
	rName := "rt" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_route." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_route" %[1]q {
  network_id  = %[1]q
  description = "data source lookup"
  enabled     = true
  groups      = [%[2]q]
  peer_groups = [%[3]q]
  domains     = ["datasource.example.com"]
  metric      = 9999
  masquerade  = true
  keep_route  = false
}

data "netbird_route" %[1]q {
  id = netbird_route.%[1]s.id
}

# Resolved by network_id instead, which takes the list-and-scan branch.
data "netbird_route" "%[1]s_by_network" {
  network_id = netbird_route.%[1]s.network_id
}`, rName, e2eGroupAllID(), e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_route."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "network_id", rName),
					resource.TestCheckResourceAttr(dsAddr, "description", "data source lookup"),
					resource.TestCheckResourceAttr(dsAddr, "metric", "9999"),
					resource.TestCheckResourceAttr(dsAddr, "domains.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "groups.0", e2eGroupAllID()),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_network", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_network", "description", dsAddr, "description"),
				),
			},
		},
	})
}

func Test_SetupKey_DataSource(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_setup_key." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_setup_key" %[1]q {
  name                   = %[1]q
  expiry_seconds         = 3600
  type                   = "reusable"
  allow_extra_dns_labels = true
  auto_groups            = [%[2]q]
  ephemeral              = false
  revoked                = false
  usage_limit            = 5
}

data "netbird_setup_key" %[1]q {
  name = netbird_setup_key.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_setup_key" "%[1]s_by_id" {
  id = netbird_setup_key.%[1]s.id
}`, rName, e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_setup_key."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "type", "reusable"),
					resource.TestCheckResourceAttr(dsAddr, "usage_limit", "5"),
					resource.TestCheckResourceAttr(dsAddr, "ephemeral", "false"),
					resource.TestCheckResourceAttr(dsAddr, "revoked", "false"),
					resource.TestCheckResourceAttr(dsAddr, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "auto_groups.0", e2eGroupNotAllID()),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "type", dsAddr, "type"),
				),
			},
		},
	})
}

func Test_Token_DataSource(t *testing.T) {
	env := testE2E(t)
	rName := "tok" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_token." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_token" %[1]q {
  user_id         = %[2]q
  name            = %[1]q
  expiration_days = 90
}

data "netbird_token" %[1]q {
  user_id = %[2]q
  name    = netbird_token.%[1]s.name
}

data "netbird_token" "%[1]s_by_id" {
  user_id = %[2]q
  id      = netbird_token.%[1]s.id
}`, rName, env.UserID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_token."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "name", rName),
					resource.TestCheckResourceAttr(dsAddr, "user_id", env.UserID),
					resource.TestCheckResourceAttrSet(dsAddr, "expiration_date"),
					resource.TestCheckResourceAttrSet(dsAddr, "created_at"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "name", dsAddr, "name"),
				),
			},
		},
	})
}

func Test_User_DataSource(t *testing.T) {
	env := testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_user." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The owner created by POST /api/setup — the one account this
				// deployment is guaranteed to have.
				Config: fmt.Sprintf(`
data "netbird_user" %[1]q {
  email = %[2]q
}

data "netbird_user" "%[1]s_by_id" {
  id = %[3]q
}`, rName, e2eAdminEmail, env.UserID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dsAddr, "id", env.UserID),
					resource.TestCheckResourceAttr(dsAddr, "email", e2eAdminEmail),
					resource.TestCheckResourceAttr(dsAddr, "role", "owner"),
					resource.TestCheckResourceAttr(dsAddr, "is_current", "true"),
					resource.TestCheckResourceAttr(dsAddr, "is_service_user", "false"),
					resource.TestCheckResourceAttr(dsAddr, "is_blocked", "false"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "email", dsAddr, "email"),
				),
			},
		},
	})
}

func Test_NameserverGroup_DataSource(t *testing.T) {
	testE2E(t)
	rName := "ns" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_nameserver_group." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_nameserver_group" %[1]q {
  name = %[1]q
  nameservers = [
    {
      ip      = "1.1.1.1"
      ns_type = "udp"
      port    = 53
    }
  ]
  groups = [%[2]q]
}

data "netbird_nameserver_group" %[1]q {
  name = netbird_nameserver_group.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_nameserver_group" "%[1]s_by_id" {
  id = netbird_nameserver_group.%[1]s.id
}`, rName, e2eGroupAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_nameserver_group."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "nameservers.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "nameservers.0.ip", "1.1.1.1"),
					resource.TestCheckResourceAttr(dsAddr, "nameservers.0.port", "53"),
					resource.TestCheckResourceAttr(dsAddr, "groups.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "groups.0", e2eGroupAllID()),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "nameservers.#", dsAddr, "nameservers.#"),
				),
			},
		},
	})
}

func Test_AgentNetworkProvider_DataSourceLookup(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_agent_network_provider." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAgentNetworkProviderResource(rName, rName, `{ "x-portkey-config" = "pc-ds" }`, "true") + fmt.Sprintf(`

data "netbird_agent_network_provider" %[1]q {
  name = netbird_agent_network_provider.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_agent_network_provider" "%[1]s_by_id" {
  id = netbird_agent_network_provider.%[1]s.id
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_agent_network_provider."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "provider_id", "openai_api"),
					resource.TestCheckResourceAttr(dsAddr, "upstream_url", "https://api.openai.com"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "provider_id", dsAddr, "provider_id"),
				),
			},
		},
	})
}

func Test_AgentNetworkGuardrail_DataSource(t *testing.T) {
	testE2E(t)
	rName := "ang" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_agent_network_guardrail." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_agent_network_guardrail" %[1]q {
  name        = %[1]q
  description = "data source lookup"
  model_allowlist = {
    enabled = true
    models  = ["gpt-4.1"]
  }
  prompt_capture = {
    enabled    = true
    redact_pii = true
  }
}

data "netbird_agent_network_guardrail" %[1]q {
  name = netbird_agent_network_guardrail.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_agent_network_guardrail" "%[1]s_by_id" {
  id = netbird_agent_network_guardrail.%[1]s.id
}`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_agent_network_guardrail."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "description", "data source lookup"),
					resource.TestCheckResourceAttr(dsAddr, "model_allowlist.enabled", "true"),
					resource.TestCheckResourceAttr(dsAddr, "model_allowlist.models.0", "gpt-4.1"),
					resource.TestCheckResourceAttr(dsAddr, "prompt_capture.redact_pii", "true"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "description", dsAddr, "description"),
				),
			},
		},
	})
}

func Test_AgentNetworkPolicy_DataSource(t *testing.T) {
	testE2E(t)
	rName := "ann" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	dsAddr := "data.netbird_agent_network_policy." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
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
}

data "netbird_agent_network_policy" %[1]q {
  name = netbird_agent_network_policy.%[1]s.name
}

# The same record fetched by ID, which the data source resolves with a
# direct GET instead of listing and scanning.
data "netbird_agent_network_policy" "%[1]s_by_id" {
  id = netbird_agent_network_policy.%[1]s.id
}`, rName, testBootstrapCluster),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(dsAddr, "id", "netbird_agent_network_policy."+rName, "id"),
					resource.TestCheckResourceAttr(dsAddr, "source_groups.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "destination_provider_ids.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "guardrail_ids.#", "1"),
					resource.TestCheckResourceAttr(dsAddr, "token_limit.enabled", "true"),
					resource.TestCheckResourceAttr(dsAddr, "token_limit.group_cap", "1000000"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "id", dsAddr, "id"),
					resource.TestCheckResourceAttrPair(dsAddr+"_by_id", "source_groups.#", dsAddr, "source_groups.#"),
				),
			},
		},
	})
}
