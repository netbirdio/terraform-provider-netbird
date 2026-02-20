resource "netbird_identity_provider" "example" {
  name          = "example"
  type          = "oidc"
  client_id     = "client-id"
  client_secret = "client-secret"
  issuer        = "https://auth.example.com"
}
