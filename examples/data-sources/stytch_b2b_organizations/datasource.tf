# List all organizations in a project
data "stytch_b2b_organizations" "all" {
  project_id = var.project_id
}

# Basic outputs
output "organization_count" {
  value = data.stytch_b2b_organizations.all.total_count
}

output "organization_names" {
  value = [for org in data.stytch_b2b_organizations.all.items : org.name]
}

# Create resources for each organization
resource "aws_s3_bucket" "org_buckets" {
  for_each = { for org in data.stytch_b2b_organizations.all.items : org.slug => org }
  
  bucket = "app-${each.value.slug}-data"
  
  tags = {
    OrganizationName = each.value.name
    OrganizationId   = each.value.id
    Environment     = contains(var.project_id, "project-live-") ? "production" : "test"
  }
}

# Filter organizations by creation date
locals {
  recent_organizations = [
    for org in data.stytch_b2b_organizations.all.items :
    org if timecmp(org.created_at, timeadd(timestamp(), "-7d")) > 0
  ]
  
  # Create lookup maps for other operations
  org_by_slug = { for org in data.stytch_b2b_organizations.all.items : org.slug => org }
  org_by_id   = { for org in data.stytch_b2b_organizations.all.items : org.id => org }
}

# Multi-environment inventory
variable "environments" {
  description = "Map of environment names to project IDs"
  type        = map(string)
  default = {
    staging = "project-test-..."
    prod    = "project-live-..."
  }
}

data "stytch_b2b_organizations" "by_env" {
  for_each = var.environments
  
  project_id = each.value
}

output "environment_summary" {
  value = {
    for env, orgs in data.stytch_b2b_organizations.by_env :
    env => {
      count         = orgs.total_count
      organizations = [for org in orgs.items : {
        name = org.name
        slug = org.slug
        age  = timeadd(org.created_at, "0s")  # Convert to timestamp
      }]
    }
  }
}