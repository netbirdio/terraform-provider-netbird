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

resource "netbird_policy" "ssh_example" {
  name    = "SSH Access"
  enabled = true

  rule {
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "netbird-ssh"
    name          = "SSH Rule"
    sources       = [netbird_group.example.id]
    destinations  = [netbird_group.example.id]

    authorized_groups = {
      (netbird_group.example.id) = ["example"]
    }
  }
}
