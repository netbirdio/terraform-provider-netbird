data "netbird_user" "example" {
  self = true
}

# Retrieve by name
data "netbird_token" "example" {
  user_id = data.netbird_user.example.id
  name    = "TF Test"
}

# Retrieve by ID
data "netbird_token" "example" {
  user_id = data.netbird_user.example.id
  id      = "d07s41rl0ubs73cg61eg"
}