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
