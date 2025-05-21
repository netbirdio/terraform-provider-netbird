resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_policy" "example" {
  name    = "TF Test"
  enabled = true

  rule {
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    name          = "TF Test"
    sources       = [netbird_group.example.id]
    destinations  = [netbird_group.example.id]
  }
}
