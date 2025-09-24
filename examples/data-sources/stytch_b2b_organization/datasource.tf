data "stytch_b2b_organization" "org" {
  project_id      = var.project_id
  organization_id = var.organization_id
}

output "org_name" {
  value = data.stytch_b2b_organization.org.name
}
