data "netbird_reverse_proxy_clusters" "all" {}

resource "netbird_reverse_proxy_domain" "example" {
  domain         = "app.example.com"
  target_cluster = data.netbird_reverse_proxy_clusters.all.clusters[0].address
}
