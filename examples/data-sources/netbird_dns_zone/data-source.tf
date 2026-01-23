data "netbird_dns_zone" "by_name" {
  name = "example.local"
}

# Query by ID
data "netbird_dns_zone" "by_id" {
  id = "d50ltp59q2cs73ea7ss0"
}

# Query by domain
data "netbird_dns_zone" "by_domain" {
  domain = "example.local"
}