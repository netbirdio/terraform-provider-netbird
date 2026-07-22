resource "netbird_agent_network_provider" "openai" {
  provider_id  = "openai_api"
  name         = "OpenAI Production"
  upstream_url = "https://api.openai.com"
  api_key      = var.openai_api_key

  models = [
    {
      id           = "gpt-4o-mini"
      input_per_1k = 0.00015
      output_per_1k = 0.0006
    },
    {
      id           = "gpt-4o"
      input_per_1k = 0.005
      output_per_1k = 0.015
    },
  ]
}
