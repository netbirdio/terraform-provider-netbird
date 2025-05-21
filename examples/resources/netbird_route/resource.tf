resource "netbird_route" "example" {
  network_id            = "Example"
  groups                = [data.netbird_group.example_a.id]
  access_control_groups = [data.netbird_group.example_b.id]
  description           = "Example Route"
  domains               = ["www.example.com"]
  peer                  = data.netbird_peer.example.id
}

resource "netbird_route" "example" {
  network_id            = "Example"
  groups                = [data.netbird_group.example_a.id]
  access_control_groups = [data.netbird_group.example_b.id]
  description           = "Example Route"
  network               = "10.0.0.0/8"
  peer_groups           = [data.netbird_group.example_c.id]
}
