# Retrieve route by network_id
data "netbird_route" "example_a" {
  network_id = "TF Test"
}

# Retrieve route by ID
data "netbird_route" "example_b" {
  id = "cvikdf3l0ubs73asl4r0"
}
