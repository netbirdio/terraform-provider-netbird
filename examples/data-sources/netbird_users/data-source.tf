# All parameters are optional but at least one is required, users matching all
# included criteria are returned
data "netbird_users" "example" {
  role            = "admin"
  status          = "active"
  is_service_user = false
  is_blocked      = false
  # Users having all the auto groups mentioned are included, even if they have more
  auto_groups = ["ch8i4ug6lnn4g9hqv7m0"]
}

data "netbird_users" "service" {
  is_service_user = true
}
