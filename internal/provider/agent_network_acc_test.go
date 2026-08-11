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

// The acceptance tests below run against the dockerized management server
// started by testEnsureManagementRunning. Bootstrapping the account's gateway
// needs a proxy cluster address, but the server stores that string verbatim
// without validating that a proxy answers there, so a placeholder is enough.
const testProxyAddress = "acc.test.invalid"

// testBootstrapGateway makes sure the account has a gateway, which providers no
// longer create as a side effect: bootstrapping is a settings create of its own.
// Idempotent, so every test that needs an endpoint can call it.
func testBootstrapGateway(t *testing.T) {
	t.Helper()
	c := testAgentNetworkClient()
	if s, err := c.GetSettings(context.Background()); err == nil && s.Endpoint != "" {
		return
	}
	if _, err := c.CreateSettings(context.Background(), api.AgentNetworkSettingsCreateRequest{
		ProxyAddress: valPtr(testProxyAddress),
	}); err != nil {
		t.Fatalf("bootstrap the agent network gateway: %v", err)
	}
}

// testReleaseGateway takes the account back to having no gateway, so a bootstrap
// test exercises the create path whatever ran before it. Releasing is refused
// while providers still route through the gateway; a test that finds the account
// in that state has nothing it can do, so it skips.
func testReleaseGateway(t *testing.T) {
	t.Helper()
	c := testAgentNetworkClient()
	s, err := c.GetSettings(context.Background())
	if err != nil {
		if netbird.IsNotFound(err) {
			t.Skip("management server predates the agent-network settings defaults contract")
		}
		t.Fatalf("read agent network settings: %v", err)
	}
	if s.Endpoint == "" {
		return
	}
	if err := c.DeleteSettings(context.Background()); err != nil {
		t.Skipf("the account's gateway cannot be released: %v", err)
	}
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
	extra_values      = %s
	metadata_disabled = %s
}`, rName, name, extraValues, metadataDisabled)
}

func Test_AgentNetworkProvider_Create(t *testing.T) {
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
	rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_provider." + rName

	config := func(providerID, upstream string) string {
		return fmt.Sprintf(`resource "netbird_agent_network_provider" "%s" {
	provider_id       = "%s"
	name              = "%s"
	upstream_url      = "%s"
	api_key           = "sk-acc-test"
}`, rName, providerID, rName, upstream)
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
}`, rName)

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
	rName := "ans" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_settings." + rName

	// The account's gateway is bootstrapped with log collection enabled. This
	// resource manages only the retention, so log collection must survive.
	//
	// The provider depends on the settings, not the other way round: the server
	// refuses to release a gateway while providers still route through it, so the
	// gateway has to be created first and destroyed last.
	config := fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	access_log_retention_days = 45
}

