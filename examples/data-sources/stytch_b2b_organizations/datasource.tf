data "stytch_b2b_organizations" "all" {
  project_id = var.project_id
}

output "org_count" {
  value = data.stytch_b2b_organizations.all.total_count
}
