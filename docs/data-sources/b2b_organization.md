---
page_title: "stytch_b2b_organization Data Source - Stytch"
subcategory: "B2B"
description: |-
  Read a single Stytch B2B Organization by ID or slug with comprehensive policy and configuration details.
---

# stytch_b2b_organization (Data Source)

Reads a single Stytch B2B Organization by `organization_id` or `slug` using the Stytch B2B Auth API. This data source provides comprehensive access to organization configuration including authentication policies, JIT provisioning settings, MFA requirements, and RBAC role assignments.

The data source supports flexible lookup methods and integrates with provider-level secret management for simplified authentication.

## Example Usage

### Basic Organization Lookup by ID

```terraform
data "stytch_b2b_organization" "main_org" {
  project_id      = "project-live-..."
  organization_id = "organization-live-..."
}

output "org_name" {
  value = data.stytch_b2b_organization.main_org.name
}

output "org_auth_methods" {
  value = data.stytch_b2b_organization.main_org.auth_methods
}
```

### Lookup by Organization Slug

```terraform
data "stytch_b2b_organization" "by_slug" {
  project_id = "project-live-..."
  slug       = "my-company"
}

output "org_id" {
  value = data.stytch_b2b_organization.by_slug.id
}

output "allowed_domains" {
  value = data.stytch_b2b_organization.by_slug.email_allowed_domains
}
```

### Using Provider-Level Authentication

```terraform
# Configure provider with B2B secrets
provider "stytch" {
  workspace_key_id     = var.workspace_key_id
  workspace_key_secret = var.workspace_key_secret
  b2b_live_secret      = data.vault_kv_secret_v2.stytch_live.data["secret"]
}

data "stytch_b2b_organization" "with_provider_auth" {
  project_id = "project-live-..."
  slug       = "enterprise"
  # No project_secret needed - uses provider configuration
}
```

### Extracting Policy Information

```terraform
data "stytch_b2b_organization" "policy_check" {
  project_id      = var.project_id
  organization_id = var.organization_id
}

# Use in conditional logic
locals {
  requires_mfa = data.stytch_b2b_organization.policy_check.mfa_policy == "REQUIRED_FOR_ALL"
  allows_email_signup = contains(["ALL_ALLOWED", "RESTRICTED"], data.stytch_b2b_organization.policy_check.email_invites)
  
  # Extract RBAC assignments
  admin_domains = [
    for assignment in data.stytch_b2b_organization.policy_check.rbac_email_implicit_role_assignments :
    assignment.domain if assignment.role_id == "admin"
  ]
}
```

### Debugging with Raw JSON

```terraform
data "stytch_b2b_organization" "debug" {
  project_id      = var.project_id
  organization_id = var.organization_id
}

# Output raw JSON for debugging
output "full_org_config" {
  value = jsondecode(data.stytch_b2b_organization.debug.raw_json)
}
```

## Schema

### Required

- `project_id` (String) Stytch project ID in format `project-live-...` or `project-test-...`

### Optional

- `project_secret` (String, Sensitive) Stytch B2B project secret. If omitted, uses provider-level `b2b_live_secret`/`b2b_test_secret` or environment variables `STYTCH_B2B_LIVE_SECRET`/`STYTCH_B2B_TEST_SECRET`
- `organization_id` (String) Organization ID in format `organization-live-...` or `organization-test-...`. Required if `slug` is not provided
- `slug` (String) Organization slug (URL-friendly identifier). Required if `organization_id` is not provided. The data source will search by slug and return the matching organization

## Attributes Reference

### Basic Information

- `id` (String) Organization ID (same as `organization_id` when provided)
- `name` (String) Organization display name
- `slug` (String) Organization slug
- `created_at` (String) RFC3339 timestamp when the organization was created
- `updated_at` (String) RFC3339 timestamp when the organization was last updated

### Authentication Policies

- `auth_methods` (String) Authentication methods policy: `"ALL_ALLOWED"` or `"RESTRICTED"`
- `allowed_auth_methods` (List of String) Allowed authentication methods when `auth_methods` is `"RESTRICTED"`. Possible values:
  - `"sso"` - SAML/OIDC single sign-on
  - `"magic_link"` - Email magic links
  - `"email_otp"` - Email one-time passwords
  - `"password"` - Username/password authentication
  - `"google_oauth"` - Google OAuth
  - `"microsoft_oauth"` - Microsoft OAuth
  - `"slack_oauth"` - Slack OAuth
  - `"github_oauth"` - GitHub OAuth
  - `"hubspot_oauth"` - HubSpot OAuth

### Email and Domain Controls

