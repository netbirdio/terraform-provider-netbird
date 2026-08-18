//go:build e2e

// How a data source is addressed, rather than what it returns.
//
// data_source_acc_test.go reads every data source back by ID and compares it
// attribute by attribute against the resource that produced it. That covers the
// mapping and nothing else: sixteen data sources accept a name — or an email, a
// network ID, a provider name — as an alternative selector, and each one also
// has to say something sensible when it is given no selector at all or when
// more than one object answers.
//
// Those are three separate code paths per data source, none of which the by-ID
// read touches:
//
//   - the selector match itself, which is a different comparison from an ID
//     lookup and in most of these resources means listing and filtering;
//   - the refusal when nothing is set, which is the provider's own error rather
//     than the server's;
//   - the refusal when the filter is ambiguous, which is the one that protects a
//     user from silently getting whichever object happened to be first.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// noSelector is the shape of the provider's own refusal. The wording differs
// between data sources — "Must add at least one of" against "Must set at least
// one of" — which is worth matching loosely rather than pinning.
var noSelector = regexp.MustCompile(`(?s)No selector|at least one of`)

// noMatch is the refusal when a filter is well formed and answers nothing. The
// wording splits the same way — "matching parameters not found" against
// "matched the given filters" — so both are accepted.
var noMatch = regexp.MustCompile(`(?s)No [Mm]atch|not found|matched the given filters`)

// ambiguous is what a filter matching more than one object has to produce.
var ambiguous = regexp.MustCompile(`(?s)Multiple Matches|match(ed)? more than one|cannot match multiple`)

