# Bootstrapping the account's Agent Network gateway. Set exactly one address:
#
#   proxy_address — allocate an endpoint one label beneath a shared proxy
#                   cluster, which is the usual choice.
#   endpoint      — claim a hostname outright, served by a proxy dedicated to
#                   this account.
#
# Both are assigned once and immutable afterwards, so changing either replaces
# the resource: the endpoint is released and a new one allocated, which requires
# the account's Agent Network providers to be destroyed first.
data "netbird_reverse_proxy_clusters" "all" {}

resource "netbird_agent_network_settings" "main" {
  proxy_address = data.netbird_reverse_proxy_clusters.all.clusters[0].address

  enable_log_collection     = true
  enable_prompt_collection  = false
  redact_pii                = false
  access_log_retention_days = 90
}

# The hostname agents are configured with.
output "agent_network_endpoint" {
  value = netbird_agent_network_settings.main.endpoint
}
