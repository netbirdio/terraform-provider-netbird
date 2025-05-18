# Copyright (c) HashiCorp, Inc.

data "netbird_account_settings" "example" {
}

data "netbird_peer" "example" {
  id = "d057h0jl0ubs73cftnp0"
}

data "netbird_peers" "example" {
  connected = false
  groups    = ["All"]
}

data "netbird_token" "example" {
  id      = "d07s41rl0ubs73cg61eg"
  user_id = "2fd8f4d6-d6c2-44c3-b7a4-adb644177b3d"
}

data "netbird_setup_key" "example" {
  id = "4206669531"
}

data "netbird_group" "example" {
  id = "d08le93l0ubs73cg8mn0"
}

data "netbird_policy" "example" {
  id = "d09825rl0ubs73cgb37g"
}

data "netbird_posture_check" "example" {
  id = "d0bsu9rl0ubs73dj2ee0"
}

data "netbird_network" "example" {
  id = "d0btcdjl0ubs73dj2frg"
}

data "netbird_network_resource" "example" {
  id         = "d0bthnjl0ubs73dj2glg"
  network_id = data.netbird_network.example.id
}

data "netbird_network_router" "example" {
  id         = "d0btiobl0ubs73dj2go0"
  network_id = data.netbird_network.example.id
}

data "netbird_nameserver_group" "example" {
  id = "d0cfcubl0ubs738c7h6g"
}