---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_nameserver_group Data Source - netbird"
subcategory: ""
description: |-
  Read Nameserver Group settings
---

# netbird_nameserver_group (Data Source)

Read Nameserver Group settings



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) NameserverGroup ID

### Read-Only

- `description` (String) Description of the nameserver group
- `domains` (List of String) Match domain list. It should be empty only if primary is true.
- `enabled` (Boolean) Nameserver group status
- `groups` (List of String) Distribution group IDs that defines group of peers that will use this nameserver group
- `name` (String) Name of nameserver group
- `nameservers` (Attributes List) Nameserver list (see [below for nested schema](#nestedatt--nameservers))
- `primary` (Boolean) Defines if a nameserver group is primary that resolves all domains. It should be true only if domains list is empty.
- `search_domains_enabled` (Boolean) Search domain status for match domains. It should be true only if domains list is not empty.

<a id="nestedatt--nameservers"></a>
### Nested Schema for `nameservers`

Read-Only:

- `ip` (String) Nameserver IP
- `ns_type` (String) Nameserver Type
- `port` (Number) Nameserver Port
