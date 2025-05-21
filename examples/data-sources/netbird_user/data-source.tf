# Retrieve by ID
data "netbird_user" "example" {
  id = "926c2f89-ebc9-4a1b-9cf9-f53a20d06f6c"
}

# Retrieve by Name
data "netbird_user" "example" {
  name = "John Doe"
}

# Retrieve by Email
data "netbird_user" "example" {
  email = "johndoe@company.co"
}
