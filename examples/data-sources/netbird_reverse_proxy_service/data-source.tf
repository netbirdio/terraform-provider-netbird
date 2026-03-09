# Retrieve by name
data "netbird_reverse_proxy_service" "example" {
  name = "my-service"
}

# Retrieve by domain
data "netbird_reverse_proxy_service" "by_domain" {
  domain = "app.example.netbird.cloud"
}

# Retrieve by ID
data "netbird_reverse_proxy_service" "by_id" {
  id = "cvit5sjl0ubs73aslrkg"
}
