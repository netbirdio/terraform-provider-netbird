data "netbird_network" "example" {
  name = "TF Test"
}

data "netbird_network_resource" "example" {
  network_id = netbird_network.example.id
  name       = "TF Test"
}
