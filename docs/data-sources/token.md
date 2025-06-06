---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_token Data Source - netbird"
subcategory: ""
description: |-
  Read Personal Access Token metadata, see NetBird Docs https://docs.netbird.io/how-to/access-netbird-public-api#creating-an-access-token for more information.
---

# netbird_token (Data Source)

Read Personal Access Token metadata, see [NetBird Docs](https://docs.netbird.io/how-to/access-netbird-public-api#creating-an-access-token) for more information.

## Example Usage

```terraform
data "netbird_user" "example" {
  self = true
}

# Retrieve by name
data "netbird_token" "example" {
  user_id = data.netbird_user.example.id
  name    = "TF Test"
}

# Retrieve by ID
data "netbird_token" "example" {
  user_id = data.netbird_user.example.id
  id      = "d07s41rl0ubs73cg61eg"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `user_id` (String) User ID

### Optional

- `id` (String) Token ID
- `name` (String) Token Name

### Read-Only

- `created_at` (String) Creation timestamp
- `expiration_date` (String) Token Expiration Date
- `last_used` (String) Last usage time
