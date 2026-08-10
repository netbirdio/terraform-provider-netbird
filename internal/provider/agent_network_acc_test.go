package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Agent Network needs a proxy cluster address to bootstrap the account's
// settings row. Prefer the address of a proxy that is actually attached to this
// deployment, so the bootstrap resolves against a cluster that exists and the
// endpoint the server derives is a real one. The server stores the string
// verbatim without validating it, so a placeholder still works when no proxy is
// running — the agent-network tests are then exercising the same code path
// against a cluster that merely does not answer.
const testBootstrapClusterFallback = "acc.test.invalid"

func testBootstrapCluster() string {
	clusters, err := testClient().ReverseProxyClusters.List(context.Background())
	if err != nil {
		return testBootstrapClusterFallback
	}
	for _, c := range clusters {
		if c.Online && c.Address != "" {
			return c.Address
		}
	}
	return testBootstrapClusterFallback
}

func testAgentNetworkClient() *netbird.AgentNetworkAPI {
	return testClient().AgentNetwork
}

func testGetProvider(id string) (*api.AgentNetworkProvider, error) {
	return testAgentNetworkClient().GetProvider(context.Background(), id)
}

func testAgentNetworkProviderResource(rName, name, extraValues, metadataDisabled string) string {
	return fmt.Sprintf(`resource "netbird_agent_network_provider" "%s" {
	provider_id       = "openai_api"
	name              = "%s"
	upstream_url      = "https://api.openai.com"
	api_key           = "sk-acc-test"
	bootstrap_cluster = "%s"
	extra_values      = %s
	metadata_disabled = %s
}`, rName, name, testBootstrapCluster(), extraValues, metadataDisabled)
}

func Test_AgentNetworkProvider_Create(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_provider." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testAgentNetworkProviderResource(rName, rName, `{ "x-portkey-config" = "pc-acc" }`, "true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "provider_id", "openai_api"),
					resource.TestCheckResourceAttr(rNameFull, "metadata_disabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "extra_values.x-portkey-config", "pc-acc"),
					func(s *terraform.State) error {
						p, err := testGetProvider(s.RootModule().Resources[rNameFull].Primary.Attributes["id"])
						if err != nil {
							return err
						}
						if !p.MetadataDisabled {
							return fmt.Errorf("metadata_disabled not persisted on the management server")
						}
						if p.ExtraValues == nil || (*p.ExtraValues)["x-portkey-config"] != "pc-acc" {
							return fmt.Errorf("extra_values not persisted, found %v", p.ExtraValues)
						}
						return nil
					},
				),
			},
		},
	})
}

// The API rebuilds the provider row from the request on update, so any field the
// provider omits is dropped. Changing only the name must not wipe extra_values
// or reset metadata_disabled.
func Test_AgentNetworkProvider_UpdatePreservesExtraValues(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_provider." + rName

	checkPersisted := func(s *terraform.State) error {
		p, err := testGetProvider(s.RootModule().Resources[rNameFull].Primary.Attributes["id"])
		if err != nil {
			return err
		}
		if p.ExtraValues == nil || (*p.ExtraValues)["x-portkey-config"] != "pc-acc" {
			return fmt.Errorf("extra_values lost on update, found %v", p.ExtraValues)
		}
		if !p.MetadataDisabled {
			return fmt.Errorf("metadata_disabled reset on update")
		}
		return nil
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testAgentNetworkProviderResource(rName, rName, `{ "x-portkey-config" = "pc-acc" }`, "true"),
				Check:        resource.ComposeAggregateTestCheckFunc(checkPersisted),
			},
			{
				// Only the name changes; the other two are left in state.
				ResourceName: rName,
				Config:       testAgentNetworkProviderResource(rName, rName+"-renamed", `{ "x-portkey-config" = "pc-acc" }`, "true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "name", rName+"-renamed"),
					checkPersisted,
				),
			},
		},
	})
}

