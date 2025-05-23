---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_setup_key Data Source - netbird"
subcategory: ""
description: |-
  Read SetupKey settings and metadata, see NetBird Docs https://docs.netbird.io/how-to/register-machines-using-setup-keys for more information.
---

# netbird_setup_key (Data Source)

Read SetupKey settings and metadata, see [NetBird Docs](https://docs.netbird.io/how-to/register-machines-using-setup-keys) for more information.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) SetupKey ID

### Read-Only

- `allow_extra_dns_labels` (Boolean)
- `auto_groups` (List of String)
- `ephemeral` (Boolean)
- `expires` (String) SetupKey Expiration Date
- `key` (String) Plaintext setup key
- `last_used` (String) Last usage time
- `name` (String) SetupKey Name
- `revoked` (Boolean)
- `state` (String)
- `type` (String)
- `updated_at` (String) Creation timestamp
- `usage_limit` (Number)
- `used_times` (Number)
- `valid` (Boolean)
