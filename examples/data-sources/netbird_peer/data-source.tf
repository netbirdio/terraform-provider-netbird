# Retrieve by IP
data "netbird_peer" "example" {
  ip = "1.2.3.4"
}

# Retrieve by Name
data "netbird_peer" "example" {
  name = "Production"
}

# Retrieve by ID
data "netbird_peer" "example" {
  id = "d057h0jl0ubs73cftnp0"
}
