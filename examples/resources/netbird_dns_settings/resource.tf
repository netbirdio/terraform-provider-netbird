resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_dns_settings" "main" {
  disabled_management_groups = [netbird_group.example.id]
}