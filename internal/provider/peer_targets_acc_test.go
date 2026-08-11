// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// API surface that only opens up once the account has real peers: a router or a
// route bound to a single peer rather than to peer groups, the peer data source,
// and reverse proxy services whose targets are peers or network resources.
//
// Every check reads the object back from the management API. Comparing Terraform
// state against Terraform state would only prove the provider is
// self-consistent; what matters is that what the provider sent is what
// management stored, and that what management reports is what the provider
// surfaces.

// A router can be pinned to one peer instead of to peer groups. The two are
// mutually exclusive, and the peer form had no coverage at all.
func Test_NetworkRouter_PeerInsteadOfPeerGroups(t *testing.T) {
	testE2E(t)
	peer1 := testPeerID(t, "peer1")
	peer2 := testPeerID(t, "peer2")

	rName := "nro" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_network_router." + rName

	config := func(peerID string, metric int) string {
		return fmt.Sprintf(`
resource "netbird_network" %[1]q {
  name = %[1]q
}

resource "netbird_network_router" %[1]q {
  network_id = netbird_network.%[1]s.id
  peer       = %[2]q
  metric     = %[3]d
  masquerade = true
  enabled    = true
}`, rName, peerID, metric)
	}

	// Reads the router back from management and checks the peer binding is what
	// the configuration asked for, with no peer_groups left behind.
	checkAPI := func(wantPeer string, wantMetric int) func(*terraform.State) error {
		return func(s *terraform.State) error {
			attrs := s.RootModule().Resources[addr].Primary.Attributes
			router, err := testClient().Networks.Routers(attrs["network_id"]).Get(context.Background(), attrs["id"])
			if err != nil {
				return err
			}
			if router.Peer == nil || *router.Peer != wantPeer {
				return fmt.Errorf("router peer mismatch: expected %s, management reports %v", wantPeer, router.Peer)
			}
			if router.PeerGroups != nil && len(*router.PeerGroups) != 0 {
				return fmt.Errorf("router bound to a peer should carry no peer_groups, management reports %v", *router.PeerGroups)
			}
			if router.Metric != wantMetric {
				return fmt.Errorf("router metric mismatch: expected %d, management reports %d", wantMetric, router.Metric)
			}
			if !router.Masquerade {
				return fmt.Errorf("masquerade not persisted on the management server")
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(peer1, 100),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "peer", peer1),
					resource.TestCheckResourceAttr(addr, "peer_groups.#", "0"),
					checkAPI(peer1, 100),
				),
			},
			{
				// Moving the router to another peer must update in place.
				Config: config(peer2, 200),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "peer", peer2),
					checkAPI(peer2, 200),
				),
			},
		},
	})
}

// peer and peer_groups conflict in the schema, so asking for both must be
// rejected at plan time rather than sent to the API.
func Test_NetworkRouter_PeerAndPeerGroupsConflict(t *testing.T) {
	testE2E(t)
	peer1 := testPeerID(t, "peer1")
	rName := "nro" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)

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
  peer        = %[2]q
  peer_groups = [%[3]q]
}`, rName, peer1, e2eGroupNotAllID()),
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination|cannot be specified when`),
			},
		},
	})
}

