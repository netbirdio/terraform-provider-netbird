---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_token Data Source - netbird"
subcategory: ""
description: |-
  Read Personal Access Token metadata, see NetBird Docs https://docs.netbird.io/how-to/access-netbird-public-api#creating-an-access-token for more information.
---

# netbird_token (Data Source)

Read Personal Access Token metadata, see [NetBird Docs](https://docs.netbird.io/how-to/access-netbird-public-api#creating-an-access-token) for more information.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) Token ID
- `user_id` (String) User ID

### Read-Only

- `created_at` (String) Creation timestamp
- `expiration_date` (String) Token Expiration Date
- `last_used` (String) Last usage time
- `name` (String) Token Name
