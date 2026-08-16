//go:build e2e

// The cases that belong to one resource and no pattern.
//
// Everything here came out of reading each resource against the operations a
// user actually performs on it, rather than from a family that applies across
// the provider. They are the leftovers in the honest sense: each one is
// specific enough that it could not be written as a table.

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

// Test_Group_AddAndRemovePeers covers the direction the group test never went.
// Adding a peer was covered; taking it away again was not, and a provider that
// sends the new list without the removal looks identical on the way in.
func Test_Group_AddAndRemovePeers(t *testing.T) {
	testE2E(t)
	rName := "g" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_group." + rName
	peerID := testPeerID(t, "peer1")
	membersAre := func(want int) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			id := s.RootModule().Resources[address].Primary.Attributes["id"]
			group, err := testClient().Groups.Get(context.Background(), id)
			if err != nil {
				return err
			}
			if len(group.Peers) != want {
				return fmt.Errorf("management has %d peers in the group, expected %d", len(group.Peers), want)
			}
			return nil
		}
	}
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().Groups.Get, &id),
		Steps: []resource.TestStep{
			{
				Config: testGroupResource(rName, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(address, &id),
					membersAre(0),
				),
			},
			{
				Config:           testGroupResource(rName, fmt.Sprintf("[%q]", peerID)),
				ConfigPlanChecks: updatesInPlace(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "peers.#", "1"),
					membersAre(1),
				),
			},
			{
				Config:           testGroupResource(rName, `[]`),
				ConfigPlanChecks: updatesInPlace(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(address, "id", &id),
					resource.TestCheckResourceAttr(address, "peers.#", "0"),
					membersAre(0),
				),
			},
		},
	})
}

// Test_DNSZone_DestroyTakesItsRecords checks the cascade. A record lives inside
// a zone, so destroying the zone has to leave nothing addressable behind — and
// Terraform destroys in dependency order, which is the part worth confirming
// against a server rather than assuming.
func Test_DNSZone_DestroyTakesItsRecords(t *testing.T) {
	testE2E(t)
	rName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	zoneAddress := "netbird_dns_zone." + rName
	var zoneID, recordID string
	config := fmt.Sprintf(`
resource "netbird_dns_zone" "%[1]s" {
  name                = "%[1]s"
  domain              = "%[1]s.local"
  distribution_groups = [%[2]q]
}

resource "netbird_dns_record" "%[1]s" {
  zone_id = netbird_dns_zone.%[1]s.id
  name    = "www.%[1]s.local"
  type    = "A"
  content = "10.0.0.1"
  ttl     = 300
}`, rName, e2eGroupNotAllID())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			if _, err := testClient().DNSZones.GetZone(context.Background(), zoneID); err == nil {
				return fmt.Errorf("zone %s is still on the management server after destroy", zoneID)
			}
			// The record is addressed through the zone, so its own lookup has to
			// fail as well rather than resolving against a zone that is gone.
			if _, err := testClient().DNSZones.GetRecord(context.Background(), zoneID, recordID); err == nil {
				return fmt.Errorf("record %s is still on the management server after destroy", recordID)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(zoneAddress, &zoneID),
					testRecordID("netbird_dns_record."+rName, &recordID),
				),
			},
		},
	})
}

// Test_Replace_UserServiceUser changes what kind of account a user is. The
// server has no way to convert one into the other, which is why the attribute
// forces replacement, and this is the assertion that says so.
func Test_Replace_UserServiceUser(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_user." + rName
	cfg := func(service string) string {
		// No email: the server drops it for a service user and returns an empty
		// one, which Terraform reports as the provider contradicting its own
		// plan. That is a defect worth fixing on its own, and not this test's
		// subject.
		return fmt.Sprintf(`resource "netbird_user" "%[1]s" {
  name            = "%[1]s"
  is_service_user = %[2]s
  role            = "user"
  auto_groups     = []
}
`, rName, service)
	}
	replaceCase(t, address, cfg("true"), cfg("false"),
		resource.TestCheckResourceAttr(address, "is_service_user", "false"),
	)
}

