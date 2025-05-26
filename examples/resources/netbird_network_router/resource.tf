resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_network" "example" {
  name        = "TF Test"
  description = "TF Test"
}

resource "netbird_network_router" "example" {
  network_id  = netbird_network.example.id
  peer_groups = [netbird_group.example.id]
  metric      = 9999
  enabled     = true
  masquerade  = true
}
