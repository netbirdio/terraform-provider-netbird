resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_peer" "example" {
  id                            = "d057h0jl0ubs73cftnp0"
  ssh_enabled                   = false
  name                          = "Production"
  groups                        = [netbird_group.example.id]
  inactivity_expiration_enabled = false
  approval_required             = false
  login_expiration_enabled      = false
  prevent_delete                = true
}