- `email_invites` (String) Email invitation policy: `"ALL_ALLOWED"`, `"RESTRICTED"`, or `"NOT_ALLOWED"`
- `email_jit_provisioning` (String) Email just-in-time provisioning policy: `"ALL_ALLOWED"`, `"RESTRICTED"`, or `"NOT_ALLOWED"`
- `email_allowed_domains` (List of String) Allowed email domains for invitations and JIT provisioning when policies are `"RESTRICTED"`

### SSO Configuration

- `sso_jit_provisioning` (String) SSO just-in-time provisioning policy: `"ALL_ALLOWED"`, `"RESTRICTED"`, or `"NOT_ALLOWED"`

### OAuth Tenant Management

- `oauth_tenant_jit_provisioning` (String) OAuth tenant JIT provisioning policy: `"RESTRICTED"` or `"NOT_ALLOWED"`

### Connected Applications

- `first_party_connected_apps_allowed_type` (String) First-party connected apps policy
- `third_party_connected_apps_allowed_type` (String) Third-party connected apps policy

### Role-Based Access Control

- `rbac_email_implicit_role_assignments` (Set of Object) Implicit role assignments based on email domains. Each object contains:
  - `domain` (String) Email domain (e.g., "company.com")
  - `role_id` (String) Role identifier assigned to users from this domain

### Debugging

- `raw_json` (String) Complete organization configuration as JSON string. Useful for debugging, exploring new fields, or extracting data not yet exposed as typed attributes

## Authentication Precedence

The data source resolves authentication credentials in the following order:

1. **Explicit**: `project_secret` attribute in the data source configuration
2. **Provider**: `b2b_live_secret` or `b2b_test_secret` from provider configuration (selected based on `project_id` format)
3. **Environment**: `STYTCH_B2B_LIVE_SECRET` or `STYTCH_B2B_TEST_SECRET` environment variables

## Lookup Behavior

- **By ID**: Direct lookup using the organization ID (most efficient)
- **By Slug**: Searches all organizations in the project to find the matching slug (requires pagination for large projects)
- **Not Found**: Returns an error if the organization doesn't exist or isn't accessible with the provided credentials

## Common Use Cases

### Conditional Configuration

```terraform
data "stytch_b2b_organization" "current" {
  project_id = var.project_id
  slug       = var.org_slug
}

# Configure other resources based on organization policies
resource "some_resource" "conditional" {
  enable_sso = data.stytch_b2b_organization.current.sso_jit_provisioning != "NOT_ALLOWED"
  
  # Only create if MFA is required
  count = data.stytch_b2b_organization.current.mfa_policy == "REQUIRED_FOR_ALL" ? 1 : 0
}
```

### Multi-Organization Setup

```terraform
variable "organization_slugs" {
  description = "List of organization slugs to configure"
  type        = list(string)
  default     = ["dev", "staging", "prod"]
}

data "stytch_b2b_organization" "orgs" {
  for_each = toset(var.organization_slugs)
  
  project_id = var.project_id
  slug       = each.value
}

# Create resources for each organization
resource "aws_s3_bucket" "org_buckets" {
  for_each = data.stytch_b2b_organization.orgs
  
  bucket = "app-${each.value.slug}-data"
  
  tags = {
    Organization = each.value.name
    OrgId       = each.value.id
  }
}
```

### Security Audit

```terraform
data "stytch_b2b_organization" "audit" {
  project_id      = var.project_id
  organization_id = var.organization_id
}

# Output security-relevant information
output "security_audit" {
  value = {
    organization        = data.stytch_b2b_organization.audit.name
    mfa_required       = data.stytch_b2b_organization.audit.mfa_policy == "REQUIRED_FOR_ALL"
    auth_methods       = data.stytch_b2b_organization.audit.auth_methods
    allowed_methods    = data.stytch_b2b_organization.audit.allowed_auth_methods
    email_domains      = data.stytch_b2b_organization.audit.email_allowed_domains
    rbac_assignments   = data.stytch_b2b_organization.audit.rbac_email_implicit_role_assignments
  }
  
  sensitive = false  # Mark as sensitive if needed
}
```

## Error Handling

Common error scenarios and their meanings:

- **"missing project secret"**: No authentication method provided - set `project_secret`, provider configuration, or environment variables
- **"organization not found"**: The specified ID or slug doesn't exist in the project
- **"failed to create b2b client"**: Invalid project ID or secret format
- **"search failed"**: API error during slug-based lookup (network, permissions, etc.)

## Performance Considerations

- **ID Lookup**: Direct API call, fastest method
- **Slug Lookup**: Requires searching through all organizations, may be slower for projects with many organizations
- **Caching**: Terraform will cache the result during a single plan/apply cycle
- **Rate Limits**: Subject to Stytch API rate limiting - use `project_secret` or provider configuration to avoid repeated authentication