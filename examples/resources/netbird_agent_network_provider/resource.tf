# The account's Agent Network settings row — which supplies the endpoint agents
# call — is created only when a provider is created with `bootstrap_cluster` set.
# Creating providers without it leaves the account unbootstrapped, so set it on
# at least the first provider. It is ignored once the account is bootstrapped.
data "netbird_reverse_proxy_clusters" "all" {}

resource "netbird_agent_network_provider" "openai" {
  provider_id  = "openai_api"
  name         = "OpenAI Production"
  upstream_url = "https://api.openai.com"
  api_key      = var.openai_api_key

  bootstrap_cluster = data.netbird_reverse_proxy_clusters.all.clusters[0].address

  models = [
    {
      id            = "gpt-4o-mini"
      input_per_1k  = 0.00015
      output_per_1k = 0.0006
    },
    {
      id            = "gpt-4o"
      input_per_1k  = 0.005
      output_per_1k = 0.015
    },
  ]
}
