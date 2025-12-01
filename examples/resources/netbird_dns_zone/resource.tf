resource "netbird_dns_zone" "internal" {
  name                 = "internal-zone"
  domain               = "internal.company.com"
  enabled              = true
  enable_search_domain = true
  distribution_groups  = [netbird_group.dev.id, netbird_group.ops.id]
}