resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_user" "service_user" {
  is_service_user = true
  name            = "Test"
  auto_groups     = [netbird_group.example.id]
  is_blocked      = false
  role            = "admin"
}

resource "netbird_user" "real_user" {
  is_service_user = false
  name            = "John Doe"
  email           = "johndoe@company.co"
  auto_groups     = [netbird_group.example.id]
  is_blocked      = false
  role            = "admin"
}