// Test_DataSource_SelectsByName reads back through the alternative selector
// rather than the ID, and requires the two to be the same object.
func Test_DataSource_SelectsByName(t *testing.T) {
	testE2E(t)

	t.Run("group by name", func(t *testing.T) {
		rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testGroupResource(rName, `[]`) + fmt.Sprintf(`
data "netbird_group" "%[1]s" {
  name = netbird_group.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("group", rName, "id", "name", "peers.#"))
	})

	t.Run("network by name", func(t *testing.T) {
		rName := "n" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testNetworkResource(rName, `Test`) + fmt.Sprintf(`
data "netbird_network" "%[1]s" {
  name = netbird_network.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("network", rName, "id", "name", "description"))
	})

	t.Run("setup key by name", func(t *testing.T) {
		rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testSetupKeyResourceNoLimit(rName, `reusable`) + fmt.Sprintf(`
data "netbird_setup_key" "%[1]s" {
  name = netbird_setup_key.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("setup_key", rName, "id", "name", "type", "revoked"))
	})

	t.Run("user by email", func(t *testing.T) {
		// The user data source takes three selectors. email is the one a person
		// would reach for and the only one that is not a name or an ID.
		rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := fmt.Sprintf(`
resource "netbird_user" "%[1]s" {
  name            = "%[1]s"
  email           = "%[1]s@example.com"
  is_service_user = false
  role            = "user"
  auto_groups     = []
}

data "netbird_user" "%[1]s" {
  email = netbird_user.%[1]s.email
}
`, rName)
		dsCase(t, cfg, samePair("user", rName, "id", "name", "email", "role"))
	})

	t.Run("posture check by name", func(t *testing.T) {
		rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := fmt.Sprintf(`resource "netbird_posture_check" "%[1]s" {
  name        = "%[1]s"
  description = "selected by name"

  netbird_version_check {
    min_version = "0.40.0"
  }
}

data "netbird_posture_check" "%[1]s" {
  name = netbird_posture_check.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("posture_check", rName, "id", "name", "description"))
	})

	t.Run("route by network_id", func(t *testing.T) {
		// A route has no name. Its selector is the network_id, which is a
		// user-chosen label rather than a server-assigned ID, so it is the same
		// kind of alternative selector as a name.
		rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testRouteResource(rName, e2eGroupAllID(), `null`, `desc`, `null`, `["example.com"]`,
			fmt.Sprintf("[%q]", e2eGroupNotAllID()), `null`) + fmt.Sprintf(`
data "netbird_route" "%[1]s" {
  network_id = netbird_route.%[1]s.network_id
}
`, rName)
		dsCase(t, cfg, samePair("route", rName, "id", "network_id", "description"))
	})

	t.Run("nameserver group by name", func(t *testing.T) {
		rName := "ns" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID())) +
			fmt.Sprintf(`
data "netbird_nameserver_group" "%[1]s" {
  name = netbird_nameserver_group.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("nameserver_group", rName, "id", "name", "enabled"))
	})

	t.Run("policy by name", func(t *testing.T) {
		rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testPolicyResourceGroups(rName, rName, "desc", "accept", "udp",
			e2eGroupAllID(), e2eGroupNotAllID(), "443") + fmt.Sprintf(`
data "netbird_policy" "%[1]s" {
  name = netbird_policy.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("policy", rName, "id", "name", "enabled"))
	})

	t.Run("network resource by name", func(t *testing.T) {
		// The parent is required either way, so the selector under test is the
		// name within it rather than the pair.
		rName := "nr" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testNetworkResourceResource(rName, e2eNetworkID(), "example.com",
			fmt.Sprintf("[%q]", e2eGroupNotAllID()), rName) + fmt.Sprintf(`
data "netbird_network_resource" "%[1]s" {
  network_id = %[2]q
  name       = netbird_network_resource.%[1]s.name
}
`, rName, e2eNetworkID())
		dsCase(t, cfg, samePair("network_resource", rName, "id", "name", "address"))
	})

	t.Run("token by name", func(t *testing.T) {
		rName := "t" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		userID := mustE2E().UserID
		cfg := testTokenResource(rName, userID, `90`) + fmt.Sprintf(`
data "netbird_token" "%[1]s" {
  user_id = %[2]q
  name    = netbird_token.%[1]s.name
}
`, rName, userID)
		dsCase(t, cfg, samePair("token", rName, "id", "name", "user_id"))
	})

	t.Run("peer by name", func(t *testing.T) {
		// A peer is registered by an agent rather than created by Terraform, so
		// this reads a fixture by the hostname it registered under.
		name := "peer1"
		id := testPeerID(t, name)
		dsCase(t, fmt.Sprintf(`
data "netbird_peer" "byname" {
  name = %q
}
`, name), resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.netbird_peer.byname", "id", id),
			resource.TestCheckResourceAttr("data.netbird_peer.byname", "name", name),
			resource.TestCheckResourceAttrSet("data.netbird_peer.byname", "ip"),
		))
	})

	t.Run("agent network provider by name", func(t *testing.T) {
		rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testAgentNetworkProviderResource(rName, rName, `null`, "false") + fmt.Sprintf(`
data "netbird_agent_network_provider" "%[1]s" {
  name = netbird_agent_network_provider.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("agent_network_provider", rName, "id", "name", "enabled"))
	})

	t.Run("agent network guardrail by name", func(t *testing.T) {
		rName := "ang" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testAgentNetworkGuardrailResource(rName) + fmt.Sprintf(`
data "netbird_agent_network_guardrail" "%[1]s" {
  name = netbird_agent_network_guardrail.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("agent_network_guardrail", rName,
			"id", "name", "model_allowlist.models.#", "prompt_capture.enabled"))
	})

	t.Run("agent network policy by name", func(t *testing.T) {
		rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testAgentNetworkPolicyResource(rName) + fmt.Sprintf(`
data "netbird_agent_network_policy" "%[1]s" {
  name = netbird_agent_network_policy.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("agent_network_policy", rName,
			"id", "name", "enabled", "source_groups.#", "destination_provider_ids.#"))
	})
}

// Test_DataSource_RejectsNoSelector covers the refusal every one of these data
// sources has to make. It is the provider's own error, raised before anything
// reaches the server, so none of these cases needs a deployment to answer.
func Test_DataSource_RejectsNoSelector(t *testing.T) {
	testE2E(t)

	// Every data source whose selectors are all optional, so that an empty block
	// reaches the provider's guard rather than Terraform's own required-argument
	// check. peers is in the list because it filters rather than looks up, and
	// still refuses to return the whole account.
	for _, kind := range []string{
		"group", "network", "setup_key", "user", "posture_check", "route",
		"nameserver_group", "policy", "peers", "identity_provider", "scim",
		"agent_network_provider", "agent_network_policy", "agent_network_guardrail",
	} {
		t.Run(kind, func(t *testing.T) {
			rejects(t, fmt.Sprintf("data \"netbird_%s\" \"nothing\" {}\n", kind), noSelector)
		})
	}

	// Two more take a parent, which Terraform requires, so an empty block would
	// fail on that instead. The parent is given and the selector left out.
	t.Run("network_resource", func(t *testing.T) {
		rejects(t, fmt.Sprintf(`
data "netbird_network_resource" "nothing" {
  network_id = %q
}
`, e2eNetworkID()), noSelector)
	})

	t.Run("token", func(t *testing.T) {
		rejects(t, fmt.Sprintf(`
data "netbird_token" "nothing" {
  user_id = %q
}
`, mustE2E().UserID), noSelector)
	})
}

// Test_DataSource_RejectsNoMatch asks for a name nothing carries. The answer
// has to be an error naming the failure, not an empty object: a data source that
// returns zeros for a name the user misspelled hands those zeros to whatever
// referenced it.
func Test_DataSource_RejectsNoMatch(t *testing.T) {
	testE2E(t)

	// identity_provider and scim are left out: their list endpoints are
	// cloud-only, so a self-hosted deployment answers with an error about the
	// integration rather than about the filter.
	for _, kind := range []string{
		"group", "network", "setup_key", "user", "posture_check",
		"nameserver_group", "policy",
		"agent_network_provider", "agent_network_policy", "agent_network_guardrail",
	} {
		t.Run(kind, func(t *testing.T) {
			rejects(t, fmt.Sprintf(`
data "netbird_%s" "missing" {
  name = "does-not-exist-%s"
}
`, kind, acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)), noMatch)
		})
	}

	// A route is selected by network_id rather than name.
	t.Run("route", func(t *testing.T) {
		rejects(t, fmt.Sprintf(`
data "netbird_route" "missing" {
  network_id = "does-not-exist-%s"
}
`, acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)), noMatch)
	})
}

// Test_DataSource_RejectsDisagreeingSelectors gives two selectors that point at
// different objects. Both are satisfiable on their own, which is what makes this
// worth a test: a data source that took the first one and ignored the second
// would return an object the configuration did not ask for.
func Test_DataSource_RejectsDisagreeingSelectors(t *testing.T) {
	testE2E(t)

	t.Run("group id and name", func(t *testing.T) {
		rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		rejects(t, testGroupResource(rName, `[]`)+fmt.Sprintf(`
data "netbird_group" "%[1]s" {
  id   = netbird_group.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName), noMatch)
	})

	t.Run("user id and email", func(t *testing.T) {
		rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		rejects(t, fmt.Sprintf(`
resource "netbird_user" "%[1]s" {
  name            = "%[1]s"
  email           = "%[1]s@example.com"
  is_service_user = false
  role            = "user"
  auto_groups     = []
}

data "netbird_user" "%[1]s" {
  id    = netbird_user.%[1]s.id
  email = "%[1]s-other@example.com"
}
`, rName), noMatch)
	})

	// The agent network data sources take a different path for a known ID: they
	// fetch it directly rather than listing, and check the name afterwards. That
	// is a separate comparison and needs its own case.
	t.Run("agent network policy id and name", func(t *testing.T) {
		rName := "anp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		rejects(t, testAgentNetworkPolicyResource(rName)+fmt.Sprintf(`
data "netbird_agent_network_policy" "%[1]s" {
  id   = netbird_agent_network_policy.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName), noMatch)
	})
}

