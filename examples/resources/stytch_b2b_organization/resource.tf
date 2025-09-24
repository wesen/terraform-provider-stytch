# Example stytch_b2b_organization resource
resource "stytch_b2b_organization" "example" {
  project_id = var.project_id
  name       = "Example Org"
  slug       = "example-org"

  # Optional: policy fields
  auth_methods           = "ALL_ALLOWED"
  email_invites          = "ALL_ALLOWED"
  email_jit_provisioning = "NOT_ALLOWED"

  # Safety
  allow_destroy = false
}
