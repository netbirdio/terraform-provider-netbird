//go:build e2e

// What happens when an import ID is wrong.
//
// Twenty-three resources support import and the suite only ever handed them an
// ID that works. Two things go untested that way: the four composite IDs, whose
// format is parsed by hand in each resource and whose error message is the only
// documentation a user gets; and the ordinary case of an ID that is well-formed
// but names nothing, where the resource has to report that rather than write a
// half-empty object into state.
//
// None of these create anything. An import step stands on its own — Terraform
// never applies the configuration beside it — so the configurations here exist
// only to name the resource type, and the composite-ID cases are answered
// before a request is made at all.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// gone is Terraform's own message for an import that resolved to nothing. It is
// the correct outcome for every resource whose Read recognises a 404 and drops
// the object instead of returning an error.
var gone = regexp.MustCompile(`(?s)Cannot import non-existent remote object`)

// badFormat matches the message each hand-written composite parser produces.
func badFormat(want string) *regexp.Regexp {
	return regexp.MustCompile(`(?s)Invalid [Ii]mport ID|Error importing|` + regexp.QuoteMeta(want))
}

func Test_Import_RejectsWrongIDs(t *testing.T) {
	testE2E(t)
	n := "i" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	cases := []struct {
		name    string
		address string
		config  string
		id      string
		want    *regexp.Regexp
	}{
		// The four composite IDs, each parsed by hand in its own resource.
		{
			name:    "dns_record without the colon",
			address: "netbird_dns_record." + n,
			id:      "onlyonepart",
			want:    badFormat("zone_id:record_id"),
			config: fmt.Sprintf(`
resource "netbird_dns_record" "%[1]s" {
  zone_id = "z"
  name    = "www.example.local"
  type    = "A"
  content = "10.0.0.1"
}`, n),
		},
		{
			name:    "network_resource without the slash",
			address: "netbird_network_resource." + n,
			id:      "onlyonepart",
			want:    badFormat("networkID/networkResourceID"),
			config: fmt.Sprintf(`
resource "netbird_network_resource" "%[1]s" {
  network_id = "n"
  name       = "%[1]s"
  address    = "example.com"
  groups     = ["g"]
}`, n),
		},
		{
			name:    "network_router without the slash",
			address: "netbird_network_router." + n,
			id:      "onlyonepart",
			want:    badFormat("networkID/networkRouterID"),
			config: fmt.Sprintf(`
resource "netbird_network_router" "%[1]s" {
  network_id = "n"
}`, n),
		},
		{
			name:    "token without the slash",
			address: "netbird_token." + n,
			id:      "onlyonepart",
			want:    badFormat("userID/tokenID"),
			config: fmt.Sprintf(`
resource "netbird_token" "%[1]s" {
  name            = "%[1]s"
  user_id         = "u"
  expiration_days = 30
}`, n),
		},

		// Well-formed IDs that name nothing. These do reach the server.
		{
			name:    "dns_zone that does not exist",
			address: "netbird_dns_zone." + n,
			id:      "nosuchzone",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_dns_zone" "%[1]s" {
  name   = "%[1]s"
  domain = "%[1]s.local"
}`, n),
		},
		{
			name:    "group that does not exist",
			address: "netbird_group." + n,
			id:      "nosuchgroup",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_group" "%[1]s" {
  name  = "%[1]s"
  peers = []
}`, n),
		},
		{
			name:    "nameserver_group that does not exist",
			address: "netbird_nameserver_group." + n,
			id:      "nosuchns",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_nameserver_group" "%[1]s" {
  name    = "%[1]s"
  groups  = [%[2]q]
  enabled = true

  nameservers = [{
    ip      = "1.1.1.1"
    ns_type = "udp"
    port    = 53
  }]
}`, n, e2eGroupNotAllID()),
		},
		{
			name:    "network that does not exist",
			address: "netbird_network." + n,
			id:      "nosuchnetwork",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_network" "%[1]s" {
  name = "%[1]s"
}`, n),
		},
		{
			name:    "peer that does not exist",
			address: "netbird_peer." + n,
			id:      "nosuchpeer",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_peer" "%[1]s" {
  id = "nosuchpeer"
}`, n),
		},
		{
			name:    "policy that does not exist",
			address: "netbird_policy." + n,
			id:      "nosuchpolicy",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_policy" "%[1]s" {
  name    = "%[1]s"
  enabled = true

  rule {
    name          = "%[1]s"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    sources       = [%[2]q]
    destinations  = [%[3]q]
    ports         = ["443"]
  }
}`, n, e2eGroupNotAllID(), e2eGroupAllID()),
		},
		{
			name:    "posture_check that does not exist",
			address: "netbird_posture_check." + n,
			id:      "nosuchcheck",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_posture_check" "%[1]s" {
  name = "%[1]s"

  netbird_version_check {
    min_version = "0.40.0"
  }
}`, n),
		},
		{
			name:    "route that does not exist",
			address: "netbird_route." + n,
			id:      "nosuchroute",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_route" "%[1]s" {
  network_id = "%[1]s"
  groups     = [%[2]q]
  domains    = ["example.com"]
}`, n, e2eGroupNotAllID()),
		},
		{
			name:    "setup_key that does not exist",
			address: "netbird_setup_key." + n,
			id:      "nosuchkey",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
}`, n),
		},
		{
			name:    "user that does not exist",
			address: "netbird_user." + n,
			id:      "nosuchuser",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_user" "%[1]s" {
  name            = "%[1]s"
  is_service_user = true
  role            = "user"
  auto_groups     = []
}`, n),
		},
		{
			name:    "agent_network_provider that does not exist",
			address: "netbird_agent_network_provider." + n,
			id:      "nosuchprovider",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_agent_network_provider" "%[1]s" {
  provider_id  = "%[1]s"
  name         = "%[1]s"
  upstream_url = "https://example.invalid"
  api_key      = "k"
}`, n),
		},
		{
			name:    "reverse_proxy_domain that does not exist",
			address: "netbird_reverse_proxy_domain." + n,
			id:      "nosuchdomain",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_reverse_proxy_domain" "%[1]s" {
  domain         = "%[1]s.example.com"
  target_cluster = "c"
}`, n),
		},
		{
			name:    "reverse_proxy_service that does not exist",
			address: "netbird_reverse_proxy_service." + n,
			id:      "nosuchservice",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%[1]s" {
  name    = "%[1]s"
  domain  = "%[1]s.example.com"
  enabled = true

  targets = [{
    target_id   = "t"
    target_type = "peer"
    port        = 8080
    protocol    = "http"
  }]

  auth = {
    link_auth = {
      enabled = true
    }
  }
}`, n),
		},
		{
			name:    "identity_provider that does not exist",
			address: "netbird_identity_provider." + n,
			id:      "nosuchidp",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_identity_provider" "%[1]s" {
  name          = "%[1]s"
  type          = "oidc"
  client_id     = "id"
  client_secret = "secret"
  issuer        = "https://oauth.id.jumpcloud.com/"
}`, n),
		},
		{
			name:    "agent_network_guardrail that does not exist",
			address: "netbird_agent_network_guardrail." + n,
			id:      "nosuchguardrail",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_agent_network_guardrail" "%[1]s" {
  name = "%[1]s"

  model_allowlist = {
    enabled = true
    models  = ["gpt-4.1"]
  }

  prompt_capture = {
    enabled = false
  }
}`, n),
		},
		{
			name:    "agent_network_policy that does not exist",
			address: "netbird_agent_network_policy." + n,
			id:      "nosuchagentpolicy",
			want:    gone,
			config: fmt.Sprintf(`
