data "netbird_reverse_proxy_domain" "free" {
  type = "free"
}

resource "netbird_reverse_proxy_service" "example" {
  name   = "web-app"
  domain = data.netbird_reverse_proxy_domain.free.domain

  targets {
    target_id   = netbird_peer.web.id
    target_type = "peer"
    port        = 8080
    protocol    = "http"
  }

  auth {
    link_auth {
      enabled = true
    }
  }
}
