//go:build e2e

// List order: lists the API returns are not order-stable, so the provider sorts them.
//
// Each case configures a list in descending order, then rewrites the same members in
// ascending order behind Terraform's back. Membership did not change, so the next plan
// must be empty and state must keep the configured order. A data source reading the same
// object has no configured order to keep and must return the list sorted.

package provider

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

// fixtureGroupsAscending returns the two fixture group IDs in ascending order.
func fixtureGroupsAscending() []string {
	ids := []string{e2eGroupAllID(), e2eGroupNotAllID()}
	slices.Sort(ids)
	return ids
}

// testGroupPair creates two groups through the API for lists that cannot hold the All
// group, and returns their IDs in ascending order.
func testGroupPair(t *testing.T, prefix string) []string {
	t.Helper()
	ids := make([]string, 0, 2)
	for _, suffix := range []string{"a", "b"} {
		g, err := testClient().Groups.Create(context.Background(), api.GroupRequest{Name: prefix + suffix})
		if err != nil {
			t.Fatalf("create group %s: %v", prefix+suffix, err)
		}
		ids = append(ids, g.Id)
		t.Cleanup(func() {
			if err := testClient().Groups.Delete(context.Background(), g.Id); err != nil {
				t.Logf("delete group %s: %v", g.Id, err)
			}
		})
	}
	slices.Sort(ids)
	return ids
}

func descending(asc []string) []string {
	desc := slices.Clone(asc)
	slices.Reverse(desc)
	return desc
}

func hclList(vals []string) string {
	out := "["
	for i, v := range vals {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", v)
	}
	return out + "]"
}

// testImportedAttrs checks attribute values on the imported instance, for imports whose
// state legitimately differs from the applied one and so cannot use ImportStateVerify.
func testImportedAttrs(id *string, want map[string]string) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		for _, s := range states {
			if s.ID != *id {
				continue
			}
			for k, v := range want {
				if got := s.Attributes[k]; got != v {
					return fmt.Errorf("imported %s = %q, want %q", k, got, v)
				}
			}
			return nil
		}
		return fmt.Errorf("no imported instance with id %q", *id)
	}
}

// listOrderCase runs the reorder-behind-Terraform's-back scenario shared by every case.
// An import has no configured order to keep, so importSorted lists the resource
// attributes that must come back sorted.
func listOrderCase(t *testing.T, address, resourceConfig, dataSourceConfig string, gone func(id *string) func(*terraform.State) error, reorder func(id string) error, inOrder, sorted []resource.TestCheckFunc, importSorted map[string]string) {
	t.Helper()
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             gone(&id),
		Steps: []resource.TestStep{
			{
				Config: resourceConfig,
				Check:  resource.ComposeAggregateTestCheckFunc(append([]resource.TestCheckFunc{testRecordID(address, &id)}, inOrder...)...),
			},
			{
				PreConfig: func() {
					if err := reorder(id); err != nil {
						t.Fatalf("could not reorder %s behind Terraform's back: %v", address, err)
					}
				},
				Config: resourceConfig,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(inOrder...),
			},
			{
				Config: resourceConfig + dataSourceConfig,
				Check:  resource.ComposeAggregateTestCheckFunc(sorted...),
			},
			{
				Config:           resourceConfig + dataSourceConfig,
				ResourceName:     address,
				ImportState:      true,
				ImportStateCheck: testImportedAttrs(&id, importSorted),
			},
		},
	})
}

