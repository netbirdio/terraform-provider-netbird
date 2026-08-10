// Copyright (c) HashiCorp, Inc.

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

// Resources whose Update path had no acceptance coverage. An in-place update is
// the step most likely to drop a field the request rebuilds from scratch, so
// each of these checks the management server after the change, not just the
// Terraform state.

// A personal access token is immutable: every attribute is RequiresReplace and
// the resource's Update refuses outright, so changing the name must destroy and
// recreate — with a new ID and a new secret — rather than edit in place.
func Test_Token_RenameReplaces(t *testing.T) {
	env := testE2E(t)
	rName := "tok" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_token." + rName
	var originalTokenID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			tokens, err := testClient().Tokens.List(context.Background(), env.UserID)
			if err != nil {
				return err
			}
			for _, tok := range tokens {
				if tok.Name == rName+"-renamed" {
					return fmt.Errorf("token %s survived destroy", tok.Name)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testTokenResource(rName, env.UserID, `180`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", rName),
					resource.TestCheckResourceAttrSet(addr, "token"),
					func(s *terraform.State) error {
						originalTokenID = s.RootModule().Resources[addr].Primary.Attributes["id"]
						return nil
					},
				),
			},
			{
				Config: fmt.Sprintf(`resource "netbird_token" %[1]q {
  user_id         = %[2]q
  name            = "%[1]s-renamed"
  expiration_days = 180
}
`, rName, env.UserID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", rName+"-renamed"),
					func(s *terraform.State) error {
						attrs := s.RootModule().Resources[addr].Primary.Attributes
						if attrs["id"] == originalTokenID {
							return fmt.Errorf("token kept ID %s across a rename; it should have been replaced", originalTokenID)
						}
						tok, err := testClient().Tokens.Get(context.Background(), attrs["user_id"], attrs["id"])
						if err != nil {
							return err
						}
						if tok.Name != rName+"-renamed" {
							return fmt.Errorf("rename not persisted, management reports %q", tok.Name)
						}
						// The token it replaced must be gone, not orphaned.
						if _, err := testClient().Tokens.Get(context.Background(), attrs["user_id"], originalTokenID); err == nil {
							return fmt.Errorf("the replaced token %s was left behind on the management server", originalTokenID)
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_User_Update(t *testing.T) {
	testE2E(t)
	rName := "u" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_user." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testUserResource(rName, `[]`, `false`, `user`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "role", "user"),
					resource.TestCheckResourceAttr(addr, "auto_groups.#", "0"),
					resource.TestCheckResourceAttr(addr, "is_blocked", "false"),
				),
			},
			{
				Config: testUserResource(rName, fmt.Sprintf("[%q]", e2eGroupNotAllID()), `true`, `admin`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "role", "admin"),
					resource.TestCheckResourceAttr(addr, "auto_groups.#", "1"),
					resource.TestCheckResourceAttr(addr, "auto_groups.0", e2eGroupNotAllID()),
					resource.TestCheckResourceAttr(addr, "is_blocked", "true"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						users, err := testClient().Users.List(context.Background())
						if err != nil {
							return err
						}
						for _, u := range users {
							if u.Id != id {
								continue
							}
							return matchPairs(map[string][]any{
								"role":            {"admin", u.Role},
								"is_blocked":      {true, u.IsBlocked},
								"auto_groups.#":   {1, len(u.AutoGroups)},
								"auto_groups.0":   {e2eGroupNotAllID(), u.AutoGroups[0]},
								"is_service_user": {true, valOr(u.IsServiceUser, false)},
							})
						}
						return fmt.Errorf("user %s not found on the management server", id)
					},
				),
			},
		},
	})
}

