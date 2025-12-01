# Query by ID
data "netbird_dns_record" "by_id" {
  zone_id = netbird_dns_zone.example.id
  id      = "record-id-here"
}

# Query by name and type
data "netbird_dns_record" "www" {
  zone_id = netbird_dns_zone.example.id
  name    = "www.example.local"
  type    = "A"
}

# Query by name only
data "netbird_dns_record" "api" {
  zone_id = netbird_dns_zone.example.id
  name    = "api.example.local"
}