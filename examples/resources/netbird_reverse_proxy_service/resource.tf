data "netbird_reverse_proxy_domain" "free" {
  type = "free"
}

# HTTP (L7) reverse proxy service with per-target options
resource "netbird_reverse_proxy_service" "web_app" {
  name   = "web-app"
  domain = data.netbird_reverse_proxy_domain.free.domain

  targets = [{
    target_id   = netbird_peer.web.id
    target_type = "peer"
    port        = 8080
    protocol    = "https"

    options = {
      skip_tls_verify = true
      request_timeout = "30s"
      path_rewrite    = "preserve"
      custom_headers = {
        "X-Forwarded-Proto" = "https"
      }
    }
  }]

  auth = {
    password_auth = {
      enabled  = true
      password = var.web_app_password
    }
  }
}

# TCP (L4) proxy service with proxy protocol
resource "netbird_reverse_proxy_service" "postgres" {
  name        = "postgres"
  domain      = "pg.${data.netbird_reverse_proxy_domain.free.domain}"
  mode        = "tcp"
  listen_port = 15432

  targets = [{
    target_id   = netbird_network_resource.db.id
    target_type = "subnet"
    host        = "10.0.0.5"
    port        = 5432
    protocol    = "tcp"

    options = {
      proxy_protocol  = true
      request_timeout = "60s"
    }
  }]
}

# UDP (L4) proxy service with session idle timeout
resource "netbird_reverse_proxy_service" "dns" {
  name        = "dns"
  domain      = "dns.${data.netbird_reverse_proxy_domain.free.domain}"
  mode        = "udp"
  listen_port = 19053

  targets = [{
    target_id   = netbird_network_resource.infra.id
    target_type = "subnet"
    host        = "10.0.0.6"
    port        = 53
    protocol    = "udp"

    options = {
      session_idle_timeout = "2m"
    }
  }]
}

# TLS (SNI passthrough) proxy service
resource "netbird_reverse_proxy_service" "tls_backend" {
  name        = "tls-backend"
  domain      = "backend.${data.netbird_reverse_proxy_domain.free.domain}"
  mode        = "tls"
  listen_port = 14443

  targets = [{
    target_id   = netbird_network_resource.backend.id
    target_type = "subnet"
    host        = "10.0.0.7"
    port        = 8443
    protocol    = "tcp"

    options = {
      proxy_protocol = true
    }
  }]
}
