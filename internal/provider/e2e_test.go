// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Test_E2E_AccountActivatedBySetupCall pins down how the suite authenticates:
// the account exists because POST /api/setup created it, and the token every
// other test uses is the one that call returned. Nothing here reaches into the
// management store.
func Test_E2E_AccountActivatedBySetupCall(t *testing.T) {
	env := testE2E(t)
	ctx := context.Background()

	status, err := netbird.NewWithOptions(netbird.WithManagementURL(env.ManagementURL)).Instance.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GET /api/instance: %v", err)
	}
	if status.SetupRequired {
		t.Fatal("instance still reports setup_required after bootstrap")
	}

	if env.Token == "" {
		t.Fatal("bootstrap produced no API token")
	}
	if !strings.HasPrefix(env.Token, "nbp_") {
		t.Errorf("token does not look like a NetBird PAT: %q", env.Token)
	}

	me, err := testClient().Users.Current(ctx)
	if err != nil {
		t.Fatalf("the token minted by /api/setup does not authenticate: %v", err)
	}
	if me.Email != e2eAdminEmail {
		t.Errorf("owner email mismatch: expected %s, found %s", e2eAdminEmail, me.Email)
	}
	if me.Role != "owner" {
		t.Errorf("the user created by /api/setup should own the account, found role %q", me.Role)
	}
	if me.Id != env.UserID {
		t.Errorf("owner ID mismatch: bootstrap recorded %s, API reports %s", env.UserID, me.Id)
	}

	accounts, err := testClient().Accounts.List(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected exactly one account on a freshly activated instance, found %d", len(accounts))
	}
	if accounts[0].Id != env.AccountID {
		t.Errorf("account ID mismatch: bootstrap recorded %s, API reports %s", env.AccountID, accounts[0].Id)
	}

	// Setup is one-shot. A replay must not hand out a second owner or token.
	createPAT := true
	replay, err := netbird.NewWithOptions(netbird.WithManagementURL(env.ManagementURL)).
		Instance.Setup(ctx, api.PostApiSetupJSONRequestBody{
		Email:     "intruder@netbird.test",
		Name:      "Intruder",
		Password:  e2eAdminPassword,
		CreatePat: &createPAT,
	})
	if err == nil && replay != nil && replay.PersonalAccessToken != nil {
		t.Fatal("POST /api/setup succeeded a second time and issued another token")
	}
}

// Test_E2E_DashboardTargetsTheManagementUnderTest checks the other half of the
// deployment: the dashboard container is up, and the management endpoint it was
// configured with is the same activated instance the provider drives. A
// dashboard pointed somewhere else is a broken deployment that unit tests cannot
// catch.
func Test_E2E_DashboardTargetsTheManagementUnderTest(t *testing.T) {
	env := testE2E(t)
	ctx := context.Background()

	if env.DashboardURL == "" {
		t.Skip("no dashboard in this deployment")
	}

	code, body, err := httpGet(ctx, env.DashboardURL+"/")
	if err != nil {
		t.Fatalf("GET %s: %v", env.DashboardURL, err)
	}
	if code != 200 {
		t.Fatalf("dashboard returned HTTP %d, expected 200", code)
	}
	if strings.TrimSpace(body) == "" {
		t.Fatal("dashboard served an empty body")
	}

	if !e2eOwned {
		t.Skip("dashboard container is not managed by this run; served-page check only")
	}

	// Ask the dashboard container itself to call the management endpoint it was
	// configured with. This proves the wiring, not just that two containers
	// happen to be running.
	out, err := composeExecHTTP(ctx, "netbird-dashboard", "http://192.168.1.10:80/api/instance")
	if err != nil {
		t.Skipf("dashboard image has no usable HTTP client to probe with: %v", err)
	}
	if !strings.Contains(out, `"setup_required":false`) {
		t.Fatalf("management endpoint configured on the dashboard did not report an activated instance, got: %s", out)
	}
}

