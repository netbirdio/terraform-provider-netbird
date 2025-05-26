data "netbird_user" "example" {
  self = true
}

resource "netbird_token" "example" {
  user_id         = data.netbird_user.example.id
  name            = "TF Test"
  expiration_days = 1
}