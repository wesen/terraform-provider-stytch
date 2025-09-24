# Basic organization lookup by ID
data "stytch_b2b_organization" "by_id" {
  project_id      = var.project_id
  organization_id = var.organization_id
}

# Organization lookup by slug
data "stytch_b2b_organization" "by_slug" {
  project_id = var.project_id
  slug       = "my-company"
}

# Using provider-level authentication
data "stytch_b2b_organization" "with_provider_auth" {
  project_id = var.project_id
  slug       = var.organization_slug
  # No project_secret needed if provider b2b_live_secret is configured
}

# Extract policy information for conditional logic
locals {
  org_policies = data.stytch_b2b_organization.by_id
  
  # Policy analysis
  is_mfa_required = local.org_policies.mfa_policy == "REQUIRED_FOR_ALL"
  allows_email_signup = contains(["ALL_ALLOWED", "RESTRICTED"], local.org_policies.email_invites)
  
  # RBAC analysis
  admin_domains = [
    for assignment in local.org_policies.rbac_email_implicit_role_assignments :
    assignment.domain if assignment.role_id == "admin"
  ]
  
  # Authentication method analysis
  oauth_methods = [
    for method in local.org_policies.allowed_auth_methods :
    method if can(regex("_oauth$", method))
  ]
}

# Outputs for use in other configurations
output "organization_info" {
  description = "Organization details and policy summary"
  value = {
    id                = data.stytch_b2b_organization.by_id.id
    name              = data.stytch_b2b_organization.by_id.name
    slug              = data.stytch_b2b_organization.by_id.slug
    created_at        = data.stytch_b2b_organization.by_id.created_at
    
    # Policy summary
    auth_restricted   = data.stytch_b2b_organization.by_id.auth_methods == "RESTRICTED"
    mfa_required     = local.is_mfa_required
    email_domains    = data.stytch_b2b_organization.by_id.email_allowed_domains
    
    # OAuth analysis
    oauth_methods    = local.oauth_methods
    oauth_restricted = data.stytch_b2b_organization.by_id.oauth_tenant_jit_provisioning == "RESTRICTED"
  }
}

# Debug output with full configuration
output "debug_full_config" {
  description = "Complete organization configuration for debugging"
  value = jsondecode(data.stytch_b2b_organization.by_id.raw_json)
  sensitive = true  # May contain sensitive policy details
}