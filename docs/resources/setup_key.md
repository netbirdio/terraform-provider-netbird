---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_setup_key Resource - netbird"
subcategory: ""
description: |-
  Create and Manage Setup Keys, see NetBird Docs https://docs.netbird.io/how-to/register-machines-using-setup-keys for more information.
---

# netbird_setup_key (Resource)

Create and Manage Setup Keys, see [NetBird Docs](https://docs.netbird.io/how-to/register-machines-using-setup-keys) for more information.

## Example Usage

```terraform
resource "netbird_group" "example" {
  name = "Test"
}

resource "netbird_setup_key" "example" {
  name                   = "TF Test"
  expiry_seconds         = 86400
  type                   = "reusable"
  allow_extra_dns_labels = true
  auto_groups            = [netbird_group.example.id]
  ephemeral              = false
  revoked                = false
  usage_limit            = 0
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) SetupKey Name

### Optional

- `allow_extra_dns_labels` (Boolean) Allow extra DNS labels to be added to the peer
- `auto_groups` (List of String) List of groups to automatically assign to peers created through this setup key
- `ephemeral` (Boolean) Indicate that the peer will be ephemeral or not, ephemeral peers are deleted after 10 minutes of inactivity
- `expiry_seconds` (Number) Expiry time in seconds (0 is unlimited)
- `revoked` (Boolean) Set to true to revoke setup key
- `type` (String) Setup Key type (one-off or reusable)
- `usage_limit` (Number) Maximum number of times SetupKey can be used (0 for unlimited)

### Read-Only

- `expires` (String) SetupKey Expiration Date
- `id` (String) SetupKey ID
- `key` (String, Sensitive) Plaintext setup key
- `last_used` (String) Last usage time
- `state` (String) Setup key state (valid or expired)
- `updated_at` (String) Creation timestamp
- `used_times` (Number) Number of times Setup Key was used
- `valid` (Boolean) True if setup key can be used to create more Peers

## Import

Import is supported using the following syntax:

```shell
terraform import netbird_setup_key.example setup_key_id

# For example

terraform import netbird_setup_key.example cvr9ibrl0ubs73c11gr0
```