resource "netbird_agent_network_policy" "%[1]s" {
  name                     = "%[1]s"
  source_groups            = [%[2]q]
  destination_provider_ids = ["whatever"]
}`, n, e2eGroupNotAllID()),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testEnsureManagementRunning(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:        c.config,
						ResourceName:  c.address,
						ImportState:   true,
						ImportStateId: c.id,
						ExpectError:   c.want,
					},
				},
			})
		})
	}
}

// Test_Import_AccountSettingsTakesAnyID records something the table above
// cannot: the account settings resource accepts whatever ID it is handed.
//
// Its Read fetches the account the token belongs to and ignores the ID
// entirely, so there is no such thing as an account settings object that does
// not exist, and no ID that can be wrong. That is reasonable for a singleton —
// there is exactly one account to bind to — but it means an import typo is
// silently accepted rather than reported, which is worth knowing before someone
// spends an afternoon on it.
func Test_Import_AccountSettingsTakesAnyID(t *testing.T) {
	testE2E(t)
	n := "i" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_account_settings." + n
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_account_settings" "%[1]s" {
  peer_login_expiration_enabled = true
}`, n),
				ResourceName:  address,
				ImportState:   true,
				ImportStateId: "not-an-account-id",
				Check: resource.TestCheckResourceAttr(address,
					"id", mustE2E().AccountID),
			},
		},
	})
}
