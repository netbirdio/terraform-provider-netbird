# A Record example
resource "netbird_dns_record" "www" {
  zone_id = netbird_dns_zone.example.id
  name    = "www.example.local"
  type    = "A"
  content = "192.168.1.100"
  ttl     = 300
}

# AAAA Record example (IPv6)
resource "netbird_dns_record" "ipv6" {
  zone_id = netbird_dns_zone.example.id
  name    = "api.example.local"
  type    = "AAAA"
  content = "2001:db8::1"
  ttl     = 600
}

# CNAME Record example
resource "netbird_dns_record" "mail" {
  zone_id = netbird_dns_zone.example.id
  name    = "mail.example.local"
  type    = "CNAME"
  content = "mail.external.com"
  ttl     = 300
}

# Wildcard Record example
resource "netbird_dns_record" "wildcard" {
  zone_id = netbird_dns_zone.example.id
  name    = "*.example.local"
  type    = "A"
  content = "10.10.1.2"
  ttl     = 300
}