//go:build e2e

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

func Test_IdentityProvider_Create(t *testing.T) {
	testE2E(t)
	rName := "idp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_identity_provider." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().IdentityProviders.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testIdentityProviderResource(rName, "jumpcloud", "oidc", "client-id", "client-secret", "https://oauth.id.jumpcloud.com/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", "jumpcloud"),
					resource.TestCheckResourceAttr(rNameFull, "type", "oidc"),
					resource.TestCheckResourceAttr(rNameFull, "client_id", "client-id"),
					resource.TestCheckResourceAttr(rNameFull, "client_secret", "client-secret"),
					resource.TestCheckResourceAttr(rNameFull, "issuer", "https://oauth.id.jumpcloud.com/"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						idp, err := testClient().IdentityProviders.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"name":      {"jumpcloud", idp.Name},
							"type":      {api.IdentityProviderTypeOidc, idp.Type},
							"client_id": {"client-id", idp.ClientId},
							"issuer":    {"https://oauth.id.jumpcloud.com/", idp.Issuer},
						})
					},
				),
			},
			{
				ResourceName:            rNameFull,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"client_secret"},
			},
		},
	})
}

func Test_IdentityProvider_Update(t *testing.T) {
	testE2E(t)
	rName := "idp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_identity_provider." + rName
	var createdID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testCheckGone(testClient().IdentityProviders.Get, &createdID),
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testIdentityProviderResource(rName, "jumpcloud", "oidc", "client-id", "client-secret", "https://oauth.id.jumpcloud.com/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					testRecordID(rNameFull, &createdID),
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testIdentityProviderResource(rName, "updated-jumpcloud", "oidc", "new-client-id", "new-secret", "https://oauth.id.jumpcloud.com/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", "updated-jumpcloud"),
					resource.TestCheckResourceAttr(rNameFull, "type", "oidc"),
					resource.TestCheckResourceAttr(rNameFull, "client_id", "new-client-id"),
					resource.TestCheckResourceAttr(rNameFull, "client_secret", "new-secret"),
					resource.TestCheckResourceAttr(rNameFull, "issuer", "https://oauth.id.jumpcloud.com/"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						idp, err := testClient().IdentityProviders.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":      {"updated-jumpcloud", idp.Name},
							"type":      {api.IdentityProviderTypeOidc, idp.Type},
							"client_id": {"new-client-id", idp.ClientId},
							"issuer":    {"https://oauth.id.jumpcloud.com/", idp.Issuer},
						})
					},
				),
			},
		},
	})
}

func testIdentityProviderResource(rName, name, idpType, clientID, clientSecret, issuer string) string {
	return fmt.Sprintf(`resource "netbird_identity_provider" "%s" {
  name          = "%s"
  type          = "%s"
  client_id     = "%s"
  client_secret = "%s"
  issuer        = "%s"
}
`, rName, name, idpType, clientID, clientSecret, issuer)
}