// Test_DataSource_RejectsAmbiguousMatch is the case that protects a user from
// silently getting whichever object came first.
//
// It needs two objects one filter can match, which most of these resources will
// not allow: names are unique per account for groups, networks and policies. A
// setup key's name is not, so this is where the path can be exercised at all.
func Test_DataSource_RejectsAmbiguousMatch(t *testing.T) {
	testE2E(t)
	rName := "amb" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := fmt.Sprintf(`
resource "netbird_setup_key" "%[1]sa" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
  auto_groups    = []
}

resource "netbird_setup_key" "%[1]sb" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
  auto_groups    = []
}

data "netbird_setup_key" "%[1]s" {
  name       = "%[1]s"
  depends_on = [netbird_setup_key.%[1]sa, netbird_setup_key.%[1]sb]
}
`, rName)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The two keys have to exist before the data source reads, and
				// the read has to fail, so the objects are created in a step of
				// their own first.
				Config: fmt.Sprintf(`
resource "netbird_setup_key" "%[1]sa" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
  auto_groups    = []
}

resource "netbird_setup_key" "%[1]sb" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
  auto_groups    = []
}
`, rName),
			},
			{
				Config:      cfg,
				ExpectError: ambiguous,
			},
		},
	})
}

// Test_DataSource_RejectsNoSelectorWithoutAGuard is the other half of the
// no-selector story.
//
// Five data sources have no explicit guard: they list and filter, and a
// configuration that names nothing matches nothing, so the answer is the
// not-found error rather than a message naming the selectors. The outcome is
// safe either way — no object is returned — but the message is worse, and a
// test that pins it is how anyone notices if that changes.
//
// The reverse proxy pair is left out: their lists need a running cluster.
func Test_DataSource_RejectsNoSelectorWithoutAGuard(t *testing.T) {
	testE2E(t)

	t.Run("dns_zone", func(t *testing.T) {
		rejects(t, "data \"netbird_dns_zone\" \"nothing\" {}\n", noMatch)
	})

	t.Run("peer", func(t *testing.T) {
		rejects(t, "data \"netbird_peer\" \"nothing\" {}\n", noMatch)
	})

	t.Run("dns_record", func(t *testing.T) {
		rName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		rejects(t, testDNSZoneResource(rName, rName+".local", true, false,
			fmt.Sprintf("[%q]", e2eGroupNotAllID()))+fmt.Sprintf(`
data "netbird_dns_record" "nothing" {
  zone_id = netbird_dns_zone.%s.id
}
`, rName), noMatch)
	})
}

