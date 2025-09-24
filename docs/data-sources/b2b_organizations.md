---
page_title: "stytch_b2b_organizations Data Source - Stytch"
subcategory: "B2B"
description: |-
  List all Stytch B2B Organizations in a project with automatic pagination and summary information.
---

# stytch_b2b_organizations (Data Source)

Lists all B2B Organizations within a Stytch project using the B2B Auth API. This data source automatically handles pagination to retrieve all organizations and provides summary information for inventory, conditional logic, and bulk operations.

The data source is optimized for listing scenarios and provides essential organization metadata without the full policy details available in the single organization data source.

## Example Usage

### Basic Organization Listing

```terraform
data "stytch_b2b_organizations" "all" {
  project_id = "project-live-..."
}

output "organization_count" {
  value = data.stytch_b2b_organizations.all.total_count
}

output "organization_names" {
  value = [for org in data.stytch_b2b_organizations.all.items : org.name]
}
```

### Using Provider-Level Authentication

```terraform
# Configure provider with B2B secrets for simplified access
provider "stytch" {
  workspace_key_id     = var.workspace_key_id
  workspace_key_secret = var.workspace_key_secret
  b2b_live_secret      = data.vault_kv_secret_v2.stytch_live.data["secret"]
}

data "stytch_b2b_organizations" "with_provider_auth" {
  project_id = "project-live-..."
  # No project_secret needed - uses provider configuration
}
```

### Creating Resources for Each Organization

```terraform
data "stytch_b2b_organizations" "orgs" {
  project_id = var.project_id
}

# Create a resource for each organization
resource "aws_s3_bucket" "org_buckets" {
  for_each = { for org in data.stytch_b2b_organizations.orgs.items : org.slug => org }
  
  bucket = "app-${each.value.slug}-data"
  
  tags = {
    OrganizationName = each.value.name
    OrganizationId   = each.value.id
    CreatedAt        = each.value.created_at
  }
}
```

### Filtering and Processing Organizations

```terraform
data "stytch_b2b_organizations" "all_orgs" {
  project_id = var.project_id
}

locals {
  # Filter organizations by age
  recent_orgs = [
    for org in data.stytch_b2b_organizations.all_orgs.items :
    org if timecmp(org.created_at, timeadd(timestamp(), "-30d")) > 0
  ]
  
  # Create lookup maps
  org_by_slug = { for org in data.stytch_b2b_organizations.all_orgs.items : org.slug => org }
  org_by_id   = { for org in data.stytch_b2b_organizations.all_orgs.items : org.id => org }
  
  # Extract slugs for other operations
  org_slugs = [for org in data.stytch_b2b_organizations.all_orgs.items : org.slug]
}

output "organization_summary" {
  value = {
    total_count    = data.stytch_b2b_organizations.all_orgs.total_count
    recent_count   = length(local.recent_orgs)
    slugs         = local.org_slugs
  }
}
```

### Conditional Resource Creation

```terraform
data "stytch_b2b_organizations" "check" {
  project_id = var.project_id
}

# Only create monitoring if there are organizations to monitor
resource "aws_cloudwatch_dashboard" "org_monitoring" {
  count = data.stytch_b2b_organizations.check.total_count > 0 ? 1 : 0
  
  dashboard_name = "stytch-organizations"
  
  dashboard_body = jsonencode({
    widgets = [
      for org in data.stytch_b2b_organizations.check.items : {
        type   = "metric"
        width  = 12
        height = 6
        properties = {
          metrics = [
            ["AWS/Lambda", "Invocations", "FunctionName", "auth-${org.slug}"],
            [".", "Errors", ".", "."],
          ]
          period = 300
          stat   = "Sum"
          region = "us-east-1"
          title  = "Auth Metrics - ${org.name}"
        }
      }
    ]
  })
}
```

### Multi-Project Organization Inventory

```terraform
variable "projects" {
  description = "Map of project environments to IDs"
  type        = map(string)
  default = {
    dev     = "project-test-..."
    staging = "project-test-..."
    prod    = "project-live-..."
  }
}

data "stytch_b2b_organizations" "by_env" {
  for_each = var.projects
  
  project_id = each.value
}

# Combine all organizations across environments
locals {
  all_organizations = merge([
    for env, orgs in data.stytch_b2b_organizations.by_env : {
      for org in orgs.items : "${env}-${org.slug}" => merge(org, {
        environment = env
        project_id  = var.projects[env]
      })
    }
  ]...)
}

output "organization_inventory" {
  value = {
    by_environment = {
      for env, orgs in data.stytch_b2b_organizations.by_env :
      env => {
        count = orgs.total_count
        names = [for org in orgs.items : org.name]
      }
    }
    total_across_all = sum([
      for orgs in data.stytch_b2b_organizations.by_env : orgs.total_count
    ])
  }
}
```

## Schema

### Required

- `project_id` (String) Stytch project ID in format `project-live-...` or `project-test-...`

### Optional

- `project_secret` (String, Sensitive) Stytch B2B project secret. If omitted, uses provider-level `b2b_live_secret`/`b2b_test_secret` or environment variables `STYTCH_B2B_LIVE_SECRET`/`STYTCH_B2B_TEST_SECRET` based on project ID format

