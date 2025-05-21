resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_nameserver_group" "example" {
  name        = "TF Test"
  description = "TF Test"
  nameservers = [
    {
      ip      = "8.8.8.8"
      ns_type = "udp"
      port    = 53
    }
  ]
  groups                 = [netbird_group.example.id]
  search_domains_enabled = false
}