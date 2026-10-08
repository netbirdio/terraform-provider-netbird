//go:build e2e

// Data source coverage.
//
// 19 of the provider's 25 data sources had no acceptance test. A data source is
// a read path with its own schema and its own mapping code, so an untested one
// can return the wrong field, drop an optional one, or diverge from the resource
// that produced the object, and nothing would say so.
//
// Each test here creates a resource and then reads it back through the matching
// data source in the same configuration, so the data source is exercised against
// an object whose values the test already knows. The central assertion is
// TestCheckResourceAttrPair: the data source and the resource must agree
// attribute by attribute. That is stronger than asserting literals, because it
// also fails when both sides are wrong in different ways, and it does not have
// to be updated when a fixture value changes.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// dataSourceByID reads back the resource the same configuration just created,
// addressed by its server-assigned ID. Referencing the resource's attribute is
// what orders the two: Terraform will not read the data source until the
// resource exists.
func dataSourceByID(kind, rName string) string {
	return fmt.Sprintf(`
data "netbird_%[1]s" "%[2]s" {
  id = netbird_%[1]s.%[2]s.id
}
`, kind, rName)
}

// samePair asserts the data source agrees with the resource on each attribute.
func samePair(kind, rName string, attrs ...string) resource.TestCheckFunc {
	ds := "data.netbird_" + kind + "." + rName
	res := "netbird_" + kind + "." + rName
	checks := []resource.TestCheckFunc{resource.TestCheckResourceAttrSet(ds, "id")}
	for _, a := range attrs {
		checks = append(checks, resource.TestCheckResourceAttrPair(ds, a, res, a))
	}
	return resource.ComposeAggregateTestCheckFunc(checks...)
}

// dsCase runs one create-then-read-back case.
func dsCase(t *testing.T, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    []resource.TestStep{{Config: config, Check: check}},
	})
}

func Test_Group_DataSource(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	dsCase(t, testGroupResource(rName, `[]`)+dataSourceByID("group", rName),
		samePair("group", rName, "name", "peers.#"))
}

func Test_Network_DataSource(t *testing.T) {
	testE2E(t)
	rName := "n" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	dsCase(t, testNetworkResource(rName, `Test`)+dataSourceByID("network", rName),
		samePair("network", rName, "name", "description"))
}

func Test_NameserverGroup_DataSource(t *testing.T) {
	testE2E(t)
	rName := "ns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID()))
	dsCase(t, cfg+dataSourceByID("nameserver_group", rName),
		samePair("nameserver_group", rName, "name", "enabled", "nameservers.#", "groups.#"))
}

func Test_Policy_DataSource(t *testing.T) {
	testE2E(t)
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testPolicyResourceGroups(rName, rName, "desc", "accept", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "443")
	dsCase(t, cfg+dataSourceByID("policy", rName),
		samePair("policy", rName, "name", "description", "enabled"))
}

func Test_PostureCheck_DataSource(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testPostureCheckResource(rName, `posture_check`, `0.40.0`, `15`, `10`, `12`, `6.8.0`, `2531`,
		`EG`, `Cairo`, `allow`, `15.160.0.0/16`, `deny`, `/root`, `C:\\process.exe`, `/macpath`)
	dsCase(t, cfg+dataSourceByID("posture_check", rName),
		samePair("posture_check", rName, "name", "description"))
}

func Test_Route_DataSource(t *testing.T) {
	testE2E(t)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testRouteResource(rName, e2eGroupAllID(), `null`, `desc`, `null`, `["example.com"]`,
		fmt.Sprintf("[%q]", e2eGroupNotAllID()), `null`)
	dsCase(t, cfg+dataSourceByID("route", rName),
		samePair("route", rName, "network_id", "description", "groups.#"))
}

func Test_SetupKey_DataSource(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testSetupKeyResource(rName, `1800`, `reusable`, `false`, `[]`, `false`, `false`, `0`)
	// Not the key itself: the server returns it only at creation, so the data
	// source cannot know it and comparing the two would assert nothing useful.
	dsCase(t, cfg+dataSourceByID("setup_key", rName),
		samePair("setup_key", rName, "name", "type", "ephemeral", "revoked"))
}

func Test_User_DataSource(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `user`)
	dsCase(t, cfg+dataSourceByID("user", rName),
		samePair("user", rName, "name", "role", "is_service_user", "is_blocked"))
}

func Test_IdentityProvider_DataSource(t *testing.T) {
	testE2E(t)
	rName := "idp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	// The issuer has to be a real one. The provider fetches
	// <issuer>/.well-known/openid-configuration before accepting the resource, so
	// an invented hostname fails with "identity provider issuer is unreachable"
	// rather than testing anything. That also means this resource cannot be
	// covered on a runner without egress — the pre-existing tests only pass
	// because their JumpCloud issuer resolves.
	//
	// The type stays "oidc" because the two OAuth2 types, google and microsoft,
	// do not survive a read: the management server stores those connectors
	// without the issuer, so it comes back empty and the data source can only
	// report what the API tells it.
	cfg := testIdentityProviderResource(rName, "jumpcloud", "oidc", "client-id", "client-secret",
		"https://oauth.id.jumpcloud.com/")
	// client_secret is write-only, so it is not comparable.
	dsCase(t, cfg+dataSourceByID("identity_provider", rName),
		samePair("identity_provider", rName, "name", "type", "client_id", "issuer"))
}

