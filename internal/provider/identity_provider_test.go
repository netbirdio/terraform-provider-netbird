package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_identityProviderAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.IdentityProvider
		expected IdentityProviderModel
	}{
		{
			resource: &api.IdentityProvider{
				Id:       valPtr("idp1"),
				Name:     "OIDC Provider",
				Type:     api.IdentityProviderTypeOidc,
				ClientId: "client-id",
				Issuer:   "https://auth.example.com",
			},
			expected: IdentityProviderModel{
				Id:       types.StringValue("idp1"),
				Name:     types.StringValue("OIDC Provider"),
				Type:     types.StringValue("oidc"),
				ClientId: types.StringValue("client-id"),
				Issuer:   types.StringValue("https://auth.example.com"),
			},
		},
		{
			resource: &api.IdentityProvider{
				Id:       valPtr("idp2"),
				Name:     "Okta",
				Type:     api.IdentityProviderTypeOkta,
				ClientId: "okta-client",
				Issuer:   "https://dev-123.okta.com",
			},
			expected: IdentityProviderModel{
				Id:       types.StringValue("idp2"),
				Name:     types.StringValue("Okta"),
				Type:     types.StringValue("okta"),
				ClientId: types.StringValue("okta-client"),
				Issuer:   types.StringValue("https://dev-123.okta.com"),
			},
		},
	}

	for _, c := range cases {
		var out IdentityProviderModel
		// Preserve client_secret as it's write-only
		out.ClientSecret = types.StringValue("preserved-secret")
		identityProviderAPIToTerraform(c.resource, &out)

		// client_secret should be preserved from existing state
		c.expected.ClientSecret = types.StringValue("preserved-secret")
		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_identityProviderTerraformToAPI(t *testing.T) {
	cases := []struct {
		data     IdentityProviderModel
		expected api.IdentityProviderRequest
	}{
		{
			data: IdentityProviderModel{
				Id:           types.StringValue("idp1"),
				Name:         types.StringValue("OIDC Provider"),
				Type:         types.StringValue("oidc"),
				ClientId:     types.StringValue("client-id"),
				ClientSecret: types.StringValue("secret"),
				Issuer:       types.StringValue("https://auth.example.com"),
			},
			expected: api.IdentityProviderRequest{
				Name:         "OIDC Provider",
				Type:         api.IdentityProviderTypeOidc,
				ClientId:     "client-id",
				ClientSecret: "secret",
				Issuer:       "https://auth.example.com",
			},
		},
		{
			data: IdentityProviderModel{
				Name:         types.StringValue("Entra ID"),
				Type:         types.StringValue("entra"),
				ClientId:     types.StringValue("entra-client"),
				ClientSecret: types.StringValue("entra-secret"),
				Issuer:       types.StringValue("https://login.microsoftonline.com/tenant/v2.0"),
			},
			expected: api.IdentityProviderRequest{
				Name:         "Entra ID",
				Type:         api.IdentityProviderTypeEntra,
				ClientId:     "entra-client",
				ClientSecret: "entra-secret",
				Issuer:       "https://login.microsoftonline.com/tenant/v2.0",
			},
		},
	}

	for _, c := range cases {
		out := identityProviderTerraformToAPI(context.Background(), c.data)
		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_IdentityProvider_Create(t *testing.T) {
	rName := "idp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_identity_provider." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testIdentityProviderResource(rName, "OIDC Provider", "oidc", "client-id", "client-secret", "https://auth.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", "OIDC Provider"),
					resource.TestCheckResourceAttr(rNameFull, "type", "oidc"),
					resource.TestCheckResourceAttr(rNameFull, "client_id", "client-id"),
					resource.TestCheckResourceAttr(rNameFull, "client_secret", "client-secret"),
					resource.TestCheckResourceAttr(rNameFull, "issuer", "https://auth.example.com"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						idp, err := testClient().IdentityProviders.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						return matchPairs(map[string][]any{
							"name":      {"OIDC Provider", idp.Name},
							"type":      {api.IdentityProviderTypeOidc, idp.Type},
							"client_id": {"client-id", idp.ClientId},
							"issuer":    {"https://auth.example.com", idp.Issuer},
						})
					},
				),
			},
		},
	})
}

func Test_IdentityProvider_Update(t *testing.T) {
	rName := "idp" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_identity_provider." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testIdentityProviderResource(rName, "OIDC Provider", "oidc", "client-id", "client-secret", "https://auth.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testIdentityProviderResource(rName, "Updated Provider", "oidc", "new-client-id", "new-secret", "https://auth2.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", "Updated Provider"),
					resource.TestCheckResourceAttr(rNameFull, "type", "oidc"),
					resource.TestCheckResourceAttr(rNameFull, "client_id", "new-client-id"),
					resource.TestCheckResourceAttr(rNameFull, "client_secret", "new-secret"),
					resource.TestCheckResourceAttr(rNameFull, "issuer", "https://auth2.example.com"),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						idp, err := testClient().IdentityProviders.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						return matchPairs(map[string][]any{
							"name":      {"Updated Provider", idp.Name},
							"type":      {api.IdentityProviderTypeOidc, idp.Type},
							"client_id": {"new-client-id", idp.ClientId},
							"issuer":    {"https://auth2.example.com", idp.Issuer},
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
