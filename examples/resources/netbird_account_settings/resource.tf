resource "netbird_account_settings" "example" {
  jwt_allow_groups                       = []
  jwt_groups_claim_name                  = "claim"
  peer_login_expiration                  = 7200
  peer_inactivity_expiration             = 7200
  peer_login_expiration_enabled          = true
  peer_inactivity_expiration_enabled     = true
  regular_users_view_blocked             = true
  groups_propagation_enabled             = true
  jwt_groups_enabled                     = false
  routing_peer_dns_resolution_enabled    = false
  peer_approval_enabled                  = false
  network_traffic_logs_enabled           = false
  network_traffic_packet_counter_enabled = false
}
