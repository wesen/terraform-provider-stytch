---
page_title: "stytch_b2b_organization Resource - Stytch"
subcategory: "B2B"
description: |-
  Manage Stytch B2B Organizations with comprehensive policy controls via the B2B Auth API.
---

# stytch_b2b_organization (Resource)

Manages a Stytch B2B Organization using the Stytch B2B Auth API. Organizations are tenant containers within B2B projects that group users and define authentication policies.

This resource provides full lifecycle management (create, read, update, delete) with built-in safety mechanisms to prevent accidental deletion. Policy changes are validated at plan-time to ensure required companion attributes are provided.

## Authentication Methods

The resource supports multiple authentication options with precedence:
1. **Resource-level**: `project_secret` attribute (explicit override)
2. **Provider-level**: `b2b_live_secret`/`b2b_test_secret` provider configuration
3. **Environment**: `STYTCH_B2B_LIVE_SECRET`/`STYTCH_B2B_TEST_SECRET` variables

## Example Usage

### Basic Organization

```terraform
resource "stytch_b2b_organization" "example" {
  project_id = "project-live-..."
  name       = "Example Corp"
  slug       = "example-corp"
  
  # Safety mechanism
  allow_destroy = false
}
```

### Organization with Authentication Policies

```terraform
resource "stytch_b2b_organization" "restricted_auth" {
  project_id = "project-live-..."
  name       = "Secure Corp"
  slug       = "secure-corp"

  # Restrict authentication methods
  auth_methods = "RESTRICTED"
  allowed_auth_methods = [
    "google_oauth",
    "microsoft_oauth",
    "sso"
  ]

  # Email invitation and JIT provisioning
  email_invites          = "RESTRICTED"
  email_jit_provisioning = "RESTRICTED"
  email_allowed_domains  = ["company.com", "subsidiary.com"]

  # SSO JIT provisioning
  sso_jit_provisioning = "RESTRICTED"
  sso_jit_provisioning_allowed_connections = [
    "saml-connection-123",
    "oidc-connection-456"
  ]

  allow_destroy = true
}
```

### Organization with MFA and OAuth Tenant Controls

```terraform
resource "stytch_b2b_organization" "enterprise" {
  project_id = "project-live-..."
  name       = "Enterprise Corp"
  slug       = "enterprise-corp"

  # Multi-factor authentication
  mfa_policy  = "REQUIRED_FOR_ALL"
  mfa_methods = "RESTRICTED"
  allowed_mfa_methods = ["totp", "sms_otp"]

  # OAuth tenant restrictions
  oauth_tenant_jit_provisioning = "RESTRICTED"
  allowed_oauth_tenants = {
    slack   = ["T1234567890", "T0987654321"]
    github  = ["enterprise-corp"]
    hubspot = ["12345678"]
  }

  allow_destroy = true
}
```

### Provider-Level Secret Configuration

```terraform
# Recommended: Use provider-level secrets
provider "stytch" {
  workspace_key_id     = var.workspace_key_id
  workspace_key_secret = var.workspace_key_secret
  b2b_live_secret      = data.vault_kv_secret_v2.stytch_live.data["secret"]
  b2b_test_secret      = data.vault_kv_secret_v2.stytch_test.data["secret"]
}

resource "stytch_b2b_organization" "with_provider_auth" {
  project_id = "project-live-..."
  name       = "Provider Auth Org"
  slug       = "provider-auth"
  # No project_secret needed - uses provider configuration
}
```

## Schema

### Required

- `project_id` (String) Stytch project ID (live or test format: `project-live-...` or `project-test-...`)
- `name` (String) Organization name (human-readable)

### Optional

- `project_secret` (String, Sensitive) Stytch B2B project secret. If omitted, uses provider-level secrets or environment variables based on `project_id` format
- `slug` (String) Organization slug (URL-friendly identifier). If omitted, Stytch may auto-generate one
- `allow_destroy` (Boolean) Safety flag - must be `true` to permit Terraform destroy operations. Defaults to `false`

#### Authentication Method Controls

- `auth_methods` (String) Authentication methods policy. Values: `"ALL_ALLOWED"`, `"RESTRICTED"`. Defaults to `"ALL_ALLOWED"`
- `allowed_auth_methods` (List of String) Allowed authentication methods when `auth_methods = "RESTRICTED"`. Valid values: `"sso"`, `"magic_link"`, `"email_otp"`, `"password"`, `"google_oauth"`, `"microsoft_oauth"`, `"slack_oauth"`, `"github_oauth"`, `"hubspot_oauth"`

#### Email Invitation and JIT Provisioning

- `email_invites` (String) Email invitation policy. Values: `"ALL_ALLOWED"`, `"RESTRICTED"`, `"NOT_ALLOWED"`
- `email_jit_provisioning` (String) Email JIT provisioning policy. Values: `"ALL_ALLOWED"`, `"RESTRICTED"`, `"NOT_ALLOWED"`
- `email_allowed_domains` (List of String) Allowed email domains when email invites or JIT provisioning are `"RESTRICTED"`. Required if either email policy is `"RESTRICTED"`

