//go:build e2e

// Drift: what happens when something else changes the object.
//
// Nothing in the suite covered this, and it is the case a provider is for.
// Somebody edits a group in the dashboard, a script rewrites a policy, an
// operator flips a setting by hand — Terraform is supposed to notice on the
// next refresh and put it back. A provider whose Read maps a field wrongly, or
// silently keeps the value already in state, passes every other test in the
// suite and fails here.
//
// Each case creates the object through Terraform, changes it behind Terraform's
// back through the same API a dashboard would use, then applies the unchanged
// configuration again. Two things are asserted: the plan is an in-place update
// rather than a no-op or a replacement, and the value the configuration asked
// for is what the server holds afterwards.

package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

// driftCase creates an object, changes it through the API, and requires the
// next apply of the same configuration to put it back in place.
//
// The mutation runs in PreConfig, which is called after the previous step and
// before this one is planned, so the refresh that opens the step is the first
// thing to see it.
func driftCase(t *testing.T, address, config string, mutate func(id string) error, back ...resource.TestCheckFunc) {
	t.Helper()
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  testRecordID(address, &id),
			},
			{
				PreConfig: func() {
					if err := mutate(id); err != nil {
						t.Fatalf("could not change %s behind Terraform's back: %v", address, err)
					}
				},
				Config:           config,
				ConfigPlanChecks: updatesInPlace(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					append([]resource.TestCheckFunc{
						resource.TestCheckResourceAttrPtr(address, "id", &id),
					}, back...)...),
			},
		},
	})
}

