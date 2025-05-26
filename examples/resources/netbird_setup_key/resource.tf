resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_setup_key" "example" {
  name                   = "TF Test"
  expiry_seconds         = 86400
  type                   = "reusable"
  allow_extra_dns_labels = true
  auto_groups            = [netbird_group.example.id]
  ephemeral              = false
  revoked                = false
  usage_limit            = 0
}
