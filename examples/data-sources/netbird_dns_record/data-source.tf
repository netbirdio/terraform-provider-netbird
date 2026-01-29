# Query by ID
data "netbird_dns_record" "by_id" {
  zone_id = netbird_dns_zone.example.id
  id      = "d50moeqtvgfc7398k38g"
}

# Query by name and type
data "netbird_dns_record" "by_name_and_type" {
  zone_id = netbird_dns_zone.example.id
  name    = "www.example.local"
  type    = "A"
}

# Query by name only
data "netbird_dns_record" "by_name" {
  zone_id = netbird_dns_zone.example.id
  name    = "api.example.local"
}