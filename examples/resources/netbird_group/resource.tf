data "netbird_peer" "example" {
  ip = "1.2.3.4"
}

resource "netbird_group" "example" {
  name = "TF Test"
  peers = [
    data.netbird_peer.example.id,
  ]
}