func Test_AgentNetworkSettings_Update(t *testing.T) {
	testE2E(t)
	rName := "ans" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_agent_network_settings." + rName

	config := func(retention int, redactPII bool) string {
		return fmt.Sprintf(`
resource "netbird_agent_network_provider" %[1]q {
  provider_id       = "openai_api"
  name              = "%[1]s-provider"
  upstream_url      = "https://api.openai.com"
  api_key           = "sk-acc-test"
  bootstrap_cluster = %[2]q
}

resource "netbird_agent_network_settings" %[1]q {
  access_log_retention_days = %[3]d
  redact_pii                = %[4]t
  depends_on                = [netbird_agent_network_provider.%[1]s]
}`, rName, testBootstrapCluster, retention, redactPII)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(45, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "access_log_retention_days", "45"),
					resource.TestCheckResourceAttr(addr, "redact_pii", "false"),
				),
			},
			{
				Config: config(60, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "access_log_retention_days", "60"),
					resource.TestCheckResourceAttr(addr, "redact_pii", "true"),
					func(s *terraform.State) error {
						got, err := testAgentNetworkClient().GetSettings(context.Background())
						if err != nil {
							return err
						}
						if got.AccessLogRetentionDays == nil || *got.AccessLogRetentionDays != 60 {
							return fmt.Errorf("retention not applied, management reports %v", got.AccessLogRetentionDays)
						}
						if !got.RedactPii {
							return fmt.Errorf("redact_pii not applied on the management server")
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_AgentNetworkPolicy_Update(t *testing.T) {
	testE2E(t)
	rName := "ann" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	addr := "netbird_agent_network_policy." + rName

	config := func(description string, groupCap int) string {
		return fmt.Sprintf(`
resource "netbird_group" %[1]q {
  name = "%[1]s-group"
}

resource "netbird_agent_network_provider" %[1]q {
  provider_id       = "openai_api"
  name              = "%[1]s-provider"
  upstream_url      = "https://api.openai.com"
  api_key           = "sk-acc-test"
  bootstrap_cluster = %[2]q
}

resource "netbird_agent_network_guardrail" %[1]q {
  name = "%[1]s-guardrail"
  model_allowlist = {
    enabled = true
    models  = ["gpt-4.1"]
  }
  prompt_capture = {
    enabled = false
  }
}

resource "netbird_agent_network_policy" %[1]q {
  name                     = %[1]q
  description              = %[3]q
  source_groups            = [netbird_group.%[1]s.id]
  destination_provider_ids = [netbird_agent_network_provider.%[1]s.id]
  guardrail_ids            = [netbird_agent_network_guardrail.%[1]s.id]

  token_limit = {
    enabled        = true
    group_cap      = %[4]d
    window_seconds = 86400
  }
}`, rName, testBootstrapCluster, description, groupCap)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("before", 1000000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "description", "before"),
					resource.TestCheckResourceAttr(addr, "token_limit.group_cap", "1000000"),
				),
			},
			{
				Config: config("after", 2000000),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "description", "after"),
					resource.TestCheckResourceAttr(addr, "token_limit.group_cap", "2000000"),
					// The guardrail link must survive an update that does not
					// mention it — the API rebuilds the row from the request.
					resource.TestCheckResourceAttr(addr, "guardrail_ids.#", "1"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[addr].Primary.Attributes["id"]
						policy, err := testAgentNetworkClient().GetPolicy(context.Background(), id)
						if err != nil {
							return err
						}
						if policy.Description != "after" {
							return fmt.Errorf("description not persisted, found %q", policy.Description)
						}
						if len(policy.GuardrailIds) != 1 {
							return fmt.Errorf("guardrail link lost on update, found %#v", policy.GuardrailIds)
						}
						if !policy.Limits.TokenLimit.Enabled || policy.Limits.TokenLimit.GroupCap != 2000000 {
							return fmt.Errorf("token limit not persisted, found %#v", policy.Limits.TokenLimit)
						}
						return nil
					},
				),
			},
		},
	})
}

// Both account-wide singletons implement Delete as a no-op: removing them from
// the configuration must leave the account's settings untouched rather than
// resetting them.
func Test_Singletons_DeleteIsANoOp(t *testing.T) {
	testE2E(t)
	rName := "sing" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)

	var beforeSettings api.AccountSettings

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "netbird_account_settings" %[1]q {
  jwt_groups_enabled = true
}

resource "netbird_dns_settings" %[1]q {
  disabled_management_groups = [%[2]q]
}`, rName, e2eGroupAllID()),
				Check: func(s *terraform.State) error {
					accounts, err := testClient().Accounts.List(context.Background())
					if err != nil {
						return err
					}
					beforeSettings = accounts[0].Settings
					if !valOr(beforeSettings.JwtGroupsEnabled, false) {
						return fmt.Errorf("jwt_groups_enabled was not applied")
					}
					return nil
				},
			},
			{
				Config:  `# both singletons removed from the configuration`,
				Destroy: false,
				Check: func(s *terraform.State) error {
					accounts, err := testClient().Accounts.List(context.Background())
					if err != nil {
						return err
					}
					if !valOr(accounts[0].Settings.JwtGroupsEnabled, false) {
						return fmt.Errorf("removing netbird_account_settings reset the account's jwt_groups_enabled")
					}
					dns, err := testClient().DNS.GetSettings(context.Background())
					if err != nil {
						return err
					}
					if len(dns.DisabledManagementGroups) != 1 || dns.DisabledManagementGroups[0] != e2eGroupAllID() {
						return fmt.Errorf("removing netbird_dns_settings changed the account's DNS settings, found %#v", dns.DisabledManagementGroups)
					}
					return nil
				},
			},
		},
	})
}