func Test_Drift_Group(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	driftCase(t, "netbird_group."+rName, testGroupResource(rName, `[]`),
		func(id string) error {
			_, err := testClient().Groups.Update(context.Background(), id, api.GroupRequest{
				Name: rName + "-changed-elsewhere", Peers: &[]string{},
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_group."+rName, "name", rName),
	)
}

func Test_Drift_Network(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	driftCase(t, "netbird_network."+rName, testNetworkResource(rName, `Test`),
		func(id string) error {
			desc := "changed elsewhere"
			_, err := testClient().Networks.Update(context.Background(), id, api.NetworkRequest{
				Name: rName, Description: &desc,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_network."+rName, "description", "Test"),
	)
}

func Test_Drift_DNSZone(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := fmt.Sprintf(`resource "netbird_dns_zone" "%[1]s" {
  name    = "%[1]s"
  domain  = "%[1]s.local"
  enabled = true
}`, rName)
	driftCase(t, "netbird_dns_zone."+rName, cfg,
		func(id string) error {
			off := false
			_, err := testClient().DNSZones.UpdateZone(context.Background(), id, api.ZoneRequest{
				Name: rName, Enabled: &off,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_dns_zone."+rName, "enabled", "true"),
	)
}

func Test_Drift_NameserverGroup(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testNameserverGroupResource(rName, `1.1.1.1`, `53`, fmt.Sprintf("[%q]", e2eGroupAllID()))
	driftCase(t, "netbird_nameserver_group."+rName, cfg,
		func(id string) error {
			_, err := testClient().DNS.UpdateNameserverGroup(context.Background(), id, api.NameserverGroupRequest{
				Name: rName, Description: "changed elsewhere", Enabled: false,
				Groups: []string{e2eGroupAllID()},
				Nameservers: []api.Nameserver{{
					Ip: "1.1.1.1", NsType: api.NameserverNsTypeUdp, Port: 53,
				}},
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_nameserver_group."+rName, "enabled", "true"),
	)
}

func Test_Drift_Route(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testRouteResource(rName, e2eGroupAllID(), `null`, `desc`, `null`, `["example.com"]`,
		fmt.Sprintf("[%q]", e2eGroupNotAllID()), `null`)
	driftCase(t, "netbird_route."+rName, cfg,
		func(id string) error {
			route, err := testClient().Routes.Get(context.Background(), id)
			if err != nil {
				return err
			}
			_, err = testClient().Routes.Update(context.Background(), id, api.RouteRequest{
				NetworkId: route.NetworkId, Description: "changed elsewhere",
				Enabled: route.Enabled, Groups: route.Groups, Masquerade: route.Masquerade,
				Metric: route.Metric, Domains: route.Domains, PeerGroups: route.PeerGroups,
				KeepRoute: route.KeepRoute,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_route."+rName, "description", "desc"),
	)
}

func Test_Drift_SetupKey(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testSetupKeyResourceNoLimit(rName, `reusable`)
	driftCase(t, "netbird_setup_key."+rName, cfg,
		func(id string) error {
			// Revoking is the only change the setup key API accepts, and it is
			// also the one an operator makes by hand in a hurry.
			_, err := testClient().SetupKeys.Update(context.Background(), id, api.SetupKeyRequest{
				AutoGroups: []string{}, Revoked: true,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_setup_key."+rName, "revoked", "false"),
	)
}

func Test_Drift_User(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `user`)
	driftCase(t, "netbird_user."+rName, cfg,
		func(id string) error {
			_, err := testClient().Users.Update(context.Background(), id, api.UserRequest{
				Role: "admin", AutoGroups: []string{e2eGroupNotAllID()}, IsBlocked: false,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_user."+rName, "role", "user"),
	)
}

func Test_Drift_Policy(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testPolicyResourceGroups(rName, rName, "desc", "accept", "tcp",
		e2eGroupAllID(), e2eGroupNotAllID(), "443")
	driftCase(t, "netbird_policy."+rName, cfg,
		func(id string) error {
			p, err := testClient().Policies.Get(context.Background(), id)
			if err != nil {
				return err
			}
			off := false
			rules := make([]api.PolicyRuleUpdate, 0, len(p.Rules))
			for _, r := range p.Rules {
				rules = append(rules, api.PolicyRuleUpdate{
					Name: r.Name, Description: r.Description, Enabled: r.Enabled,
					Action: api.PolicyRuleUpdateAction(r.Action), Bidirectional: r.Bidirectional,
					Protocol: api.PolicyRuleUpdateProtocol(r.Protocol),
					Ports:    r.Ports, Sources: groupIDs(r.Sources), Destinations: groupIDs(r.Destinations),
				})
			}
			_, err = testClient().Policies.Update(context.Background(), id, api.PolicyCreate{
				Name: p.Name, Description: p.Description, Enabled: off, Rules: rules,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_policy."+rName, "enabled", "true"),
	)
}

// groupIDs turns the group objects a policy read returns back into the IDs an
// update takes.
func groupIDs(groups *[]api.GroupMinimum) *[]string {
	if groups == nil {
		return nil
	}
	ids := make([]string, 0, len(*groups))
	for _, g := range *groups {
		ids = append(ids, g.Id)
	}
	return &ids
}

func Test_Drift_Peer(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	// peer3 is one of the consumable fixtures: this test renames it, and a
	// shared one would leave the next test looking for a name that moved.
	peerID := testPeerID(t, "peer3")
	cfg := testPeerResource(rName, peerID, rName)
	driftCase(t, "netbird_peer."+rName, cfg,
		func(id string) error {
			p, err := testClient().Peers.Get(context.Background(), id)
			if err != nil {
				return err
			}
			_, err = testClient().Peers.Update(context.Background(), id, api.PeerRequest{
				Name: rName + "-changed-elsewhere", SshEnabled: p.SshEnabled,
				LoginExpirationEnabled: p.LoginExpirationEnabled,
			})
			return err
		},
		resource.TestCheckResourceAttr("netbird_peer."+rName, "name", rName),
	)
}

func Test_Drift_NetworkResource(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	netID := e2eNetworkID()
	cfg := testNetworkResourceResource(rName, netID, `example.com`,
		fmt.Sprintf("[%q]", e2eGroupNotAllID()), rName)
	driftCase(t, "netbird_network_resource."+rName, cfg,
		func(id string) error {
			desc := "changed elsewhere"
			_, err := testClient().Networks.Resources(netID).Update(context.Background(), id,
				api.NetworkResourceRequest{
					Name: rName, Description: &desc, Address: "example.com",
					Enabled: true, Groups: []string{e2eGroupNotAllID()},
				})
			return err
		},
		resource.TestCheckResourceAttr("netbird_network_resource."+rName, "name", rName),
	)
}

// Test_Drift_TokenDeletedElsewhere is the one drift case that cannot be an
// update: a token has no mutable attribute, so the only thing that can happen
// to it out of band is deletion. Terraform has to notice it is gone and issue a
// new one rather than report an empty plan.
func Test_Drift_TokenDeletedElsewhere(t *testing.T) {
	testE2E(t)
	rName := "d" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_token." + rName
	userID := mustE2E().UserID
	cfg := testTokenResource(rName, userID, `180`)
	var first string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check:  testRecordID(address, &first),
			},
			{
				PreConfig: func() {
					if err := testClient().Tokens.Delete(context.Background(), userID, first); err != nil {
						t.Fatalf("could not delete the token behind Terraform's back: %v", err)
					}
				},
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(address, "id"),
					testIDChanged(address, &first),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[address].Primary.Attributes["id"]
						_, err := testClient().Tokens.Get(context.Background(), userID, id)
						return err
					},
				),
			},
		},
	})
}
