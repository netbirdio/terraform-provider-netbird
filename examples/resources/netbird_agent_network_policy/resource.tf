resource "netbird_agent_network_policy" "engineering" {
  name        = "Engineering → OpenAI"
  description = "Allow engineers to call OpenAI with a monthly token budget."
  enabled     = true

  source_groups            = [netbird_group.engineering.id]
  destination_provider_ids = [netbird_agent_network_provider.openai.id]
  guardrail_ids            = [netbird_agent_network_guardrail.strict.id]

  token_limit = {
    enabled        = true
    group_cap      = 50000000
    user_cap       = 5000000
    window_seconds = 2592000
  }

  budget_limit = {
    enabled        = true
    group_cap_usd  = 500.0
    user_cap_usd   = 50.0
    window_seconds = 2592000
  }
}
