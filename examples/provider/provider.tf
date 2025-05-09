# Copyright (c) HashiCorp, Inc.

variable "netbird_token" {
  sensitive   = true
  description = "NetBird Management Access Token"
}

terraform {
  required_providers {
    netbird = {
      source = "registry.terraform.io/netbirdio/netbird"
    }
  }
}

provider "netbird" {
  token          = var.netbird_token        # Required
  management_url = "https://api.netbird.io" # Optional, defaults to this value
}
