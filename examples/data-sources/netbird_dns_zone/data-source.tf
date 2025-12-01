data "netbird_dns_zone" "example" {
  name = "example-zone"
}

# Query by ID
data "netbird_dns_zone" "by_id" {
  id = "zone-id-here"
}

# Query by domain
data "netbird_dns_zone" "by_domain" {
  domain = "example.local"
}