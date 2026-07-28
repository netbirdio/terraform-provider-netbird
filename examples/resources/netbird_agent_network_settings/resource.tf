resource "netbird_agent_network_settings" "main" {
  enable_log_collection     = true
  enable_prompt_collection  = false
  redact_pii                = false
  access_log_retention_days = 90
}
