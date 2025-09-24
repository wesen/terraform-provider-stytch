### Design for a separate Terraform provider dedicated to Stytch B2B Auth Organizations

This document proposes a clean, decoupled Terraform provider focused exclusively on Stytch B2B Auth APIs for organizations. It consolidates what we learned while extending the mixed provider, and defines an architecture that avoids cross-talk with the Management API provider.

Reference context: See `go-go-mento/ttmp/2025-09-23/05-implementing-the-stytch-provider-organization-datasource-and-progress-log-and-knowledge-gained.md` for the work log, pitfalls, and learnings that motivated this separation.

### Why a separate provider
- Clear API boundaries: B2B Auth (tenant/org-level auth) is separate from the Management API (workspace/project admin). Mixing causes plan/apply inconsistencies and confusing secret handling.
- Secrets and precedence: B2B Auth uses project secrets (live/test) rather than workspace management keys. Combining both in one provider confuses precedence and state handling.
- Schema and drift: B2B org policy fields (auth methods, email JIT, tenants, MFA) have different lifecycles and defaults than Management resources. Keeping them isolated yields predictable diffs.
- Safety and state hygiene: We must treat project secrets as write-only; a focused provider makes that rule easy to enforce everywhere.

### Provider identity and scope
- Address (proposed): `registry.terraform.io/mento/stytch-b2b` (final namespace TBD)
- Goals (v0.1):
  - Resource: `stytch_b2b_organization` (Create/Read/Update/Delete/Import)
  - Data source: `stytch_b2b_organization` (by id or slug)
  - Data source: `stytch_b2b_organizations` (list with pagination)
- Non-goals (initially): members, connections, apps, SDK config (can be added later in this dedicated provider as separate resources/DS)

### Secret handling (core principle)
- Inputs supported:
  - Provider-level: `b2b_live_secret`, `b2b_test_secret` (Sensitive)
  - Resource-level: `project_secret` (Sensitive, WriteOnly)
  - Environment fallback: `STYTCH_B2B_LIVE_SECRET`, `STYTCH_B2B_TEST_SECRET`
- Precedence: resource attribute > provider attribute > environment variable
- Detection: choose secret based on `project_id` prefix (`project-live-` vs `project-test-`).
- State hygiene:
  - Mark `project_secret` as WriteOnly in schema
  - Explicitly clear `ProjectSecret` before saving state in all CRUD paths
  - Never log secrets; emit masked debug (e.g., `secret_present=true`)

### Target APIs and SDK
- Stytch Go SDK: `github.com/stytchauth/stytch-go/v16` (B2B Auth namespaces)
- Endpoints used:
  - Organizations: Create, Get, Search, Update, Delete
- Error handling:
  - Summarize Stytch error type and message in diagnostics
  - Normalize not-found into state removal in Read

### Resource: stytch_b2b_organization
- Schema
  - Required: `project_id`, `name`
  - Optional: `slug`, `project_secret` (WriteOnly)
  - Optional policy enums with validation + normalization:
    - `auth_methods` = ALL_ALLOWED | RESTRICTED
    - `allowed_auth_methods` (List[String]); only relevant when RESTRICTED
    - `email_invites` = ALL_ALLOWED | RESTRICTED | NOT_ALLOWED
    - `email_jit_provisioning` = ALL_ALLOWED | RESTRICTED | NOT_ALLOWED
    - `email_allowed_domains` (List[String]); required when any email policy is RESTRICTED
    - `sso_jit_provisioning` = ALL_ALLOWED | RESTRICTED | NOT_ALLOWED
    - `sso_jit_provisioning_allowed_connections` (List[String]); required when RESTRICTED
    - `oauth_tenant_jit_provisioning` = RESTRICTED | NOT_ALLOWED
    - `allowed_oauth_tenants` (Map[String][]String) keys: slack|hubspot|github; required when RESTRICTED
    - `mfa_policy` = REQUIRED_FOR_ALL | OPTIONAL
    - `mfa_methods` = ALL_ALLOWED | RESTRICTED
    - `allowed_mfa_methods` (List[String] sms_otp|totp); required when RESTRICTED
  - Read-only (Computed): `id`, timestamps, connected-app policies, `rbac_email_implicit_role_assignments` (Set of {domain, role_id})
  - Safety: `allow_destroy` boolean gate for deletes
- Behavior
  - Create: name+slug; do not attempt to write computed/read-only surfaces. Initialize computed attributes to deterministic empty values.
  - Read: prefer Get by `id`, fallback to Search by `slug`; map policies and normalize (clear irrelevant allow-lists when the governing enum is not RESTRICTED).
  - Update: only send changed, applicable policy fields; normalize lists/maps to avoid null/unknown diffs.
  - Delete: blocked unless `allow_destroy=true`.
  - Import: `project_id,organization_id` or `project_id,slug` (resolve slug to id using provider/env secret).

