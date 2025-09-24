# Basic organization
resource "stytch_b2b_organization" "basic" {
  project_id = var.project_id
  name       = "Basic Organization"
  slug       = "basic-org"
  
  allow_destroy = false
}

# Organization with restricted authentication
resource "stytch_b2b_organization" "restricted" {
  project_id = var.project_id
  name       = "Restricted Organization"
  slug       = "restricted-org"

  # Only allow SSO and Google OAuth
  auth_methods = "RESTRICTED"
  allowed_auth_methods = [
    "sso",
    "google_oauth"
  ]

  # Require company email domains
  email_invites         = "RESTRICTED"
  email_jit_provisioning = "RESTRICTED"
  email_allowed_domains = ["company.com", "subsidiary.org"]

  allow_destroy = true
}

# Enterprise organization with MFA and OAuth controls
resource "stytch_b2b_organization" "enterprise" {
  project_id = var.project_id
  name       = "Enterprise Organization"
  slug       = "enterprise-org"

  # Require MFA for all users
  mfa_policy  = "REQUIRED_FOR_ALL"
  mfa_methods = "RESTRICTED"
  allowed_mfa_methods = ["totp"]

  # Control OAuth tenant access
  oauth_tenant_jit_provisioning = "RESTRICTED"
  allowed_oauth_tenants = {
    slack  = ["T1234567890"]
    github = ["enterprise-corp"]
  }

  # Disable email JIT but allow invites
  email_invites          = "ALL_ALLOWED"
  email_jit_provisioning = "NOT_ALLOWED"

  allow_destroy = true
}

# Using environment variable authentication
resource "stytch_b2b_organization" "env_auth" {
  project_id = var.project_id
  name       = "Environment Auth Org"
  slug       = "env-auth-org"
  
  # No project_secret - relies on STYTCH_B2B_LIVE_SECRET env var
  # or provider-level b2b_live_secret configuration
  
  allow_destroy = true
}