resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_network" "example" {
  name        = "TF Test"
  description = "TF Test"
}

resource "netbird_network_resource" "example" {
  network_id  = netbird_network.example.id
  name        = "TF Test"
  description = "TF Test"
  address     = "www.example.com"
  groups      = [netbird_group.example.id]
  enabled     = true
}