## Attributes Reference

- `total_count` (Number) Total number of organizations in the project
- `items` (List of Object) List of organization summaries. Each organization object contains:
  - `id` (String) Organization ID (format: `organization-live-...` or `organization-test-...`)
  - `name` (String) Organization display name
  - `slug` (String) Organization slug (URL-friendly identifier)
  - `created_at` (String) RFC3339 timestamp when the organization was created
  - `updated_at` (String) RFC3339 timestamp when the organization was last updated

## Authentication

The data source uses the same authentication precedence as other B2B resources:

1. **Explicit**: `project_secret` attribute in the data source configuration
2. **Provider**: `b2b_live_secret` or `b2b_test_secret` from provider configuration (auto-selected based on `project_id` format)
3. **Environment**: `STYTCH_B2B_LIVE_SECRET` or `STYTCH_B2B_TEST_SECRET` environment variables

## Pagination Handling

The data source automatically handles pagination internally:

- **Cursor-based**: Uses Stytch's cursor-based pagination system
- **Batch Size**: Retrieves organizations in batches of 1000 (API maximum)
- **Automatic**: Continues until all organizations are retrieved
- **Memory**: All results are loaded into memory and returned as a single list

For very large projects (hundreds of organizations), consider the performance implications and whether you need all organizations at once.

## Performance Considerations

### API Calls

- **Single Project**: One API call per 1000 organizations (rounded up)
- **Multiple Projects**: Separate API calls for each project
- **Rate Limiting**: Subject to Stytch API rate limits

### Memory Usage

- **Small Projects** (< 100 orgs): Negligible memory impact
- **Large Projects** (> 1000 orgs): Each organization uses ~200-500 bytes in Terraform state
- **Very Large Projects** (> 10,000 orgs): Consider using filtering or pagination in your application logic

### Terraform State

- **State Size**: Organization list is stored in Terraform state
- **Refresh**: Re-fetched on every `terraform plan` or `terraform refresh`
- **Dependencies**: Changes trigger dependent resource updates

## Error Handling

Common error scenarios:

- **"missing project secret"**: Authentication not configured - provide credentials via resource, provider, or environment
- **"failed to create b2b client"**: Invalid project ID or secret format
- **API errors**: Network issues, rate limiting, or service outages during pagination

## Use Case Patterns

### Organization Discovery

```terraform
# Discover what organizations exist before creating specific configurations
data "stytch_b2b_organizations" "discovery" {
  project_id = var.project_id
}

# Use results to inform other data sources
data "stytch_b2b_organization" "details" {
  for_each = { for org in data.stytch_b2b_organizations.discovery.items : org.slug => org }
  
  project_id      = var.project_id
  organization_id = each.value.id
}
```

### Bulk Operations

```terraform
data "stytch_b2b_organizations" "bulk" {
  project_id = var.project_id
}

# Create monitoring alerts for each organization
resource "aws_cloudwatch_metric_alarm" "org_errors" {
  for_each = { for org in data.stytch_b2b_organizations.bulk.items : org.slug => org }
  
  alarm_name          = "stytch-${each.value.slug}-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "Errors"
  namespace           = "Stytch/B2B"
  period              = "120"
  statistic           = "Sum"
  threshold           = "10"
  alarm_description   = "High error rate for ${each.value.name}"
  
  dimensions = {
    OrganizationId = each.value.id
  }
}
```

### Environment Synchronization

```terraform
# Ensure production has all organizations that exist in staging
data "stytch_b2b_organizations" "staging" {
  project_id = var.staging_project_id
}

data "stytch_b2b_organizations" "prod" {
  project_id = var.prod_project_id
}

locals {
  staging_slugs = toset([for org in data.stytch_b2b_organizations.staging.items : org.slug])
  prod_slugs    = toset([for org in data.stytch_b2b_organizations.prod.items : org.slug])
  missing_in_prod = setsubtract(local.staging_slugs, local.prod_slugs)
}

# Create missing organizations in production
resource "stytch_b2b_organization" "sync_to_prod" {
  for_each = local.missing_in_prod
  
  project_id = var.prod_project_id
  name       = "${each.key} (synced from staging)"
  slug       = each.key
  
  # Copy basic policies from staging (would need additional data source lookups for full sync)
  allow_destroy = false
}
```

## Comparison with Single Organization Data Source

| Feature | `stytch_b2b_organizations` | `stytch_b2b_organization` |
|---------|---------------------------|---------------------------|
| **Purpose** | List all organizations | Get single organization details |
| **Fields** | Basic info only | Full policy configuration |
| **Performance** | Paginated bulk retrieval | Direct single lookup |
| **Use Cases** | Inventory, bulk ops, discovery | Configuration, conditionals |
| **Memory** | Higher for large projects | Minimal |
| **API Calls** | One per 1000 organizations | One per organization |

Use `stytch_b2b_organizations` for discovery and bulk operations, then `stytch_b2b_organization` for detailed configuration of specific organizations.