# All parameters are optional but at least one is required, groups matching all
# included criteria are returned
data "netbird_groups" "example" {
  name   = "NetbirdUsersInfraDyn"
  issued = "jwt"
}

# Every OIDC group that exists, i.e. that a member already logged in with
data "netbird_groups" "oidc" {
  issued = "jwt"
}

locals {
  oidc_group_id_by_name = { for g in data.netbird_groups.oidc.groups : g.name => g.id }
}
