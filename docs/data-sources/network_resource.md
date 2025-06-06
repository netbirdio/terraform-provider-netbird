---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_network_resource Data Source - netbird"
subcategory: ""
description: |-
  Read Network Resource settings and metadata, see NetBird Docs https://docs.netbird.io/how-to/networks#resources for more information.
---

# netbird_network_resource (Data Source)

Read Network Resource settings and metadata, see [NetBird Docs](https://docs.netbird.io/how-to/networks#resources) for more information.

## Example Usage

```terraform
data "netbird_network" "example" {
  name = "TF Test"
}

data "netbird_network_resource" "example" {
  network_id = netbird_network.example.id
  name       = "TF Test"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `network_id` (String) The unique identifier of a network

### Optional

- `id` (String) The unique identifier of a resource
- `name` (String) NetworkResource Name

### Read-Only

- `address` (String) Network resource address (either a direct host like 1.1.1.1 or 1.1.1.1/32, or a subnet like 192.168.178.0/24, or domains like example.com and *.example.com)
- `description` (String) NetworkResource Description
- `enabled` (Boolean) NetworkResource status
- `groups` (List of String) Group IDs containing the resource