func Test_ListOrder_SetupKeyAutoGroups(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	asc := testGroupPair(t, rName)
	desc := descending(asc)

	cfg := fmt.Sprintf(`resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
  auto_groups    = %[2]s
}
`, rName, hclList(desc))
	ds := fmt.Sprintf(`
data "netbird_setup_key" "%[1]s" {
  id = netbird_setup_key.%[1]s.id
}
`, rName)

	listOrderCase(t, address, cfg, ds,
		func(id *string) func(*terraform.State) error { return testCheckGone(testClient().SetupKeys.Get, id) },
		func(id string) error {
			_, err := testClient().SetupKeys.Update(context.Background(), id, api.SetupKeyRequest{AutoGroups: asc})
			return err
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "auto_groups.0", desc[0]),
			resource.TestCheckResourceAttr(address, "auto_groups.1", desc[1]),
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr("data.netbird_setup_key."+rName, "auto_groups.0", asc[0]),
			resource.TestCheckResourceAttr("data.netbird_setup_key."+rName, "auto_groups.1", asc[1]),
		},
		map[string]string{"auto_groups.0": asc[0], "auto_groups.1": asc[1]},
	)
}

func Test_ListOrder_PolicyRuleGroups(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_policy." + rName
	asc := fixtureGroupsAscending()
	desc := descending(asc)

	cfg := fmt.Sprintf(`resource "netbird_policy" "%[1]s" {
  name    = "%[1]s"
  enabled = true

  rule {
    name          = "%[1]s"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    sources       = %[2]s
    destinations  = %[2]s
    ports         = ["8080", "443"]
  }
}
`, rName, hclList(desc))
	ds := fmt.Sprintf(`
data "netbird_policy" "%[1]s" {
  id = netbird_policy.%[1]s.id
}
`, rName)

	listOrderCase(t, address, cfg, ds,
		func(id *string) func(*terraform.State) error { return testCheckGone(testClient().Policies.Get, id) },
		func(id string) error {
			p, err := testClient().Policies.Get(context.Background(), id)
			if err != nil {
				return err
			}
			rules := make([]api.PolicyRuleUpdate, 0, len(p.Rules))
			for _, r := range p.Rules {
				rules = append(rules, api.PolicyRuleUpdate{
					Id: r.Id, Name: r.Name, Description: r.Description, Enabled: r.Enabled,
					Action: api.PolicyRuleUpdateAction(r.Action), Bidirectional: r.Bidirectional,
					Protocol: api.PolicyRuleUpdateProtocol(r.Protocol),
					Ports:    &[]string{"443", "8080"}, Sources: &asc, Destinations: &asc,
				})
			}
			_, err = testClient().Policies.Update(context.Background(), id, api.PolicyCreate{
				Name: p.Name, Description: p.Description, Enabled: p.Enabled, Rules: rules,
			})
			return err
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "rule.0.sources.0", desc[0]),
			resource.TestCheckResourceAttr(address, "rule.0.destinations.0", desc[0]),
			resource.TestCheckResourceAttr(address, "rule.0.ports.0", "8080"),
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr("data.netbird_policy."+rName, "rule.0.sources.0", asc[0]),
			resource.TestCheckResourceAttr("data.netbird_policy."+rName, "rule.0.destinations.0", asc[0]),
			resource.TestCheckResourceAttr("data.netbird_policy."+rName, "rule.0.ports.0", "443"),
		},
		map[string]string{"rule.0.sources.0": asc[0], "rule.0.destinations.0": asc[0], "rule.0.ports.0": "443"},
	)
}

// Test_ListOrder_PolicyRuleConfiguredOrder applies the same rule groups in both orders.
// The server answers rule groups in an order of its own, so one of the two applies
// disagrees with the plan unless the provider keeps the planned order.
func Test_ListOrder_PolicyRuleConfiguredOrder(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_policy." + rName
	asc := fixtureGroupsAscending()
	desc := descending(asc)

	cfg := func(groups []string) string {
		return fmt.Sprintf(`resource "netbird_policy" "%[1]s" {
  name    = "%[1]s"
  enabled = true

  rule {
    name          = "%[1]s"
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    sources       = %[2]s
    destinations  = %[2]s
  }
}
`, rName, hclList(groups))
	}

	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Policies.Get, &id),
		Steps: []resource.TestStep{
			{
				Config: cfg(desc),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(address, &id),
					resource.TestCheckResourceAttr(address, "rule.0.sources.0", desc[0]),
					resource.TestCheckResourceAttr(address, "rule.0.destinations.0", desc[0]),
				),
			},
			{
				Config: cfg(asc),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(address, "id", &id),
					resource.TestCheckResourceAttr(address, "rule.0.sources.0", asc[0]),
					resource.TestCheckResourceAttr(address, "rule.0.destinations.0", asc[0]),
				),
			},
		},
	})
}

