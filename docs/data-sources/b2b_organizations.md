---
page_title: "stytch_b2b_organizations Data Source - Stytch"
subcategory: "B2B"
description: |-
  List Stytch B2B Organizations with pagination.
---

# stytch_b2b_organizations (Data Source)

Lists B2B Organizations in a project using the B2B Auth API. Supports pagination internally.

## Example Usage

```hcl
data "stytch_b2b_organizations" "all" {
  project_id = var.project_id
}

output "org_count" {
  value = data.stytch_b2b_organizations.all.total_count
}
```

## Argument Reference

- `project_id` (String, Required)
- `project_secret` (String, Optional, Sensitive)

## Attribute Reference

- `total_count` (Number)
- `items` (List of Object): `id`, `name`, `slug`, `created_at`, `updated_at`.