resource "netbird_agent_network_provider" "%[1]s" {
	provider_id       = "openai_api"
	name              = "%[1]s-provider"
	upstream_url      = "https://api.openai.com"
	api_key           = "sk-acc-test"
	depends_on        = [netbird_agent_network_settings.%[1]s]
}`, rName)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			// The settings resource adopts an existing gateway rather than
			// creating one, so the account needs to have been bootstrapped.
			testBootstrapGateway(t)
			// Normalize the singleton so the assertion does not depend on
			// whatever a previous run left behind. The addresses are echoed
			// because the update requires them unchanged.
			c := testAgentNetworkClient()
			if cur, err := c.GetSettings(context.Background()); err == nil && !cur.EnableLogCollection {
				if _, err := c.UpdateSettings(context.Background(), api.AgentNetworkSettingsRequest{
					Endpoint:               cur.Endpoint,
					ProxyAddress:           cur.ProxyAddress,
					EnableLogCollection:    true,
					AccessLogRetentionDays: 30,
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

// Test_AgentNetworkSettings_BootstrapViaProxyAddress covers the bootstrap path:
// a settings resource carrying `proxy_address` creates the account's gateway,
// allocating an endpoint one label beneath that cluster. Nothing else bootstraps
// an account, so the gateway is released first and the create path is what runs,
// whatever ran before this test.
func Test_AgentNetworkSettings_BootstrapViaProxyAddress(t *testing.T) {
	rName := "ansboot" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_settings." + rName

	config := fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	proxy_address         = "%[2]s"
	enable_log_collection = true
	redact_pii            = true
}`, rName, testProxyAddress)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			testReleaseGateway(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "proxy_address", testProxyAddress),
					// An endpoint beneath a shared cluster, so not dedicated.
					resource.TestCheckResourceAttr(rNameFull, "dedicated", "false"),
					resource.TestMatchResourceAttr(rNameFull, "endpoint",
						regexp.MustCompile(`^[^.]+\.`+regexp.QuoteMeta(testProxyAddress)+`$`)),
					resource.TestCheckResourceAttr(rNameFull, "enable_log_collection", "true"),
					resource.TestCheckResourceAttr(rNameFull, "redact_pii", "true"),
					func(s *terraform.State) error {
						got, err := testAgentNetworkClient().GetSettings(context.Background())
						if err != nil {
							return err
						}
						if got.ProxyAddress != testProxyAddress {
							return fmt.Errorf("proxy address not pinned by the bootstrap, found %q", got.ProxyAddress)
						}
						if got.Endpoint == "" {
							return fmt.Errorf("account still reads as unbootstrapped after the settings apply")
						}
						attrs := s.RootModule().Resources[rNameFull].Primary.Attributes
						if attrs["endpoint"] != got.Endpoint {
							return fmt.Errorf("endpoint mismatch: state %q, API %q", attrs["endpoint"], got.Endpoint)
						}
						return nil
					},
				),
			},
			{
				// The addresses are assigned once. A different one has to be
				// reported as a replacement, not applied in place.
				ResourceName: rName,
				Config: fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	proxy_address         = "changed.cluster.invalid"
	enable_log_collection = true
	redact_pii            = true
}`, rName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// A gateway can also be claimed by hostname, which makes it dedicated: the proxy
// at that address is the account's gateway, so endpoint and proxy_address are the
// same. Only reachable on an account with no gateway yet.
func Test_AgentNetworkSettings_BootstrapDedicatedEndpoint(t *testing.T) {
	rName := "ansded" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_agent_network_settings." + rName
	endpoint := rName + ".acc.test.invalid"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			testReleaseGateway(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config: fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	endpoint = "%[2]s"
}`, rName, endpoint),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "endpoint", endpoint),
					// A claimed hostname is served by a proxy declaring exactly
					// that address, so the two addresses coincide.
					resource.TestCheckResourceAttr(rNameFull, "proxy_address", endpoint),
					resource.TestCheckResourceAttr(rNameFull, "dedicated", "true"),
					func(s *terraform.State) error {
						got, err := testAgentNetworkClient().GetSettings(context.Background())
						if err != nil {
							return err
						}
						if got.Endpoint != endpoint || got.ProxyAddress != endpoint {
							return fmt.Errorf("management holds endpoint %q / proxy address %q, expected both to be %q",
								got.Endpoint, got.ProxyAddress, endpoint)
						}
						if !got.Dedicated {
							return fmt.Errorf("a self-addressed gateway should report dedicated")
						}
						return nil
					},
				),
			},
		},
	})
}

// Bootstrapping needs one of the two addresses. Without either, the apply has to
// say so rather than reach an API that would refuse it.
func Test_AgentNetworkSettings_BootstrapRequiresAnAddress(t *testing.T) {
	rName := "ansreq" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			testReleaseGateway(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config: fmt.Sprintf(`
resource "netbird_agent_network_settings" "%[1]s" {
	enable_log_collection = true
}`, rName),
				ExpectError: regexp.MustCompile(`Agent Network not bootstrapped`),
			},
		},
	})
}
