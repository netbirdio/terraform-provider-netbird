# Retrieve by domain name
data "netbird_reverse_proxy_domain" "example" {
  domain = "app.example.netbird.cloud"
}

# Retrieve the free domain
data "netbird_reverse_proxy_domain" "free" {
  type = "free"
}

# Retrieve by ID
data "netbird_reverse_proxy_domain" "by_id" {
  id = "cvit5sjl0ubs73aslrkg"
}
