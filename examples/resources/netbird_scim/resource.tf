resource "netbird_scim" "example" {
  provider_name       = "okta"
  prefix              = "okta-scim"
  enabled             = true
  group_prefixes      = ["engineering", "product"]
  user_group_prefixes = ["users"]
}
