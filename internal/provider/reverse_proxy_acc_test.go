package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func Test_ReverseProxyClusters_DataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testEnsureManagementRunning(t)
			_, err := testClient().ReverseProxyClusters.List(context.Background())
			if err != nil {
				t.Skip("reverse proxy clusters endpoint not available")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "netbird_reverse_proxy_clusters" "all" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.netbird_reverse_proxy_clusters.all", "clusters.#"),
				),
			},
		},
	})
}

func Test_ReverseProxyDomain_DataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "netbird_reverse_proxy_domain" "free" {
  type = "free"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.netbird_reverse_proxy_domain.free", "type", "free"),
					resource.TestCheckResourceAttr("data.netbird_reverse_proxy_domain.free", "validated", "true"),
					resource.TestCheckResourceAttrSet("data.netbird_reverse_proxy_domain.free", "domain"),
				),
			},
		},
	})
}

func Test_ReverseProxyDomain_CRUD(t *testing.T) {
	rName := "d" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domainName := rName + ".external.test"
	rNameFull := "netbird_reverse_proxy_domain." + rName

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			domains, err := testClient().ReverseProxyDomains.List(context.Background())
			if err != nil {
				return err
			}
			for _, d := range domains {
				if d.Domain == domainName {
					return fmt.Errorf("domain %s still exists after destroy", domainName)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testReverseProxyDomainResource(rName, domainName, "external.test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "domain", domainName),
					resource.TestCheckResourceAttr(rNameFull, "target_cluster", "external.test"),
					resource.TestCheckResourceAttr(rNameFull, "type", "custom"),
					func(s *terraform.State) error {
						domains, err := testClient().ReverseProxyDomains.List(context.Background())
						if err != nil {
							return err
						}
						id := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						for _, d := range domains {
							if d.Id == id {
								if d.Domain != domainName {
									return fmt.Errorf("domain mismatch: expected %s, got %s", domainName, d.Domain)
								}
								return nil
							}
						}
						return fmt.Errorf("domain %s not found in API", id)
					},
				),
			},
			{
				ResourceName:      rNameFull,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testReverseProxyDomainImportID(rNameFull),
			},
		},
	})
}

func Test_ReverseProxyService_PasswordAuth(t *testing.T) {
	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + ".external.test"
	rNameFull := "netbird_reverse_proxy_service." + rName
	peerID := "peer1"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy: func(s *terraform.State) error {
			services, err := testClient().ReverseProxyServices.List(context.Background())
			if err != nil {
				return err
			}
			for _, svc := range services {
				if svc.Name == rName {
					return fmt.Errorf("service %s still exists", rName)
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: testReverseProxyServicePasswordAuth(rName, domain, peerID, "secret123"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					resource.TestCheckResourceAttr(rNameFull, "domain", domain),
					resource.TestCheckResourceAttr(rNameFull, "enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "targets.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "targets.0.target_id", peerID),
					resource.TestCheckResourceAttr(rNameFull, "targets.0.target_type", "peer"),
					resource.TestCheckResourceAttr(rNameFull, "targets.0.port", "8080"),
					resource.TestCheckResourceAttr(rNameFull, "targets.0.protocol", "http"),
					resource.TestCheckResourceAttr(rNameFull, "auth.password_auth.enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "auth.password_auth.password", "secret123"),
					resource.TestCheckResourceAttrSet(rNameFull, "proxy_cluster"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return fmt.Errorf("get service: %w", err)
						}
						if svc.Name != rName {
							return fmt.Errorf("name mismatch: expected %s, got %s", rName, svc.Name)
						}
						if !svc.Enabled {
							return fmt.Errorf("expected service to be enabled")
						}
						if len(svc.Targets) != 1 {
							return fmt.Errorf("expected 1 target, got %d", len(svc.Targets))
						}
						if svc.Targets[0].TargetId != peerID {
							return fmt.Errorf("target_id mismatch")
						}
						if svc.Auth.PasswordAuth == nil || !svc.Auth.PasswordAuth.Enabled {
							return fmt.Errorf("expected password auth to be enabled")
						}
						return nil
					},
				),
			},
			{
				Config: testReverseProxyServicePasswordAuthUpdated(rName, domain, peerID, "newsecret456"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(rNameFull, "enabled", "false"),
					resource.TestCheckResourceAttr(rNameFull, "auth.password_auth.password", "newsecret456"),
					resource.TestCheckResourceAttr(rNameFull, "pass_host_header", "true"),
					resource.TestCheckResourceAttr(rNameFull, "rewrite_redirects", "true"),
				),
			},
			{
				ResourceName:            rNameFull,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"auth.password_auth.password"},
			},
		},
	})
}