#### SSO and JIT Provisioning

- `sso_jit_provisioning` (String) SSO JIT provisioning policy. Values: `"ALL_ALLOWED"`, `"RESTRICTED"`, `"NOT_ALLOWED"`
- `sso_jit_provisioning_allowed_connections` (List of String) Allowed SSO connection IDs when `sso_jit_provisioning = "RESTRICTED"`. Required when policy is `"RESTRICTED"`

#### OAuth Tenant Controls

- `oauth_tenant_jit_provisioning` (String) OAuth tenant JIT provisioning policy. Values: `"RESTRICTED"`, `"NOT_ALLOWED"` (no `"ALL_ALLOWED"` option)
- `allowed_oauth_tenants` (Map of List(String)) OAuth tenant allowlist when `oauth_tenant_jit_provisioning = "RESTRICTED"`. Keys must be one of: `"slack"`, `"hubspot"`, `"github"`. Values are lists of tenant identifiers. Required when policy is `"RESTRICTED"`

#### Multi-Factor Authentication

- `mfa_policy` (String) MFA requirement policy. Values: `"REQUIRED_FOR_ALL"`, `"OPTIONAL"`
- `mfa_methods` (String) MFA methods policy. Values: `"ALL_ALLOWED"`, `"RESTRICTED"`
- `allowed_mfa_methods` (List of String) Allowed MFA methods when `mfa_methods = "RESTRICTED"`. Valid values: `"sms_otp"`, `"totp"`. Required when policy is `"RESTRICTED"`

### Read-Only

- `id` (String) Organization ID (format: `organization-live-...` or `organization-test-...`)
- `created_at` (String) RFC3339 timestamp when the organization was created
- `updated_at` (String) RFC3339 timestamp when the organization was last updated by Stytch
- `last_updated` (String) RFC850 timestamp when the organization was last updated by Terraform
- `rbac_email_implicit_role_assignments` (Set of Object) Implicit role assignments by email domain
  - `domain` (String) Email domain
  - `role_id` (String) Role identifier
- `first_party_connected_apps_allowed_type` (String) First-party connected apps policy
- `third_party_connected_apps_allowed_type` (String) Third-party connected apps policy

## Validation Rules

The resource enforces validation at plan-time to prevent configuration errors:

1. **Email Restrictions**: When `email_invites` or `email_jit_provisioning` is `"RESTRICTED"`, `email_allowed_domains` must be non-empty
2. **MFA Restrictions**: When `mfa_methods` is `"RESTRICTED"`, `allowed_mfa_methods` must be non-empty
3. **SSO Restrictions**: When `sso_jit_provisioning` is `"RESTRICTED"`, `sso_jit_provisioning_allowed_connections` must be non-empty
4. **OAuth Restrictions**: When `oauth_tenant_jit_provisioning` is `"RESTRICTED"`, `allowed_oauth_tenants` must be non-empty with supported keys
5. **OAuth Tenant Keys**: Only `"slack"`, `"hubspot"`, and `"github"` are supported as keys in `allowed_oauth_tenants`

## Import

Organizations can be imported using either the organization ID or slug:

```shell
# Import by organization ID
terraform import stytch_b2b_organization.example "project-live-...,organization-live-..."

# Import by organization slug (resolved to ID automatically)
terraform import stytch_b2b_organization.example "project-live-...,example-slug"
```

## Security Considerations

- **Secrets**: Use provider-level secret configuration or environment variables instead of hardcoding `project_secret` in configurations
- **Destroy Protection**: Always set `allow_destroy = false` for production organizations to prevent accidental deletion
- **Policy Validation**: The resource validates policy combinations at plan-time to prevent invalid API calls
- **State Management**: Sensitive fields like `project_secret` are marked as sensitive and won't appear in logs

## Common Patterns

### Development/Testing Organization

```terraform
resource "stytch_b2b_organization" "dev" {
  project_id = "project-test-..."
  name       = "Development Org"
  slug       = "dev-org"
  
  # Permissive for development
  auth_methods           = "ALL_ALLOWED"
  email_invites          = "ALL_ALLOWED"
  email_jit_provisioning = "ALL_ALLOWED"
  mfa_policy            = "OPTIONAL"
  
  allow_destroy = true  # OK for dev/test
}
```

### Production Enterprise Organization

```terraform
resource "stytch_b2b_organization" "prod" {
  project_id = "project-live-..."
  name       = "Production Corp"
  slug       = "prod-corp"
  
  # Restrictive for security
  auth_methods = "RESTRICTED"
  allowed_auth_methods = ["sso", "google_oauth"]
  
  email_invites         = "RESTRICTED"
  email_allowed_domains = ["company.com"]
  
  mfa_policy = "REQUIRED_FOR_ALL"
  
  # Never allow destruction in production
  allow_destroy = false
}
```