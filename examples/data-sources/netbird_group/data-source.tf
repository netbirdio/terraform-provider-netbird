# Retrieve group by name
data "netbird_group" "example_a" {
  name = "TF Test"
}

# Retrieve group by ID
data "netbird_group" "example_b" {
  id = "cvikdf3l0ubs73asl4r0"
}

# Retrieve group by ID and Name, useful in case group name is duplicated due to
# SSO sync, name and ID both must match
data "netbird_group" "example_c" {
  name = "TF Test"
  id   = "cvikdf3l0ubs73asl4r0"
}