func Test_ReverseProxyService_PinAuth(t *testing.T) {
	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + ".external.test"
	rNameFull := "netbird_reverse_proxy_service." + rName
	peerID := "peer1"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testReverseProxyServicePinAuth(rName, domain, peerID, "9876"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "auth.pin_auth.enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "auth.pin_auth.pin", "9876"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return fmt.Errorf("get service: %w", err)
						}
						if svc.Auth.PinAuth == nil || !svc.Auth.PinAuth.Enabled {
							return fmt.Errorf("expected pin auth to be enabled")
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_ReverseProxyService_BearerAuth(t *testing.T) {
	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + ".external.test"
	rNameFull := "netbird_reverse_proxy_service." + rName
	peerID := "peer1"
	groupID := "group-all"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testReverseProxyServiceBearerAuth(rName, domain, peerID, groupID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "auth.bearer_auth.enabled", "true"),
					resource.TestCheckResourceAttr(rNameFull, "auth.bearer_auth.distribution_groups.#", "1"),
					resource.TestCheckResourceAttr(rNameFull, "auth.bearer_auth.distribution_groups.0", groupID),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return fmt.Errorf("get service: %w", err)
						}
						if svc.Auth.BearerAuth == nil || !svc.Auth.BearerAuth.Enabled {
							return fmt.Errorf("expected bearer auth to be enabled")
						}
						if svc.Auth.BearerAuth.DistributionGroups == nil || len(*svc.Auth.BearerAuth.DistributionGroups) != 1 {
							return fmt.Errorf("expected 1 distribution group")
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_ReverseProxyService_MultipleTargets(t *testing.T) {
	rName := "s" + acctest.RandStringFromCharSet(8, acctest.CharSetAlpha)
	domain := rName + ".external.test"
	rNameFull := "netbird_reverse_proxy_service." + rName
	peerID1 := "peer1"
	peerID2 := "peer2"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testReverseProxyServiceMultiTarget(rName, domain, peerID1, peerID2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "targets.#", "2"),
					func(s *terraform.State) error {
						id := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						svc, err := testClient().ReverseProxyServices.Get(context.Background(), id)
						if err != nil {
							return fmt.Errorf("get service: %w", err)
						}
						if len(svc.Targets) != 2 {
							return fmt.Errorf("expected 2 targets, got %d", len(svc.Targets))
						}
						return nil
					},
				),
			},
		},
	})
}

func Test_AccountSettings_PeerExpose(t *testing.T) {
	groupID := "group-all"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccountSettingsPeerExpose(true, groupID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbird_account_settings.test", "peer_expose_enabled", "true"),
					resource.TestCheckResourceAttr("netbird_account_settings.test", "peer_expose_groups.#", "1"),
					resource.TestCheckResourceAttr("netbird_account_settings.test", "peer_expose_groups.0", groupID),
				),
			},
			{
				Config: testAccountSettingsPeerExpose(false, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbird_account_settings.test", "peer_expose_enabled", "false"),
					resource.TestCheckResourceAttr("netbird_account_settings.test", "peer_expose_groups.#", "0"),
				),
			},
		},
	})
}

// --- Config helpers ---

func testReverseProxyDomainResource(rName, domain, cluster string) string {
	return fmt.Sprintf(`resource "netbird_reverse_proxy_domain" "%s" {
  domain         = %q
  target_cluster = %q
}`, rName, domain, cluster)
}

func testReverseProxyDomainImportID(rNameFull string) func(*terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[rNameFull]
		if !ok {
			return "", fmt.Errorf("resource not found: %s", rNameFull)
		}
		return rs.Primary.Attributes["id"], nil
	}
}

func testReverseProxyServicePasswordAuth(rName, domain, peerID, password string) string {
	return fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%s" {
  name    = %q
  domain  = %q
  enabled = true

  targets = [{
    target_id   = %q
    target_type = "peer"
    port        = 8080
    protocol    = "http"
  }]

  auth = {
    password_auth = {
      enabled  = true
      password = %q
    }
  }
}`, rName, rName, domain, peerID, password)
}

func testReverseProxyServicePasswordAuthUpdated(rName, domain, peerID, password string) string {
	return fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%s" {
  name              = %q
  domain            = %q
  enabled           = false
  pass_host_header  = true
  rewrite_redirects = true

  targets = [{
    target_id   = %q
    target_type = "peer"
    port        = 8080
    protocol    = "http"
  }]

  auth = {
    password_auth = {
      enabled  = true
      password = %q
    }
  }
}`, rName, rName, domain, peerID, password)
}

func testReverseProxyServicePinAuth(rName, domain, peerID, pin string) string {
	return fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%s" {
  name   = %q
  domain = %q

  targets = [{
    target_id   = %q
    target_type = "peer"
    port        = 80
    protocol    = "http"
  }]

  auth = {
    pin_auth = {
      enabled = true
      pin     = %q
    }
  }
}`, rName, rName, domain, peerID, pin)
}

func testReverseProxyServiceBearerAuth(rName, domain, peerID, groupID string) string {
	return fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%s" {
  name   = %q
  domain = %q

  targets = [{
    target_id   = %q
    target_type = "peer"
    port        = 8080
    protocol    = "http"
  }]

  auth = {
    bearer_auth = {
      enabled             = true
      distribution_groups = [%q]
    }
  }
}`, rName, rName, domain, peerID, groupID)
}

func testReverseProxyServiceMultiTarget(rName, domain, peerID1, peerID2 string) string {
	return fmt.Sprintf(`
resource "netbird_reverse_proxy_service" "%s" {
  name   = %q
  domain = %q

  targets = [
    {
      target_id   = %q
      target_type = "peer"
      port        = 8080
      protocol    = "http"
    },
    {
      target_id   = %q
      target_type = "peer"
      port        = 9090
      protocol    = "http"
    },
  ]

  auth = {
    password_auth = {
      enabled  = true
      password = "multitest"
    }
  }
}`, rName, rName, domain, peerID1, peerID2)
}

func testAccountSettingsPeerExpose(enabled bool, groupID string) string {
	groups := "[]"
	if groupID != "" {
		groups = fmt.Sprintf("[%q]", groupID)
	}
	return fmt.Sprintf(`resource "netbird_account_settings" "test" {
  peer_expose_enabled = %t
  peer_expose_groups  = %s
}`, enabled, groups)
}
