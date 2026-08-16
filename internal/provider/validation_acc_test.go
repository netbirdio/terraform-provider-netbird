//go:build e2e

// The validators the suite did not exercise.
//
// A validator that never fires in a test is a rejection nobody has confirmed
// happens. lifecycle_acc_test.go covers seven of them, all string enumerations;
// this file covers the rest — the numeric ranges, the collection sizes, and the
// enumerations on resources that need a parent object to exist before they can
// be written at all.
//
// Every case here is refused before a request is made, so none of them creates
// anything, none of them needs a peer or a proxy cluster, and the whole file
// runs in about a second. The configurations are written out in full rather
// than built from the shared helpers: a rejection test has to control exactly
// one value, and a helper that gains an argument later would quietly change
// what is being rejected.

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
)

// The shapes terraform-plugin-framework produces for the validators used here.
var (
	outOfRange = regexp.MustCompile(`(?s)value must be between|Invalid Attribute Value`)
	wrongSize  = regexp.MustCompile(`(?s)list must contain|Invalid Attribute Value|Insufficient|Too few`)
	tooLong    = regexp.MustCompile(`(?s)length must be|Invalid Attribute Value`)
)

func Test_Rejects_OutsideValidatorLimits(t *testing.T) {
	testE2E(t)
	rName := "v" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	group := e2eGroupNotAllID()

	cases := []struct {
		name   string
		config string
		want   *regexp.Regexp
	}{
		{
			// 60 seconds is the shortest TTL the resource accepts.
			name: "dns_record ttl below the range",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_dns_zone" "%[1]s" {
  name   = "%[1]s"
  domain = "reject.local"
}

resource "netbird_dns_record" "%[1]s" {
  zone_id = netbird_dns_zone.%[1]s.id
  name    = "www"
  type    = "A"
  content = "10.0.0.1"
  ttl     = 30
}`, rName),
		},
		{
			name: "nameserver_group name longer than 40",
			want: tooLong,
			config: fmt.Sprintf(`
resource "netbird_nameserver_group" "%[1]s" {
  name        = %[2]q
  description = "rejection fixture"
  enabled     = true
  groups      = [%[3]q]

  nameservers = [{
    ip      = "1.1.1.1"
    ns_type = "udp"
    port    = 53
  }]
}`, rName, strings.Repeat("n", 41), group),
		},
		{
			// The metric decides which of two routes to the same network wins,
			// and 0 is not a position in that ordering.
			name: "network_router metric below the range",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_network_router" "%[1]s" {
  network_id  = %[2]q
  peer_groups = [%[3]q]
  masquerade  = true
  metric      = 0
}`, rName, e2eNetworkID(), group),
		},
		{
			// The schema allows exactly one rule. Two is the bound that can be
			// checked before an apply: an absent block reaches the server
			// instead, which is a gap in the validator rather than in the test.
			name: "policy with two rules",
			want: wrongSize,
			config: fmt.Sprintf(`
resource "netbird_policy" "%[1]s" {
  name        = "%[1]s"
  description = "rejection fixture"
  enabled     = true

  rule {
    name          = "%[1]s-a"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    sources       = [%[2]q]
    destinations  = [%[3]q]
    ports         = ["443"]
  }

  rule {
    name          = "%[1]s-b"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "udp"
    sources       = [%[2]q]
    destinations  = [%[3]q]
    ports         = ["53"]
  }
}`, rName, group, e2eGroupAllID()),
		},
		{
			name: "policy port range starting above 65535",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_policy" "%[1]s" {
  name        = "%[1]s"
  description = "rejection fixture"
  enabled     = true

  rule {
    name          = "%[1]s"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    sources       = [%[2]q]
    destinations  = [%[3]q]

    port_ranges = [{
      start = 70000
      end   = 70001
    }]
  }
}`, rName, group, e2eGroupAllID()),
		},
		{
			name: "policy port range ending above 65535",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_policy" "%[1]s" {
  name        = "%[1]s"
  description = "rejection fixture"
  enabled     = true

  rule {
    name          = "%[1]s"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    sources       = [%[2]q]
    destinations  = [%[3]q]

    port_ranges = [{
      start = 80
      end   = 70000
    }]
  }
}`, rName, group, e2eGroupAllID()),
		},
		{
			name: "reverse_proxy_service unknown target type",
			want: oneOf,
			config: fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%[1]s" {
  name    = "%[1]s"
  domain  = "%[1]s.reject.local"
  enabled = true

  targets = [{
    target_id   = "whatever"
    target_type = "vm"
    port        = 8080
    protocol    = "http"
  }]
  auth = {
    link_auth = {
      enabled = true
    }
  }
}`, rName),
		},
		{
			name: "reverse_proxy_service port above 65535",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%[1]s" {
  name    = "%[1]s"
  domain  = "%[1]s.reject.local"
  enabled = true

  targets = [{
    target_id   = "whatever"
    target_type = "peer"
    port        = 70000
    protocol    = "http"
  }]
  auth = {
    link_auth = {
      enabled = true
    }
  }
}`, rName),
		},
		{
			// The proxy terminates HTTP and HTTPS. Anything else would be a
			// different product.
			name: "reverse_proxy_service unknown protocol",
			want: oneOf,
			config: fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%[1]s" {
  name    = "%[1]s"
  domain  = "%[1]s.reject.local"
  enabled = true

  targets = [{
    target_id   = "whatever"
    target_type = "peer"
    port        = 8080
    protocol    = "ftp"
  }]
  auth = {
    link_auth = {
      enabled = true
    }
  }
}`, rName),
		},
		{
			name: "route network_id longer than 40",
			want: tooLong,
			config: fmt.Sprintf(`
resource "netbird_route" "%[1]s" {
  network_id  = %[2]q
  description = "rejection fixture"
  groups      = [%[3]q]
  domains     = ["example.com"]
}`, rName, strings.Repeat("n", 41), group),
		},
		{
			name: "route metric below the range",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_route" "%[1]s" {
  network_id  = "%[1]s"
  description = "rejection fixture"
  groups      = [%[2]q]
  domains     = ["example.com"]
  metric      = 0
}`, rName, group),
		},
		{
			// A year is the longest a personal access token may live.
			name: "token expiration beyond a year",
			want: outOfRange,
			config: fmt.Sprintf(`
resource "netbird_token" "%[1]s" {
  name            = "%[1]s"
  user_id         = %[2]q
  expiration_days = 400
}`, rName, mustE2E().UserID),
		},
		{
			// Not a validator but the same contract: the combination cannot be
			// satisfied, so it is refused before the apply rather than after.
			name: "one-off setup key with a usage limit it cannot honour",
			want: regexp.MustCompile(`(?s)Invalid Attribute Combination|one-off setup key can be used once`),
			config: fmt.Sprintf(`
resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "one-off"
  usage_limit    = 5
}`, rName),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rejects(t, c.config, c.want)
		})
	}
}