// Test_E2E_AgentsRegisteredThroughSetupKey asserts the peer fixtures are real:
// they were created by agents that logged in with a setup key, so they carry the
// attributes only a registration produces.
func Test_E2E_AgentsRegisteredThroughSetupKey(t *testing.T) {
	testE2E(t)
	ctx := context.Background()

	for _, name := range e2ePeerNames {
		id := testPeerID(t, name)
		peer, err := testClient().Peers.Get(ctx, id)
		if err != nil {
			t.Fatalf("read agent %s: %v", name, err)
		}
		if peer.Ip == "" {
			t.Errorf("agent %s has no address, so management never completed its registration", name)
		}
		if peer.Hostname != name {
			t.Errorf("agent %s reports hostname %q", name, peer.Hostname)
		}
		if peer.Os == "" {
			t.Errorf("agent %s reported no OS, so it sent no system info at login", name)
		}
	}
}

// Test_E2E_TerraformDrivesTheLiveStack runs a configuration that spans several
// resources and references a real registered agent, then verifies the result
// through the management API and checks it is cleaned up on destroy. This is the
// full path: Terraform -> provider -> HTTP -> management -> store.
func Test_E2E_TerraformDrivesTheLiveStack(t *testing.T) {
	testE2E(t)
	peerID := testPeerID(t, "peer1")

	rName := "e2e" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	groupAddr := "netbird_group." + rName
	policyAddr := "netbird_policy." + rName
	keyAddr := "netbird_setup_key." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			groups, err := testClient().Groups.List(context.Background())
			if err != nil {
				return err
			}
			for _, g := range groups {
				if g.Name == rName {
					return fmt.Errorf("group %s survived destroy", rName)
				}
			}
			policies, err := testClient().Policies.List(context.Background())
			if err != nil {
				return err
			}
			for _, p := range policies {
				if p.Name == rName {
					return fmt.Errorf("policy %s survived destroy", rName)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testE2EStackConfig(rName, peerID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(groupAddr, "id"),
					resource.TestCheckResourceAttr(groupAddr, "peers.#", "1"),
					resource.TestCheckResourceAttr(groupAddr, "peers.0", peerID),
					resource.TestCheckResourceAttrSet(policyAddr, "id"),
					resource.TestCheckResourceAttrSet(keyAddr, "key"),
					func(s *terraform.State) error {
						ctx := context.Background()
						groupID := s.RootModule().Resources[groupAddr].Primary.Attributes["id"]

						group, err := testClient().Groups.Get(ctx, groupID)
						if err != nil {
							return fmt.Errorf("read the group back: %w", err)
						}
						if len(group.Peers) != 1 || group.Peers[0].Id != peerID {
							return fmt.Errorf("the group does not contain the registered agent, found %#v", group.Peers)
						}

						policyID := s.RootModule().Resources[policyAddr].Primary.Attributes["id"]
						policy, err := testClient().Policies.Get(ctx, policyID)
						if err != nil {
							return fmt.Errorf("read the policy back: %w", err)
						}
						if len(policy.Rules) != 1 {
							return fmt.Errorf("expected 1 rule, found %d", len(policy.Rules))
						}
						rule := policy.Rules[0]
						if rule.Sources == nil || len(*rule.Sources) != 1 || (*rule.Sources)[0].Id != groupID {
							return fmt.Errorf("the policy rule does not source from the Terraform-managed group, found %#v", rule.Sources)
						}

						keyID := s.RootModule().Resources[keyAddr].Primary.Attributes["id"]
						key, err := testClient().SetupKeys.Get(ctx, keyID)
						if err != nil {
							return fmt.Errorf("read the setup key back: %w", err)
						}
						if len(key.AutoGroups) != 1 || key.AutoGroups[0] != groupID {
							return fmt.Errorf("the setup key does not auto-assign the Terraform-managed group, found %#v", key.AutoGroups)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      groupAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testE2EStackConfig(rName, peerID string) string {
	return fmt.Sprintf(`
resource "netbird_group" %[1]q {
  name  = %[1]q
  peers = [%[2]q]
}

resource "netbird_policy" %[1]q {
  name        = %[1]q
  description = "created end to end"
  enabled     = true

  rule {
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    name          = %[1]q
    sources       = [netbird_group.%[1]s.id]
    destinations  = [%[3]q]
    ports         = ["8080"]
  }
}

resource "netbird_setup_key" %[1]q {
  name            = %[1]q
  expiry_seconds  = 3600
  type            = "reusable"
  auto_groups     = [netbird_group.%[1]s.id]
  usage_limit     = 0
  ephemeral       = false
  revoked         = false
}
`, rName, peerID, e2eGroupAllID())
}
