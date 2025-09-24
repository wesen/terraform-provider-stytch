---
page_title: "stytch_b2b_organization Resource - Stytch"
subcategory: "B2B"
description: |-
  Manage Stytch B2B Organizations via the B2B Auth API.
---

# stytch_b2b_organization (Resource)

Manages a Stytch B2B Organization using the Stytch B2B Auth API.

Delete is guarded by `allow_destroy = true` to prevent accidental removal.

## Example Usage

Provider with Vault-provided live app secret (recommended):

```hcl
provider "stytch" {
  workspace_key_id     = var.workspace_key_id
  workspace_key_secret = var.workspace_key_secret
  b2b_live_secret      = data.vault_kv_secret_v2.stytch_app_live.data["secret"]
}

resource "stytch_b2b_organization" "example" {
  project_id     = var.project_id
  # Optional if provider b2b_live_secret is set; otherwise set project_secret or env
  # project_secret = data.vault_kv_secret_v2.stytch_app_live.data["secret"]

  name = "Example Org"
  slug = "example-org"

  # Policy examples
  auth_methods           = "ALL_ALLOWED"
  email_invites          = "ALL_ALLOWED"
  email_jit_provisioning = "NOT_ALLOWED"

  # Destroy safety
  allow_destroy = false
}
```

Import by organization ID or slug:

```bash
terraform import stytch_b2b_organization.example "project-live-...,organization-live-..."
terraform import stytch_b2b_organization.example "project-live-...,example-org"
```

## Argument Reference

- `project_id` (String, Required) Stytch project ID (live or test).
- `project_secret` (String, Optional, Sensitive) Stytch B2B project secret. If omitted, the provider-level `b2b_live_secret`/`b2b_test_secret` or environment variables `STYTCH_B2B_LIVE_SECRET`/`STYTCH_B2B_TEST_SECRET` are used based on `project_id`.
- `name` (String, Required) Organization name.
- `slug` (String, Optional) Organization slug.
- `allow_destroy` (Bool, Optional) Must be true to permit delete.

Policy arguments (Optional; some require companions when RESTRICTED):
- `auth_methods` (String) One of `ALL_ALLOWED`, `RESTRICTED`.
- `allowed_auth_methods` (List of String) When `auth_methods = RESTRICTED`.
- `email_invites` (String) One of `ALL_ALLOWED`, `RESTRICTED`, `NOT_ALLOWED`.
- `email_jit_provisioning` (String) One of `ALL_ALLOWED`, `RESTRICTED`, `NOT_ALLOWED`.
- `email_allowed_domains` (List of String) Required when email invites/JIT are `RESTRICTED`.
- `sso_jit_provisioning` (String) One of `ALL_ALLOWED`, `RESTRICTED`, `NOT_ALLOWED`.
- `sso_jit_provisioning_allowed_connections` (List of String) Required when SSO JIT is `RESTRICTED`.
- `oauth_tenant_jit_provisioning` (String) One of `RESTRICTED`, `NOT_ALLOWED`.
- `allowed_oauth_tenants` (Map of List(String)) Required when OAuth tenant JIT is `RESTRICTED`. Keys: `slack`, `hubspot`, `github`.
- `mfa_policy` (String) One of `REQUIRED_FOR_ALL`, `OPTIONAL`.
- `mfa_methods` (String) One of `ALL_ALLOWED`, `RESTRICTED`.
- `allowed_mfa_methods` (List of String) Required when `mfa_methods = RESTRICTED`.

## Attribute Reference

Computed:
- `id` Organization ID.
- `created_at`, `updated_at`, `last_updated` Timestamps.
- `rbac_email_implicit_role_assignments` (Set of Object) Implicit role assignments by email domain.
- `first_party_connected_apps_allowed_type`, `third_party_connected_apps_allowed_type` (String)

## Import

Import using `project_id,organization_id` or `project_id,slug`:

```bash
terraform import stytch_b2b_organization.example "project-live-...,organization-live-..."
terraform import stytch_b2b_organization.example "project-live-...,example-slug"
```


