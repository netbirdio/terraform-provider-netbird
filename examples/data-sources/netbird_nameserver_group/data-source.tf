# Retrieve by ID
data "netbird_nameserver_group" "example" {
  id = "cuqekmjl0ubs73cfjmvg"
}

# Retrieve by Name
data "netbird_nameserver_group" "example" {
  name = "TF Test"
}