// A route can also be pinned to a single peer. The existing update test reaches
// that path; this covers creating one that way, and the distribution and access
// control groups alongside it.
func Test_Route_CreatedWithPeer(t *testing.T) {
	testE2E(t)
	peer1 := testPeerID(t, "peer1")
	rName := "rt" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_route." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_route" %[1]q {
  network_id            = %[1]q
  peer                  = %[2]q
  network               = "100.20.0.0/16"
  description           = "routed through one peer"
  groups                = [%[3]q]
  access_control_groups = [%[4]q]
  metric                = 500
  masquerade            = true
  enabled               = true
}`, rName, peer1, e2eGroupAllID(), e2eGroupNotAllID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "peer", peer1),
					resource.TestCheckResourceAttr(addr, "peer_groups.#", "0"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						route, err := testClient().Routes.Get(context.Background(), id)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"peer":                    {peer1, route.Peer},
							"network":                 {"100.20.0.0/16", route.Network},
							"description":             {"routed through one peer", route.Description},
							"metric":                  {int(500), route.Metric},
							"masquerade":              {true, route.Masquerade},
							"groups.#":                {int(1), len(route.Groups)},
							"groups.0":                {e2eGroupAllID(), route.Groups[0]},
							"access_control_groups.#": {int(1), len(*route.AccessControlGroups)},
							"access_control_groups.0": {e2eGroupNotAllID(), (*route.AccessControlGroups)[0]},
						})
					},
				),
			},
		},
	})
}

// The peer data source resolves by id, by name and by ip. Each lookup must
// return the peer management holds, with the whole attribute surface matching
// what the API reports rather than merely being set.
func Test_Peer_DataSource_AllLookupsMatchAPI(t *testing.T) {
	testE2E(t)
	peerID := testPeerID(t, "peer1")

	peer, err := testClient().Peers.Get(context.Background(), peerID)
	if err != nil {
		t.Fatalf("read the reference peer: %v", err)
	}

	rName := "p" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	byID := "data.netbird_peer." + rName + "_by_id"
	byName := "data.netbird_peer." + rName + "_by_name"
	byIP := "data.netbird_peer." + rName + "_by_ip"

	// Every attribute the data source exposes that management also reports, so a
	// field the provider stops mapping shows up here.
	//
	// The peer is read back inside the check rather than compared against the
	// snapshot above: a live agent changes connection_ip, version, ui_version
	// and its geolocation fields whenever it reconnects, so a snapshot taken
	// before the apply would fail for reasons that have nothing to do with the
	// provider.
	sameAsAPI := func(addr string) resource.TestCheckFunc {
		return checkAttrsMatchAPI(addr, func(attrs map[string]string) (map[string]string, error) {
			current, err := testClient().Peers.Get(context.Background(), attrs["id"])
			if err != nil {
				return nil, err
			}
			return map[string]string{
				"id":                            current.Id,
				"name":                          current.Name,
				"hostname":                      current.Hostname,
				"ip":                            current.Ip,
				"dns_label":                     current.DnsLabel,
				"os":                            current.Os,
				"version":                       current.Version,
				"kernel_version":                current.KernelVersion,
				"user_id":                       current.UserId,
				"ssh_enabled":                   fmt.Sprint(current.SshEnabled),
				"login_expiration_enabled":      fmt.Sprint(current.LoginExpirationEnabled),
				"login_expired":                 fmt.Sprint(current.LoginExpired),
				"approval_required":             fmt.Sprint(current.ApprovalRequired),
				"inactivity_expiration_enabled": fmt.Sprint(current.InactivityExpirationEnabled),
				"connection_ip":                 current.ConnectionIp,
				"serial_number":                 current.SerialNumber,
				"country_code":                  current.CountryCode,
				"city_name":                     current.CityName,
				"geoname_id":                    fmt.Sprint(current.GeonameId),
				"ui_version":                    current.UiVersion,
				"groups.#":                      apiListCount(current.Groups),
			}, nil
		})
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "netbird_peer" "%[1]s_by_id"   { id   = %[2]q }
data "netbird_peer" "%[1]s_by_name" { name = %[3]q }
data "netbird_peer" "%[1]s_by_ip"   { ip   = %[4]q }`,
					rName, peer.Id, peer.Name, peer.Ip),
				Check: resource.ComposeAggregateTestCheckFunc(
					sameAsAPI(byID),
					sameAsAPI(byName),
					sameAsAPI(byIP),
				),
			},
		},
	})
}