### Data sources
- stytch_b2b_organization
  - Inputs: `project_id`, `organization_id` or `slug`; optional `project_secret`
  - Outputs: `id`, `name`, `slug`, timestamps, selected policy fields, `rbac_email_implicit_role_assignments`, and an optional `raw_json` for debugging
- stytch_b2b_organizations
  - Inputs: `project_id`; optional `project_secret`
  - Outputs: list of organizations: `id`, `name`, `slug`, timestamps; pagination under the hood

### Diff normalization rules (to avoid churn)
- When `auth_methods != RESTRICTED`, clear `allowed_auth_methods` in state
- When `oauth_tenant_jit_provisioning != RESTRICTED`, clear `allowed_oauth_tenants` in state
- When `sso_jit_provisioning != RESTRICTED`, clear `sso_jit_provisioning_allowed_connections` in state
- Always initialize list/map/set computed fields to known-empty values before overlaying remote data

### Provider configuration
- Example
```hcl
terraform {
  required_providers {
    stytchb2b = {
      source  = "mento/stytch-b2b"
      version = ">= 0.1.0"
    }
  }
}

provider "stytchb2b" {
  b2b_live_secret = var.stytch_b2b_live_secret     # or env STYTCH_B2B_LIVE_SECRET
  b2b_test_secret = var.stytch_b2b_test_secret     # or env STYTCH_B2B_TEST_SECRET
}

resource "stytchb2b_organization" "mento" {
  project_id = var.live_project_id
  name       = "Mento"
  slug       = "mento"

  auth_methods           = "RESTRICTED"
  allowed_auth_methods   = ["google_oauth"]
  email_invites          = "RESTRICTED"
  email_jit_provisioning = "RESTRICTED"
  email_allowed_domains  = ["mento.co"]
  sso_jit_provisioning   = "ALL_ALLOWED"
  mfa_policy             = "OPTIONAL"
  mfa_methods            = "ALL_ALLOWED"
  oauth_tenant_jit_provisioning = "NOT_ALLOWED"
}
```

### Developer ergonomics
- Terraform Plugin Framework; module layout similar to our current provider, but with B2B-only namespaces
- Central helper for secret resolution with precedence and project-id environment detection
- Central mapping utilities for policy normalization and error summarization
- WriteOnly and explicit nullification of `project_secret` before state writes
- `tflog.Debug` at key points: secret presence (masked), mapping coverage, summarized diffs

### Testing strategy
- Unit tests
  - Secret resolution precedence
  - Policy validators (RESTRICTED requires allow-lists)
  - Normalization behavior (lists/maps cleared when not applicable)
- Acceptance tests (TF_ACC)
  - Create org (test project), update policies, import by id and slug, delete with `allow_destroy=true`
  - Env-gated secrets; skip live-destructive tests

### Release and distribution
- Versioning: semantic versions; note breaking changes in policy mapping or schema
- Registry: publish to an org namespace (to be decided) separate from the Management provider
- Docs: full pages for provider, resource, and data sources; examples for env/provided secrets

### Migration path from mixed provider
- Run both providers side-by-side using provider aliases
- State mv for org resources:
```bash
terraform state mv 'stytch_b2b_organization.live' 'stytchb2b_organization.live'
```
- Remove B2B org usage from the mixed provider configuration once migrated

### Roadmap
- v0.1
  - Provider + Org DS/Resource
  - WriteOnly `project_secret`, precedence, normalization
- v0.2
  - Expand resource updates to cover additional safe policy fields
  - Add import by slug quality-of-life improvements
- v0.3+
  - RBAC implicit role assignments: keep read-only until official write APIs; add separate resource once available
  - Additional B2B surfaces (members, SSO/OIDC connections) as separate resources

### Risks and mitigations
- API surface mismatch: keep resource updates conservative; prefer plan-time validation and read-only reflection over speculative writes
- Secret leakage: enforce WriteOnly + clearing; no secret logs; prefer provider/env over per-resource secrets
- Drift due to toggles: normalize non-applicable allow-lists; use deterministic empty values in state

### Conclusion
A dedicated Stytch B2B provider isolates the B2B Auth concerns—secrets, schemas, and behaviors—from Management resources, eliminating sensitive attribute inconsistencies and policy drift. The outlined architecture gives us a secure, predictable, and extensible foundation to manage organizations with Terraform.