// Test_SetupKey_ImportThenApply is the case the expiry_seconds plan modifier
// exists for, and it starts the way a user starts: with a key the server
// already has and Terraform has never seen. The import gives state no
// expiry_seconds, because the API never reports one, and the apply that follows
// has to adopt what the configuration says rather than destroy the key to get
// it.
func Test_SetupKey_ImportThenApply(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_setup_key." + rName
	config := fmt.Sprintf(`resource "netbird_setup_key" "%[1]s" {
  name           = "%[1]s"
  expiry_seconds = 1800
  type           = "reusable"
  auto_groups    = []
}`, rName)
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().SetupKeys.Get, &id),
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					key, err := testClient().SetupKeys.Create(context.Background(), api.CreateSetupKeyRequest{
						Name: rName, Type: "reusable", ExpiresIn: 1800,
						AutoGroups: []string{}, UsageLimit: 0,
					})
					if err != nil {
						t.Fatalf("could not create the key to import: %v", err)
					}
					id = key.Id
				},
				Config:             config,
				ResourceName:       address,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  func(*terraform.State) (string, error) { return id, nil },
			},
			{
				Config:           config,
				ConfigPlanChecks: updatesInPlace(address),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(address, "id", &id),
					resource.TestCheckResourceAttr(address, "expiry_seconds", "1800"),
					resource.TestCheckResourceAttr(address, "name", rName),
				),
			},
		},
	})
}

// Test_Token_ImportThenPlan is the same flow for the token, where the value the
// API cannot report is recoverable from two timestamps that it can. If that
// arithmetic is wrong the plan after the import is not empty, and the only way
// to satisfy it is a new token.
func Test_Token_ImportThenPlan(t *testing.T) {
	testE2E(t)
	rName := "t" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_token." + rName
	userID := mustE2E().UserID
	config := testTokenResource(rName, userID, `180`)
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					token, err := testClient().Tokens.Create(context.Background(), userID,
						api.PersonalAccessTokenRequest{Name: rName, ExpiresIn: 180})
					if err != nil {
						t.Fatalf("could not create the token to import: %v", err)
					}
					id = token.PersonalAccessToken.Id
				},
				Config:             config,
				ResourceName:       address,
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateIdFunc:  func(*terraform.State) (string, error) { return userID + "/" + id, nil },
			},
			{
				// Nothing to do: expiration_days came back from the timestamps,
				// so the token Terraform imported is the token it plans for.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// Destroying an account-wide singleton.
//
// Three resources describe settings rather than objects, and destroy means
// something different for each of them. Nothing asserted which, and the answer
// is not what a reader would assume: two of the three do nothing at all, so
// `terraform destroy` drops them from state and leaves the account exactly as
// the configuration left it.
//
// That is defensible — an account's settings cannot be deleted, only changed,
// and reverting to defaults would be a decision the configuration never asked
// for. It is also a surprise worth having in a test, because it means a
// destroyed configuration is not a clean account.

func Test_Destroy_DNSSettings_KeepsTheSettings(t *testing.T) {
	testE2E(t)
	rName := "ds" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	group := e2eGroupNotAllID()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			settings, err := testClient().DNS.GetSettings(context.Background())
			if err != nil {
				return err
			}
			for _, g := range settings.DisabledManagementGroups {
				if g == group {
					return nil
				}
			}
			return fmt.Errorf("the group the configuration disabled is no longer disabled after destroy, so something did revert it: %v",
				settings.DisabledManagementGroups)
		},
		Steps: []resource.TestStep{
			{
				Config: testDNSSettingsResource(rName, fmt.Sprintf("[%q]", group)),
				Check: resource.TestCheckResourceAttr("netbird_dns_settings."+rName,
					"disabled_management_groups.#", "1"),
			},
		},
	})
}

func Test_Destroy_AccountSettings_KeepsTheSettings(t *testing.T) {
	testE2E(t)
	rName := "as" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	address := "netbird_account_settings." + rName
	config := fmt.Sprintf(`resource "netbird_account_settings" "%[1]s" {
  peer_login_expiration_enabled = true
  peer_login_expiration         = 7200
}`, rName)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			accounts, err := testClient().Accounts.List(context.Background())
			if err != nil {
				return err
			}
			for _, a := range accounts {
				if a.Id != mustE2E().AccountID {
					continue
				}
				if a.Settings.PeerLoginExpiration != 7200 {
					return fmt.Errorf("peer_login_expiration is %d after destroy, expected the configured 7200 to still be there",
						a.Settings.PeerLoginExpiration)
				}
				return nil
			}
			return fmt.Errorf("the account under test is not in the list")
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(address, "peer_login_expiration", "7200"),
			},
		},
	})
}
