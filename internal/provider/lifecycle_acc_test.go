//go:build e2e

// Replacement and rejection.
//
// Two lifecycle behaviours the suite never asserted.
//
// Nine resources declare RequiresReplace on 22 attributes between them, which is
// a promise that changing one recreates the object rather than updating it. No
// test checked that, and a provider that quietly updated in place would look
// identical to one that recreated — the apply succeeds either way. The tests
// below change such an attribute and require the server-assigned ID to change
// with it.
//
// Twenty resources declare 68 validators, and five ExpectError steps existed for
// all of them, every one in agent_network. A validator that never fires in a test
// is a rejection nobody has confirmed happens, so each test here feeds one value
// the validator should refuse and requires the plan to fail. These cost nothing
// to run: the configuration is rejected before anything reaches the server.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// rejects asserts a configuration is refused, with a message matching want.
func rejects(t *testing.T, config string, want *regexp.Regexp) {
	t.Helper()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    []resource.TestStep{{Config: config, ExpectError: want}},
	})
}

// oneOf is the message shape terraform-plugin-framework produces for a
// stringvalidator.OneOf rejection.
var oneOf = regexp.MustCompile(`(?s)value must be one of|Invalid Attribute Value`)

func Test_SetupKey_Replaced(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_setup_key." + rName
	var first string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// usage_limit is left out of both steps: the server fixes it at
				// 1 for a one-off key, so a configuration that names 0 for both
				// steps is asking the second one for something it cannot have.
				Config: testSetupKeyResourceNoLimit(rName, `reusable`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &first),
					resource.TestCheckResourceAttr(rNameFull, "type", "reusable"),
				),
			},
			{
				// type is RequiresReplace: a setup key's kind is fixed at creation.
				Config: testSetupKeyResourceNoLimit(rName, `one-off`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "type", "one-off"),
					testIDChanged(rNameFull, &first),
				),
			},
		},
	})
}

func Test_Token_Replaced(t *testing.T) {
	testE2E(t)
	rName := "t" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_token." + rName
	userID := mustE2E().UserID
	var first string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTokenResource(rName, userID, `180`),
				Check:  resource.ComposeAggregateTestCheckFunc(testRecordID(rNameFull, &first)),
			},
			{
				// Every attribute a token has is RequiresReplace, and Update says
				// so in as many words: "Personal Access Tokens can't be updated".
				// So changing the expiry has to produce a different token.
				Config: testTokenResource(rName, userID, `90`),
				Check:  resource.ComposeAggregateTestCheckFunc(testIDChanged(rNameFull, &first)),
			},
		},
	})
}

func Test_User_Replaced(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_user." + rName
	var first string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `user`),
				Check:  resource.ComposeAggregateTestCheckFunc(testRecordID(rNameFull, &first)),
			},
			{
				// role is not RequiresReplace, so this one must NOT be replaced —
				// the same assertion in the opposite direction, which is what
				// distinguishes an in-place update from a recreation.
				Config: testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `admin`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "role", "admin"),
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &first),
				),
			},
			{
				// is_blocked had never been set to true anywhere in the suite, so
				// the blocking path was written but never exercised. Also in place.
				Config: testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `true`, `admin`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "is_blocked", "true"),
					resource.TestCheckResourceAttrPtr(rNameFull, "id", &first),
				),
			},
		},
	})
}

// The rejection tests. Each feeds one value outside the validator's set.

func Test_SetupKey_RejectsUnknownType(t *testing.T) {
	testE2E(t)
	rName := "sk" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rejects(t, testSetupKeyResource(rName, `1800`, `perpetual`, `false`, `[]`, `false`, `false`, `0`), oneOf)
}

func Test_User_RejectsUnknownRole(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rejects(t, testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `false`, `superuser`), oneOf)
}

func Test_IdentityProvider_RejectsUnknownType(t *testing.T) {
	testE2E(t)
	rName := "idp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testIdentityProviderResource(rName, "jumpcloud", "ldap", "client-id", "client-secret",
		"https://acc.example/oauth2")
	rejects(t, cfg, oneOf)
}

func Test_Policy_RejectsUnknownAction(t *testing.T) {
	testE2E(t)
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testPolicyResourceGroups(rName, rName, "desc", "reject", "udp", e2eGroupAllID(), e2eGroupNotAllID(), "443")
	rejects(t, cfg, oneOf)
}

func Test_Policy_RejectsUnknownProtocol(t *testing.T) {
	testE2E(t)
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testPolicyResourceGroups(rName, rName, "desc", "accept", "sctp", e2eGroupAllID(), e2eGroupNotAllID(), "443")
	rejects(t, cfg, oneOf)
}

func Test_PostureCheck_RejectsUnknownGeoAction(t *testing.T) {
	testE2E(t)
	rName := "pc" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	cfg := testPostureCheckResource(rName, `posture_check`, `0.40.0`, `15`, `10`, `12`, `6.8.0`, `2531`,
		`EG`, `Cairo`, `permit`, `15.160.0.0/16`, `deny`, `/root`, `C:\\process.exe`, `/macpath`)
	rejects(t, cfg, oneOf)
}

func Test_DNSRecord_RejectsUnknownType(t *testing.T) {
	testE2E(t)
	zoneName := "z" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rName := "r" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	// MX is a real record type, and not one this provider accepts.
	cfg := testDNSRecordResource(zoneName, "reject.local", rName, "mail", "MX", "10 mail.reject.local", 300)
	rejects(t, cfg, oneOf)
}
