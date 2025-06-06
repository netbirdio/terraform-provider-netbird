---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "netbird_user Data Source - netbird"
subcategory: ""
description: |-
  Read Existing Users metadata, see NetBird Docs https://docs.netbird.io/how-to/add-users-to-your-network for more information.
---

# netbird_user (Data Source)

Read Existing Users metadata, see [NetBird Docs](https://docs.netbird.io/how-to/add-users-to-your-network) for more information.

## Example Usage

```terraform
# Retrieve by ID
data "netbird_user" "example" {
  id = "926c2f89-ebc9-4a1b-9cf9-f53a20d06f6c"
}

# Retrieve by Name
data "netbird_user" "example" {
  name = "John Doe"
}

# Retrieve by Email
data "netbird_user" "example" {
  email = "johndoe@company.co"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `email` (String) User Email
- `id` (String) The unique identifier of a user
- `name` (String) User Name

### Read-Only

- `auto_groups` (List of String) Group IDs to auto-assign to peers registered by this user
- `is_blocked` (Boolean) If set to true then user is blocked and can't use the system
- `is_current` (Boolean) Set to true if the caller user is the same as the resource user
- `is_service_user` (Boolean) If set to true, user is a Service Account User
- `issued` (String) User issue method
- `last_login` (String) User Last Login timedate
- `role` (String) User's NetBird account role (owner|admin|user|billing_admin|auditor|network_admin).
- `status` (String) User status (active or invited)