// Test_DataSource_RejectsNoMatchOnTheRest covers the data sources whose
// not-found path needs a parent or a fixture, so they do not fit the table in
// Test_DataSource_RejectsNoMatch.
func Test_DataSource_RejectsNoMatchOnTheRest(t *testing.T) {
	testE2E(t)
	gone := "does-not-exist-" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)

	t.Run("dns_zone", func(t *testing.T) {
		rejects(t, fmt.Sprintf("data \"netbird_dns_zone\" \"missing\" {\n  name = %q\n}\n", gone), noMatch)
	})

	t.Run("peer", func(t *testing.T) {
		rejects(t, fmt.Sprintf("data \"netbird_peer\" \"missing\" {\n  name = %q\n}\n", gone), noMatch)
	})

	t.Run("dns_record", func(t *testing.T) {
		rName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		rejects(t, testDNSZoneResource(rName, rName+".local", true, false,
			fmt.Sprintf("[%q]", e2eGroupNotAllID()))+fmt.Sprintf(`
data "netbird_dns_record" "missing" {
  zone_id = netbird_dns_zone.%s.id
  name    = %q
}
`, rName, gone), noMatch)
	})

	t.Run("network_resource", func(t *testing.T) {
		rejects(t, fmt.Sprintf(`
data "netbird_network_resource" "missing" {
  network_id = %q
  name       = %q
}
`, e2eNetworkID(), gone), noMatch)
	})

	t.Run("token", func(t *testing.T) {
		rejects(t, fmt.Sprintf(`
data "netbird_token" "missing" {
  user_id = %q
  name    = %q
}
`, mustE2E().UserID, gone), noMatch)
	})
}