// Services can target network resources — host, subnet or domain — as well as
// peers. Only the peer form was covered, so the resource forms are the ones that
// could silently break.
func Test_ReverseProxyService_ResourceTargets(t *testing.T) {
	env := testE2E(t)
	cluster := testRequireProxyCluster(t)

	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + "." + cluster.Address
	addr := "netbird_reverse_proxy_service." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_reverse_proxy_service" %[1]q {
  name   = %[1]q
  domain = %[2]q

  targets = [
    {
      target_id   = %[3]q
      target_type = "host"
      port        = 8080
      protocol    = "http"
    },
    {
      # A subnet has no single address, so the host to route to must be given
      # explicitly. peer, host and domain targets have theirs resolved by the
      # server from the peer or resource.
      target_id   = %[4]q
      target_type = "subnet"
      host        = "192.168.10.5"
      port        = 8081
      protocol    = "http"
    },
    {
      target_id   = %[5]q
      target_type = "domain"
      port        = 8082
      protocol    = "https"
    },
  ]

  auth = {
    password_auth = {
      enabled  = true
      password = "resourcetargets"
    }
  }
}`, rName, domain, env.ResourceHostID, env.ResourceSubnetID, env.ResourceDomainID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "targets.#", "3"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return err
						}
						if svc.Domain != domain {
							return fmt.Errorf("domain mismatch: expected %s, management reports %s", domain, svc.Domain)
						}
						if len(svc.Targets) != 3 {
							return fmt.Errorf("expected 3 targets on the management server, found %d", len(svc.Targets))
						}
						// The API is free to reorder targets, so compare as a set
						// of type/id/port triples.
						want := map[string]bool{
							"host:" + env.ResourceHostID + ":8080":     true,
							"subnet:" + env.ResourceSubnetID + ":8081": true,
							"domain:" + env.ResourceDomainID + ":8082": true,
						}
						for _, tgt := range svc.Targets {
							key := fmt.Sprintf("%s:%s:%d", tgt.TargetType, tgt.TargetId, tgt.Port)
							if !want[key] {
								return fmt.Errorf("unexpected target %s on the management server", key)
							}
							delete(want, key)
						}
						if len(want) != 0 {
							return fmt.Errorf("targets missing from the management server: %v", want)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"auth.password_auth.password"},
			},
		},
	})
}

// A service mixing a peer target with a network-resource target, which is the
// shape a real deployment tends to have.
func Test_ReverseProxyService_MixedPeerAndResourceTargets(t *testing.T) {
	env := testE2E(t)
	cluster := testRequireProxyCluster(t)
	peerID := testPeerID(t, "peer1")

	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + "." + cluster.Address
	addr := "netbird_reverse_proxy_service." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_reverse_proxy_service" %[1]q {
  name   = %[1]q
  domain = %[2]q

  targets = [
    {
      target_id   = %[3]q
      target_type = "peer"
      port        = 8080
      protocol    = "http"
      path        = "/peer"
    },
    {
      target_id   = %[4]q
      target_type = "subnet"
      host        = "192.168.10.5"
      port        = 8081
      protocol    = "http"
      path        = "/subnet"
    },
  ]

  auth = {
    pin_auth = {
      enabled = true
      pin     = "4242"
    }
  }
}`, rName, domain, peerID, env.ResourceSubnetID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "targets.#", "2"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return err
						}
						byType := map[string]string{}
						paths := map[string]string{}
						for _, tgt := range svc.Targets {
							byType[string(tgt.TargetType)] = tgt.TargetId
							paths[string(tgt.TargetType)] = valOr(tgt.Path, "")
						}
						if byType["peer"] != peerID {
							return fmt.Errorf("peer target mismatch: expected %s, management reports %q", peerID, byType["peer"])
						}
						if byType["subnet"] != env.ResourceSubnetID {
							return fmt.Errorf("subnet target mismatch: expected %s, management reports %q", env.ResourceSubnetID, byType["subnet"])
						}
						if paths["peer"] != "/peer" || paths["subnet"] != "/subnet" {
							return fmt.Errorf("per-target paths not persisted, management reports %v", paths)
						}
						if svc.Auth.PinAuth == nil || !svc.Auth.PinAuth.Enabled {
							return fmt.Errorf("pin auth not enabled on the management server")
						}
						return nil
					},
				),
			},
		},
	})
}

// The clusters data source is only meaningful with a proxy attached. Assert the
// live cluster's attributes against the API rather than just that the list is
// non-empty.
func Test_ReverseProxyClusters_DataSourceMatchesAPI(t *testing.T) {
	testE2E(t)
	cluster := testRequireProxyCluster(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "netbird_reverse_proxy_clusters" "live" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(s *terraform.State) error {
						attrs := s.RootModule().Resources["data.netbird_reverse_proxy_clusters.live"].Primary.Attributes
						count := attrs["clusters.#"]
						if count == "0" || count == "" {
							return fmt.Errorf("data source returned no clusters, but management reports %s", cluster.Address)
						}
						// Find the registered cluster in the data source output.
						for i := 0; ; i++ {
							prefix := fmt.Sprintf("clusters.%d.", i)
							address, ok := attrs[prefix+"address"]
							if !ok {
								break
							}
							if address != cluster.Address {
								continue
							}
							if got := attrs[prefix+"connected_proxies"]; got != fmt.Sprint(cluster.ConnectedProxies) {
								return fmt.Errorf("connected_proxies mismatch for %s: data source %s, API %d",
									address, got, cluster.ConnectedProxies)
							}
							return nil
						}
						return fmt.Errorf("cluster %s reported by the API is missing from the data source", cluster.Address)
					},
				),
			},
		},
	})
}