func Test_ListOrder_NameserverGroupDomains(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_nameserver_group." + rName
	asc := fixtureGroupsAscending()
	desc := descending(asc)
	domainsAsc := []string{"a-" + rName + ".example.com", "b-" + rName + ".example.com"}
	domainsDesc := descending(domainsAsc)

	cfg := fmt.Sprintf(`resource "netbird_nameserver_group" "%[1]s" {
  name    = "%[1]s"
  primary = false
  domains = %[2]s
  groups  = %[3]s
  nameservers = [
    {
      ip      = "8.8.8.8"
      ns_type = "udp"
      port    = 53
    }
  ]
}
`, rName, hclList(domainsDesc), hclList(desc))
	ds := fmt.Sprintf(`
data "netbird_nameserver_group" "%[1]s" {
  id = netbird_nameserver_group.%[1]s.id
}
`, rName)

	listOrderCase(t, address, cfg, ds,
		func(id *string) func(*terraform.State) error {
			return testCheckGone(testClient().DNS.GetNameserverGroup, id)
		},
		func(id string) error {
			ns, err := testClient().DNS.GetNameserverGroup(context.Background(), id)
			if err != nil {
				return err
			}
			_, err = testClient().DNS.UpdateNameserverGroup(context.Background(), id, api.NameserverGroupRequest{
				Name: ns.Name, Description: ns.Description, Enabled: ns.Enabled, Primary: ns.Primary,
				SearchDomainsEnabled: ns.SearchDomainsEnabled, Nameservers: ns.Nameservers,
				Domains: domainsAsc, Groups: asc,
			})
			return err
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "domains.0", domainsDesc[0]),
			resource.TestCheckResourceAttr(address, "groups.0", desc[0]),
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr("data.netbird_nameserver_group."+rName, "domains.0", domainsAsc[0]),
			resource.TestCheckResourceAttr("data.netbird_nameserver_group."+rName, "groups.0", asc[0]),
		},
		map[string]string{"domains.0": domainsAsc[0], "groups.0": asc[0]},
	)
}

func Test_ListOrder_GroupPeers(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_group." + rName
	asc := []string{testPeerID(t, "peer1"), testPeerID(t, "peer2")}
	slices.Sort(asc)
	desc := descending(asc)

	cfg := fmt.Sprintf(`resource "netbird_group" "%[1]s" {
  name  = "%[1]s"
  peers = %[2]s
}
`, rName, hclList(desc))
	ds := fmt.Sprintf(`
data "netbird_group" "%[1]s" {
  id = netbird_group.%[1]s.id
}
`, rName)

	listOrderCase(t, address, cfg, ds,
		func(id *string) func(*terraform.State) error { return testCheckGone(testClient().Groups.Get, id) },
		func(id string) error {
			_, err := testClient().Groups.Update(context.Background(), id, api.GroupRequest{Name: rName, Peers: &asc})
			return err
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "peers.0", desc[0]),
			resource.TestCheckResourceAttr(address, "peers.1", desc[1]),
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr("data.netbird_group."+rName, "peers.0", asc[0]),
			resource.TestCheckResourceAttr("data.netbird_group."+rName, "peers.1", asc[1]),
		},
		map[string]string{"peers.0": asc[0], "peers.1": asc[1]},
	)
}

