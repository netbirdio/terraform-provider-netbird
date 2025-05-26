data "netbird_network" "example" {
  name = "TF Test"
}

resource "netbird_network_router" "example" {
  id         = "cvr9ic3l0ubs73c11gs0"
  network_id = netbird_network.example.id
}