// With a proxy attached, the gateway management allocates is a real one: the
// endpoint must sit one label beneath the live cluster's address, and the
// provider must surface the same values the API holds.
func Test_AgentNetwork_EndpointDerivedFromLiveCluster(t *testing.T) {
	testE2E(t)
	cluster := testRequireProxyCluster(t)

	rName := "anl" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	providerAddr := "netbird_agent_network_provider." + rName
	settingsAddr := "netbird_agent_network_settings." + rName

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			// The gateway is bootstrapped by this test, beneath the live cluster,
			// so release whatever an earlier test left pinned elsewhere.
			testReleaseGateway(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// The provider depends on the settings: the gateway has to exist
				// before anything routes through it, and outlive it on destroy.
				Config: fmt.Sprintf(`
resource "netbird_agent_network_settings" %[1]q {
  proxy_address             = %[2]q
  access_log_retention_days = 30
}

resource "netbird_agent_network_provider" %[1]q {
  provider_id  = "openai_api"
  name         = "%[1]s-provider"
  upstream_url = "https://api.openai.com"
  api_key      = "sk-acc-test"
  depends_on   = [netbird_agent_network_settings.%[1]s]
}`, rName, cluster.Address),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(providerAddr, "id"),
					resource.TestCheckResourceAttr(settingsAddr, "proxy_address", cluster.Address),
					// An endpoint beneath a shared cluster, so not dedicated.
					resource.TestCheckResourceAttr(settingsAddr, "dedicated", "false"),
					func(s *terraform.State) error {
						got, err := testAgentNetworkClient().GetSettings(context.Background())
						if err != nil {
							return err
						}
						attrs := s.RootModule().Resources[settingsAddr].Primary.Attributes

						// The provider must report exactly what the API holds.
						if attrs["endpoint"] != got.Endpoint {
							return fmt.Errorf("endpoint mismatch: provider %q, API %q", attrs["endpoint"], got.Endpoint)
						}
						if attrs["proxy_address"] != got.ProxyAddress {
							return fmt.Errorf("proxy address mismatch: provider %q, API %q", attrs["proxy_address"], got.ProxyAddress)
						}

						// And the endpoint has to hang one label beneath the
						// cluster the live proxy declares.
						if got.ProxyAddress != cluster.Address {
							return fmt.Errorf("gateway served from %q, but the live proxy cluster is %q", got.ProxyAddress, cluster.Address)
						}
						label, parent, found := strings.Cut(got.Endpoint, ".")
						if !found || label == "" || parent != cluster.Address {
							return fmt.Errorf("endpoint %q is not a single label beneath %q", got.Endpoint, cluster.Address)
						}
						return nil
					},
				),
			},
		},
	})
}

// The host requirement is asymmetric and easy to get wrong, so pin it: a subnet
// target without a host must be refused, while the same service with the host
// filled in is accepted.
func Test_ReverseProxyService_SubnetTargetRequiresHost(t *testing.T) {
	env := testE2E(t)
	cluster := testRequireProxyCluster(t)

	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + "." + cluster.Address

	config := func(host string) string {
		return fmt.Sprintf(`
resource "netbird_reverse_proxy_service" %[1]q {
  name   = %[1]q
  domain = %[2]q

  targets = [
    {
      target_id   = %[3]q
      target_type = "subnet"
      %[4]s
      port        = 8080
      protocol    = "http"
    },
  ]

  auth = {
    password_auth = {
      enabled  = true
      password = "subnethost"
    }
  }
}`, rName, domain, env.ResourceSubnetID, host)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config(""),
				ExpectError: regexp.MustCompile(`empty host but target_type is "subnet"`),
			},
			{
				Config: config(`host = "192.168.20.7"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbird_reverse_proxy_service."+rName, "targets.0.host", "192.168.20.7"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources["netbird_reverse_proxy_service."+rName].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return err
						}
						if len(svc.Targets) != 1 {
							return fmt.Errorf("expected 1 target, management reports %d", len(svc.Targets))
						}
						if valOr(svc.Targets[0].Host, "") != "192.168.20.7" {
							return fmt.Errorf("host not persisted, management reports %v", svc.Targets[0].Host)
						}
						return nil
					},
				),
			},
		},
	})
}
