# Retrieve by id
data "netbird_scim" "example" {
  id = "1"
}

# Retrieve by provider name
data "netbird_scim" "example" {
  provider_name = "okta"
}