// provider_id is mutable server-side, so changing it must update in place. A
// forced replacement would have to delete the provider first, which the server
// refuses while a policy still references it.
func Test_AgentNetworkProvider_ProviderIdUpdatesInPlace(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_provider." + rName

	config := func(providerID, upstream string) string {
		return fmt.Sprintf(`resource "netbird_agent_network_provider" "%s" {
	provider_id       = "%s"
	name              = "%s"
	upstream_url      = "%s"
	api_key           = "sk-acc-test"
	bootstrap_cluster = "%s"
}`, rName, providerID, rName, upstream, testBootstrapCluster())
	}

	var firstID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config("openai_api", "https://api.openai.com"),
				Check: resource.ComposeAggregateTestCheckFunc(func(s *terraform.State) error {
					firstID = s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
					return nil
				}),
			},
			{
				ResourceName: rName,
				Config:       config("anthropic_api", "https://api.anthropic.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "provider_id", "anthropic_api"),
					func(s *terraform.State) error {
						if got := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]; got != firstID {
							return fmt.Errorf("provider was replaced (id %s -> %s); provider_id should update in place", firstID, got)
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_AgentNetworkGuardrail_Create(t *testing.T) {
	testE2E(t)
	rName := "ang" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_guardrail." + rName

	config := func(models string, redactPii string) string {
		return fmt.Sprintf(`resource "netbird_agent_network_guardrail" "%s" {
	name        = "%s"
	description = "acceptance test"
	model_allowlist = {
		enabled = true
		models  = %s
	}
	prompt_capture = {
		enabled    = true
		redact_pii = %s
	}
}`, rName, rName, models, redactPii)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config(`["gpt-4.1"]`, "true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "model_allowlist.models.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "prompt_capture.redact_pii", "true"),
				),
			},
			{
				ResourceName: rName,
				Config:       config(`["gpt-4.1", "o4-mini"]`, "false"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "model_allowlist.models.#", "2"),
					resource.TestCheckResourceAttr(rNameFull, "prompt_capture.redact_pii", "false"),
				),
			},
		},
	})
}

func Test_AgentNetworkPolicy_Create(t *testing.T) {
	testE2E(t)
	rName := "anpol" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_policy." + rName

	config := fmt.Sprintf(`
resource "netbird_group" "%[1]s" {
	name = "%[1]s-group"
}

resource "netbird_agent_network_provider" "%[1]s" {
	provider_id       = "openai_api"
	name              = "%[1]s-provider"
	upstream_url      = "https://api.openai.com"
	api_key           = "sk-acc-test"
	bootstrap_cluster = "%[2]s"
}

resource "netbird_agent_network_guardrail" "%[1]s" {
	name = "%[1]s-guardrail"
	model_allowlist = {
		enabled = true
		models  = ["gpt-4.1"]
	}
	prompt_capture = {
		enabled = false
	}
}

resource "netbird_agent_network_policy" "%[1]s" {
	name                     = "%[1]s"
	source_groups            = [netbird_group.%[1]s.id]
	destination_provider_ids = [netbird_agent_network_provider.%[1]s.id]
	guardrail_ids            = [netbird_agent_network_guardrail.%[1]s.id]

	token_limit = {
		enabled        = true
		group_cap      = 1000000
		window_seconds = 86400
	}
}`, rName, testBootstrapCluster())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "source_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "destination_provider_ids.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "guardrail_ids.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "token_limit.enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "token_limit.group_cap", "1000000"),
					// Not set in config, so it takes the schema default.
					resource.TestCheckResourceAttr(rNameFull, "token_limit.user_cap", "0"),
				),
			},
		},
	})
}

// The server rejects an enabled limit whose caps are all zero. That must be
// caught during plan rather than failing partway through an apply.
func Test_AgentNetworkPolicy_EnabledLimitNeedsACap(t *testing.T) {
	testE2E(t)
	rName := "anpol" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	config := fmt.Sprintf(`
resource "netbird_group" "%[1]s" {
	name = "%[1]s-group"
}

resource "netbird_agent_network_policy" "%[1]s" {
	name                     = "%[1]s"
	source_groups            = [netbird_group.%[1]s.id]
	destination_provider_ids = ["does-not-matter"]

	token_limit = {
		enabled = true
	}
}`, rName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config,
				ExpectError:  regexp.MustCompile(`group_cap or user_cap to be greater than 0`),
			},
		},
	})
}

// Required lists must be rejected at plan time when empty.
func Test_AgentNetworkPolicy_EmptySourceGroupsRejected(t *testing.T) {
	testE2E(t)
	rName := "anpol" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	config := fmt.Sprintf(`resource "netbird_agent_network_policy" "%[1]s" {
	name                     = "%[1]s"
	source_groups            = []
	destination_provider_ids = ["does-not-matter"]
}`, rName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config,
				ExpectError:  regexp.MustCompile(`at least 1 element`),
			},
		},
	})
}

