---
page_title: "stytch_b2b_organization Data Source - Stytch"
subcategory: "B2B"
description: |-
  Read a Stytch B2B Organization by ID or slug via the B2B Auth API.
---

# stytch_b2b_organization (Data Source)

Reads a Stytch B2B Organization by `organization_id` or `slug`.

## Example Usage

```hcl
data "stytch_b2b_organization" "org" {
  project_id      = var.project_id
  organization_id = var.organization_id
}

output "org_name" {
  value = data.stytch_b2b_organization.org.name
}
```

Lookup by slug:
```hcl
data "stytch_b2b_organization" "by_slug" {
  project_id = var.project_id
  slug       = "mento"
}
```

## Argument Reference

- `project_id` (String, Required) Stytch project ID (live or test).
- `project_secret` (String, Optional, Sensitive) Optional B2B project secret. If omitted, provider-level `b2b_live_secret`/`b2b_test_secret` are used.
- `organization_id` (String, Optional) Organization ID.
- `slug` (String, Optional) Organization slug. If `organization_id` is omitted, the data source will search by slug.

## Attribute Reference

- `id`, `name`, `slug`, `created_at`, `updated_at`.
- `auth_methods`, `allowed_auth_methods`, `email_allowed_domains`, `email_invites`, `email_jit_provisioning`.
- `sso_jit_provisioning`, `oauth_tenant_jit_provisioning`, `first_party_connected_apps_allowed_type`, `third_party_connected_apps_allowed_type`.
- `rbac_email_implicit_role_assignments` (Set of Object).
- `raw_json` (String) Raw JSON for debugging.