// Test_ListOrder_GuardrailModels covers a list nested in an object attribute.
func Test_ListOrder_GuardrailModels(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_agent_network_guardrail." + rName
	asc := []string{"gpt-4.1", "o4-mini"}
	desc := descending(asc)

	cfg := fmt.Sprintf(`resource "netbird_agent_network_guardrail" "%[1]s" {
  name        = "%[1]s"
  description = "list order"
  model_allowlist = {
    enabled = true
    models  = %[2]s
  }
  prompt_capture = {
    enabled    = false
    redact_pii = false
  }
}
`, rName, hclList(desc))
	ds := fmt.Sprintf(`
data "netbird_agent_network_guardrail" "%[1]s" {
  id = netbird_agent_network_guardrail.%[1]s.id
}
`, rName)

	listOrderCase(t, address, cfg, ds,
		func(id *string) func(*terraform.State) error {
			return testCheckGone(testAgentNetworkClient().GetGuardrail, id)
		},
		func(id string) error {
			g, err := testAgentNetworkClient().GetGuardrail(context.Background(), id)
			if err != nil {
				return err
			}
			checks := g.Checks
			checks.ModelAllowlist.Models = asc
			_, err = testAgentNetworkClient().UpdateGuardrail(context.Background(), id, api.AgentNetworkGuardrailRequest{
				Name: g.Name, Description: &g.Description, Checks: checks,
			})
			return err
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "model_allowlist.models.0", desc[0]),
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr("data.netbird_agent_network_guardrail."+rName, "model_allowlist.models.0", asc[0]),
		},
		map[string]string{"model_allowlist.models.0": asc[0]},
	)
}

// Test_ListOrder_PostureCheck covers lists of objects: geo locations nested in a block,
// network ranges nested in a block, and the process_check blocks themselves.
func Test_ListOrder_PostureCheck(t *testing.T) {
	testE2E(t)
	rName := "lo" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_posture_check." + rName

	cfg := fmt.Sprintf(`resource "netbird_posture_check" "%[1]s" {
  name        = "%[1]s"
  description = "list order"

  geo_location_check {
    locations = [
      { country_code = "EG", city_name = "Cairo" },
      { country_code = "DE", city_name = "Berlin" },
    ]
    action = "allow"
  }

  peer_network_range_check {
    ranges = ["10.2.0.0/16", "10.1.0.0/16"]
    action = "deny"
  }

  process_check {
    linux_path = "/usr/bin/zsh"
  }

  process_check {
    linux_path = "/usr/bin/bash"
  }
}
`, rName)
	ds := fmt.Sprintf(`
data "netbird_posture_check" "%[1]s" {
  id = netbird_posture_check.%[1]s.id
}
`, rName)

	listOrderCase(t, address, cfg, ds,
		func(id *string) func(*terraform.State) error {
			return testCheckGone(testClient().PostureChecks.Get, id)
		},
		func(id string) error {
			pc, err := testClient().PostureChecks.Get(context.Background(), id)
			if err != nil {
				return err
			}
			checks := pc.Checks
			slices.Reverse(checks.GeoLocationCheck.Locations)
			slices.Reverse(checks.PeerNetworkRangeCheck.Ranges)
			slices.Reverse(checks.ProcessCheck.Processes)
			_, err = testClient().PostureChecks.Update(context.Background(), id, api.PostureCheckUpdate{
				Name: pc.Name, Description: valOr(pc.Description, ""), Checks: &checks,
			})
			return err
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr(address, "geo_location_check.locations.0.city_name", "Cairo"),
			resource.TestCheckResourceAttr(address, "peer_network_range_check.ranges.0", "10.2.0.0/16"),
			resource.TestCheckResourceAttr(address, "process_check.0.linux_path", "/usr/bin/zsh"),
		},
		[]resource.TestCheckFunc{
			resource.TestCheckResourceAttr("data.netbird_posture_check."+rName, "geo_location_check.locations.0.city_name", "Berlin"),
			resource.TestCheckResourceAttr("data.netbird_posture_check."+rName, "peer_network_range_check.ranges.0", "10.1.0.0/16"),
			resource.TestCheckResourceAttr("data.netbird_posture_check."+rName, "process_check.0.linux_path", "/usr/bin/bash"),
		},
		map[string]string{
			"geo_location_check.locations.0.city_name": "Berlin",
			"peer_network_range_check.ranges.0":        "10.1.0.0/16",
			"process_check.0.linux_path":               "/usr/bin/bash",
		},
	)
}
