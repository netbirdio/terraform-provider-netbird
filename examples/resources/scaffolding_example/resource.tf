# Copyright (c) HashiCorp, Inc.

resource "netbird_account" "example" {
  jwt_allow_groups = false
}

resource "netbird_peer" "example" {
  id          = "d057h0jl0ubs73cftnp0"
  ssh_enabled = false
}

resource "netbird_token" "example" {
  user_id         = "2fd8f4d6-d6c2-44c3-b7a4-adb644177b3d"
  name            = "TF Test"
  expiration_days = 1
}

resource "netbird_setup_key" "example" {
  name           = "TF Test"
  expiry_seconds = 86400
  type           = "reusable"
}

resource "netbird_group" "example" {
  name = "TF Test"
  peers = [
    netbird_peer.example.id,
  ]
}

resource "netbird_policy" "example" {
  name    = "TF Test"
  enabled = true

  rule {
    action        = "accept"
    bidirectional = true
    enabled       = true
    protocol      = "tcp"
    name          = "TF Test"
    sources       = [netbird_group.example.id]
    destinations  = [netbird_group.example.id]
  }
}

resource "netbird_posture_check" "example" {
  name        = "TF Test"
  description = "Meow"

  netbird_version_check {
    min_version = "0.1.0"
  }

  os_version_check {
    android_min_version        = "0.0.0"
    ios_min_version            = "0.0.0"
    darwin_min_version         = "0.0.0"
    linux_min_kernel_version   = "0.0.0"
    windows_min_kernel_version = "0.0.1"
  }

  geo_location_check {
    locations = [
      {
        country_code = "EG"
      },
      {
        country_code = "DE"
      }
    ]
    action = "allow"
  }

  peer_network_range_check {
    ranges = [
      "0.0.0.0/0"
    ]

    action = "allow"
  }

  process_check {
    linux_path = "/some/path/in/linux"
    mac_path   = "/some/path/in/mac"
  }

  process_check {
    linux_path   = "/some/path/in/linux"
    windows_path = "C:\\some\\path\\in\\windows"
  }
}

resource "netbird_network" "example" {
  name        = "TF Test"
  description = "TF Test"
}

resource "netbird_network_resource" "example" {
  network_id  = netbird_network.example.id
  name        = "TF Test"
  description = "TF Test"
  address     = "www.example.com"
  groups      = [netbird_group.example.id]
}

resource "netbird_network_router" "example" {
  network_id  = netbird_network.example.id
  peer_groups = [netbird_group.example.id]
  metric      = 9999
}

resource "netbird_nameserver_group" "example" {
  name        = "TF Test"
  description = "TF Test"
  nameservers = [{
    ip      = "8.8.8.8"
    ns_type = "udp"
    port    = 53
  }]
  groups                 = [netbird_group.example.id]
  search_domains_enabled = false
}