// The settings row is a server-managed singleton and the API replaces every
// mutable field on PUT. Managing only one field must leave the others at their
// existing account values instead of resetting them to the schema defaults.
func Test_AgentNetworkSettings_AdoptsExistingValues(t *testing.T) {
	testE2E(t)
	rName := "ans" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_settings." + rName

	// The provider create bootstraps the settings row, which the server seeds
	// with log collection enabled. The settings resource manages only the
	// retention, so log collection must survive.
	config := fmt.Sprintf(`
resource "netbird_agent_network_provider" "%[1]s" {
	provider_id       = "openai_api"
	name              = "%[1]s-provider"
	upstream_url      = "https://api.openai.com"
	api_key           = "sk-acc-test"
	bootstrap_cluster = "%[2]s"
}

resource "netbird_agent_network_settings" "%[1]s" {
	access_log_retention_days = 45
	depends_on                = [netbird_agent_network_provider.%[1]s]
}`, rName, testBootstrapCluster())

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			// Normalize the singleton so the assertion does not depend on
			// whatever a previous run left behind.
			c := testAgentNetworkClient()
			if cur, err := c.GetSettings(context.Background()); err == nil && !cur.EnableLogCollection {
				retention := 30
				if _, err := c.UpdateSettings(context.Background(), api.AgentNetworkSettingsRequest{
					EnableLogCollection:    true,
					AccessLogRetentionDays: &retention,
				}); err != nil {
					t.Fatalf("failed to normalize agent-network settings: %v", err)
				}
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "access_log_retention_days", "45"),
					resource.TestCheckResourceAttrSet(rNameFull, "endpoint"),
					// Unmanaged in config, so it must reflect the account value.
					resource.TestCheckResourceAttr(rNameFull, "enable_log_collection", "true"),
					func(s *terraform.State) error {
						got, err := testAgentNetworkClient().GetSettings(context.Background())
						if err != nil {
							return err
						}
						if !got.EnableLogCollection {
							return fmt.Errorf("enable_log_collection was clobbered to false by an unrelated settings apply")
						}
						if got.AccessLogRetentionDays == nil || *got.AccessLogRetentionDays != 45 {
							return fmt.Errorf("access_log_retention_days not applied, found %v", got.AccessLogRetentionDays)
						}
						return nil
					},
				),
			},
		},
	})
}

// Test_AgentNetworkSettings_BootstrapViaCluster covers the settings-first
// bootstrap path: a settings resource carrying `cluster` creates the
// account's settings row without any provider existing. It needs a
// management server with the settings-defaults contract AND an account that
// has never been bootstrapped, so it self-skips against older servers (where
// the unbootstrapped read surfaces as not-found) and on accounts a previous
// test already bootstrapped — on a fresh compose environment it runs when
// scheduled before the provider tests.
func Test_AgentNetworkSettings_BootstrapViaCluster(t *testing.T) {
	testE2E(t)
	rName := "ansboot" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_settings." + rName

	config := fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	cluster               = "%[2]s"
	enable_log_collection = true
	redact_pii            = true
}`, rName, testBootstrapCluster())

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			s, err := testAgentNetworkClient().GetSettings(context.Background())
			if err != nil {
				if netbird.IsNotFound(err) {
					t.Skip("management server predates the agent-network settings defaults contract")
				}
				t.Fatalf("failed to read agent-network settings: %v", err)
			}
			if s.Endpoint != "" {
				t.Skip("account already bootstrapped; the settings cluster bootstrap needs a fresh account")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "cluster", testBootstrapCluster()),
					resource.TestCheckResourceAttrSet(rNameFull, "subdomain"),
					resource.TestMatchResourceAttr(rNameFull, "endpoint",
						regexp.MustCompile(`\.`+regexp.QuoteMeta(testBootstrapCluster())+`$`)),
					resource.TestCheckResourceAttr(rNameFull, "enable_log_collection", "true"),
					resource.TestCheckResourceAttr(rNameFull, "redact_pii", "true"),
					func(s *terraform.State) error {
						got, err := testAgentNetworkClient().GetSettings(context.Background())
						if err != nil {
							return err
						}
						if got.Cluster != testBootstrapCluster() {
							return fmt.Errorf("cluster not pinned by the settings bootstrap, found %q", got.Cluster)
						}
						if got.Endpoint == "" {
							return fmt.Errorf("account still reads as unbootstrapped after the settings apply")
						}
						return nil
					},
				),
			},
			{
				// The cluster is pinned for good: a change must fail at plan
				// time, before any destroy or API call can run.
				ResourceName: rName,
				Config: fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	cluster               = "changed.cluster.invalid"
	enable_log_collection = true
	redact_pii            = true
}`, rName),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Cluster is immutable once assigned`),
			},
		},
	})
}