// Test_DataSource_RejectsDisagreeingSelectorsOnTheRest is the same case as
// above for every remaining data source, in a table because the configuration
// is the only thing that differs: create an object, then ask for it by its ID
// and by a second selector that names something else.
//
// The two cloud-only data sources and the reverse proxy pair are left out —
// nothing on a self-hosted deployment can create the objects.
func Test_DataSource_RejectsDisagreeingSelectorsOnTheRest(t *testing.T) {
	testE2E(t)
	rName := "dis" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	notAll := fmt.Sprintf("[%q]", e2eGroupNotAllID())

	for _, c := range []struct {
		name   string
		config string
	}{
		{"network", testNetworkResource(rName, `Test`) + fmt.Sprintf(`
data "netbird_network" "d" {
  id   = netbird_network.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		{"setup_key", testSetupKeyResourceNoLimit(rName, `reusable`) + fmt.Sprintf(`
data "netbird_setup_key" "d" {
  id   = netbird_setup_key.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		// description is set deliberately: leaving it out fails the apply
		// outright, because the attribute is Optional without being Computed and
		// the API answers with an empty string rather than nothing. That is a
		// defect of its own, not this case's subject.
		{"posture_check", fmt.Sprintf(`resource "netbird_posture_check" "%[1]s" {
  name        = "%[1]s"
  description = "selector test"

  netbird_version_check {
    min_version = "0.40.0"
  }
}

data "netbird_posture_check" "d" {
  id   = netbird_posture_check.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		{"nameserver_group", testNameserverGroupResource(rName, `1.1.1.1`, `53`, notAll) + fmt.Sprintf(`
data "netbird_nameserver_group" "d" {
  id   = netbird_nameserver_group.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		{"policy", testPolicyResourceGroups(rName, rName, "desc", "accept", "udp",
			e2eGroupAllID(), e2eGroupNotAllID(), "443") + fmt.Sprintf(`
data "netbird_policy" "d" {
  id   = netbird_policy.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		// A route's second selector is its network_id rather than a name.
		{"route", testRouteResource(rName, e2eGroupAllID(), `null`, `desc`, `null`,
			`["example.com"]`, notAll, `null`) + fmt.Sprintf(`
data "netbird_route" "d" {
  id         = netbird_route.%[1]s.id
  network_id = "%[1]s-not-the-network"
}
`, rName)},
		{"agent_network_guardrail", testAgentNetworkGuardrailResource(rName) + fmt.Sprintf(`
data "netbird_agent_network_guardrail" "d" {
  id   = netbird_agent_network_guardrail.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		{"agent_network_provider", testAgentNetworkProviderResource(rName, rName, `null`, "false") +
			fmt.Sprintf(`
data "netbird_agent_network_provider" "d" {
  id   = netbird_agent_network_provider.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		{"dns_zone", testDNSZoneResource(rName, rName+".local", true, false, notAll) + fmt.Sprintf(`
data "netbird_dns_zone" "d" {
  id   = netbird_dns_zone.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName)},
		{"dns_record", testDNSRecordResource(rName, rName+".local", rName+"rec", "www",
			"A", "10.0.0.1", 300) + fmt.Sprintf(`
data "netbird_dns_record" "d" {
  zone_id = netbird_dns_zone.%[1]s.id
  id      = netbird_dns_record.%[1]srec.id
  name    = "not-the-name.%[1]s.local"
}
`, rName)},
		{"network_resource", testNetworkResourceResource(rName, e2eNetworkID(), "example.com",
			notAll, rName) + fmt.Sprintf(`
data "netbird_network_resource" "d" {
  network_id = %[2]q
  id         = netbird_network_resource.%[1]s.id
  name       = "%[1]s-not-the-name"
}
`, rName, e2eNetworkID())},
		{"token", testTokenResource(rName, mustE2E().UserID, `90`) + fmt.Sprintf(`
data "netbird_token" "d" {
  user_id = %[2]q
  id      = netbird_token.%[1]s.id
  name    = "%[1]s-not-the-name"
}
`, rName, mustE2E().UserID)},
		// A peer needs no resource: the fixture is already registered.
		{"peer", fmt.Sprintf(`
data "netbird_peer" "d" {
  id   = %q
  name = "not-the-hostname"
}
`, testPeerID(t, "peer1"))},
	} {
		t.Run(c.name, func(t *testing.T) {
			rejects(t, c.config, noMatch)
		})
	}
}

// Test_DataSource_TokenAmbiguousMatch is the second place the ambiguous path can
// be reached: two tokens for the same user can carry the same name.
func Test_DataSource_TokenAmbiguousMatch(t *testing.T) {
	testE2E(t)
	rName := "amb" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	userID := mustE2E().UserID
	both := fmt.Sprintf(`
resource "netbird_token" "%[1]sa" {
  user_id         = %[2]q
  name            = "%[1]s"
  expiration_days = 90
}

resource "netbird_token" "%[1]sb" {
  user_id         = %[2]q
  name            = "%[1]s"
  expiration_days = 90
}
`, rName, userID)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: both},
			{
				Config: both + fmt.Sprintf(`
data "netbird_token" "%[1]s" {
  user_id = %[2]q
  name    = "%[1]s"
}
`, rName, userID),
				ExpectError: ambiguous,
			},
		},
	})
}

// The by-ID read for the two DNS data sources and for the account settings
// singleton, compared against the resource that produced them. The existing
// tests for these three select by name, or assert literals, so the pairing was
// never made.
func Test_DNSZone_DataSourceByID(t *testing.T) {
	testE2E(t)
	rName := "dz" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testDNSZoneResource(rName, rName+".local", true, false,
		fmt.Sprintf("[%q]", e2eGroupNotAllID())) + dataSourceByID("dns_zone", rName)
	dsCase(t, cfg, samePair("dns_zone", rName, "name", "domain", "enabled",
		"enable_search_domain", "distribution_groups.#"))
}

func Test_DNSRecord_DataSourceByID(t *testing.T) {
	testE2E(t)
	rName := "dr" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testDNSRecordResource(rName, rName+".local", rName, "www", "A", "10.0.0.1", 300) +
		fmt.Sprintf(`
data "netbird_dns_record" "%[1]s" {
  zone_id = netbird_dns_zone.%[1]s.id
  id      = netbird_dns_record.%[1]s.id
}
`, rName)
	dsCase(t, cfg, samePair("dns_record", rName, "name", "type", "content", "ttl", "zone_id"))
}

func Test_AccountSettings_DataSourcePairs(t *testing.T) {
	testE2E(t)
	rName := "as" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testAccountResource(rName) + fmt.Sprintf(`
data "netbird_account_settings" "%[1]s" {
  depends_on = [netbird_account_settings.%[1]s]
}
`, rName)
	dsCase(t, cfg, samePair("account_settings", rName, "peer_login_expiration",
		"peer_inactivity_expiration", "peer_login_expiration_enabled",
		"regular_users_view_blocked", "groups_propagation_enabled", "jwt_groups_enabled"))
}

// The identity provider data source rounds out the same three cases. The
// resource needs egress — the provider fetches the issuer's discovery document
// before the server accepts it — which is why these sit apart from the tables
// above rather than inside them.
func Test_DataSource_IdentityProviderSelectors(t *testing.T) {
	testE2E(t)
	const issuer = "https://oauth.id.jumpcloud.com/"

	t.Run("by name", func(t *testing.T) {
		rName := "ip" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		cfg := testIdentityProviderResource(rName, rName, "oidc", "client-id", "client-secret", issuer) +
			fmt.Sprintf(`
data "netbird_identity_provider" "%[1]s" {
  name = netbird_identity_provider.%[1]s.name
}
`, rName)
		dsCase(t, cfg, samePair("identity_provider", rName, "id", "name", "type", "issuer"))
	})

	t.Run("no match", func(t *testing.T) {
		rejects(t, fmt.Sprintf(`
data "netbird_identity_provider" "missing" {
  name = "does-not-exist-%s"
}
`, acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)), noMatch)
	})

	t.Run("disagreeing selectors", func(t *testing.T) {
		rName := "ip" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
		rejects(t, testIdentityProviderResource(rName, rName, "oidc", "client-id", "client-secret", issuer)+
			fmt.Sprintf(`
data "netbird_identity_provider" "d" {
  id   = netbird_identity_provider.%[1]s.id
  name = "%[1]s-not-the-name"
}
`, rName), noMatch)
	})
}