func Test_DNSSettings_DataSource(t *testing.T) {
	testE2E(t)
	rName := "dns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	// The settings are per-account rather than per-object, so the data source
	// takes no selector and there is nothing to address it by.
	cfg := testDNSSettingsResource(rName, "[]") + `
data "netbird_dns_settings" "current" {}
`
	dsCase(t, cfg, resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttrPair("data.netbird_dns_settings.current",
			"disabled_management_groups.#", "netbird_dns_settings."+rName, "disabled_management_groups.#"),
	))
}

func Test_AccountSettings_DataSource(t *testing.T) {
	env := testE2E(t)
	// Also a per-account singleton, and one that exists without being created,
	// so this reads the account the harness bootstrapped.
	dsCase(t, `data "netbird_account_settings" "current" {}`,
		resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.netbird_account_settings.current", "id", env.AccountID),
			resource.TestCheckResourceAttrSet("data.netbird_account_settings.current", "peer_login_expiration"),
		))
}

func Test_Peer_DataSource(t *testing.T) {
	testE2E(t)
	// A peer is registered by an agent rather than created by Terraform, so this
	// reads one of the shared fixtures directly instead of creating anything.
	peerID := testPeerID(t, "peer1")
	dsCase(t, fmt.Sprintf(`
data "netbird_peer" "p" {
  id = %q
}
`, peerID), resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.netbird_peer.p", "id", peerID),
		resource.TestCheckResourceAttr("data.netbird_peer.p", "name", "peer1"),
		resource.TestCheckResourceAttrSet("data.netbird_peer.p", "os"),
		resource.TestCheckResourceAttrSet("data.netbird_peer.p", "ip"),
	))
}

func Test_NetworkResource_DataSource(t *testing.T) {
	testE2E(t)
	rName := "nre" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testNetworkResourceResource(rName, e2eNetworkID(), `example.com`,
		fmt.Sprintf("[%q, %q]", e2eGroupNotAllID(), e2eGroupAllID()), rName) + fmt.Sprintf(`
data "netbird_network_resource" "%[1]s" {
  network_id = %[2]q
  id         = netbird_network_resource.%[1]s.id
}
`, rName, e2eNetworkID())
	dsCase(t, cfg, samePair("network_resource", rName, "name", "address", "groups.#"))
}

func Test_NetworkRouter_DataSource(t *testing.T) {
	testE2E(t)
	rName := "nro" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testNetworkRouterResource(rName, e2eNetworkID(), fmt.Sprintf("[%q]", e2eGroupNotAllID())) +
		fmt.Sprintf(`
data "netbird_network_router" "%[1]s" {
  network_id = %[2]q
  id         = netbird_network_router.%[1]s.id
}
`, rName, e2eNetworkID())
	dsCase(t, cfg, samePair("network_router", rName, "peer_groups.#", "masquerade", "metric"))
}

func Test_Token_DataSource(t *testing.T) {
	testE2E(t)
	rName := "t" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	userID := mustE2E().UserID
	cfg := testTokenResource(rName, userID, `180`) + fmt.Sprintf(`
data "netbird_token" "%[1]s" {
  user_id = %[2]q
  id      = netbird_token.%[1]s.id
}
`, rName, userID)
	// The token value itself is returned only at creation, so it is not
	// comparable against a read.
	dsCase(t, cfg, samePair("token", rName, "name", "user_id"))
}

func Test_AgentNetworkProvider_DataSource(t *testing.T) {
	testE2E(t)
	rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testAgentNetworkProviderResource(rName, rName, `{ "x-portkey-config" = "pc-ds" }`, "false")
	// api_key is write-only.
	dsCase(t, cfg+dataSourceByID("agent_network_provider", rName),
		samePair("agent_network_provider", rName, "name", "enabled"))
}

func Test_AgentNetworkGuardrail_DataSource(t *testing.T) {
	testE2E(t)
	rName := "ang" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	dsCase(t, testAgentNetworkGuardrailResource(rName)+dataSourceByID("agent_network_guardrail", rName),
		samePair("agent_network_guardrail", rName, "name", "description",
			"model_allowlist.enabled", "model_allowlist.models.#",
			"prompt_capture.enabled", "prompt_capture.redact_pii"))
}

func Test_AgentNetworkPolicy_DataSource(t *testing.T) {
	testE2E(t)
	rName := "anpol" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	dsCase(t, testAgentNetworkPolicyResource(rName)+dataSourceByID("agent_network_policy", rName),
		samePair("agent_network_policy", rName, "name", "description", "enabled",
			"source_groups.#", "destination_provider_ids.#", "guardrail_ids.#",
			"token_limit.enabled", "token_limit.group_cap", "token_limit.window_seconds"))
}

func Test_Scim_DataSource(t *testing.T) {
	// Skipped for the same reason as the SCIM resource tests: the integration is
	// cloud-only, so a self-hosted deployment has nothing to read.
	t.Skip("skipping cloud test")
	rName := "scim" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testScimResource(rName, "azure", `"azure-scim"`, `null`, `["eng"]`, `["users"]`)
	dsCase(t, cfg+dataSourceByID("scim", rName),
		samePair("scim", rName, "provider_name", "enabled"))
}
