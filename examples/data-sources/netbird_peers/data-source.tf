data "netbird_group" "example" {
  name = "Test"
}

data "netbird_peers" "example" {
  # All parameters are optional, peers matching all included criteria are return in ids field
  name                          = "Production"
  ip                            = "1.2.3.4"
  connection_ip                 = "12.2.3.4"
  dns_label                     = "test.local"
  user_id                       = ""
  hostname                      = "prod"
  country_code                  = "EG"
  city_name                     = "Cairo"
  os                            = "Ubuntu 24.04"
  connected                     = false
  ssh_enabled                   = true
  inactivity_expiration_enabled = false
  approval_required             = false
  login_expiration_enabled      = false
  login_expired                 = false
  geoname_id                    = 360630
  # Peers containing all groups mentioned are included, even if they have more groups attached
  groups = [data.netbird_group.example.id]
}
