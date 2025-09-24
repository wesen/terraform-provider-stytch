package resources_test

import (
	"regexp"
	"testing"

	"github.com/stytchauth/terraform-provider-stytch/internal/provider/testutil"
)

func TestAccB2BOrganizationResource_Validators(t *testing.T) {
	t.Parallel()

	// Each case expects a plan-time validation error from ValidateConfig
	for _, ec := range []testutil.ErrorCase{
		{
			Name: "email restricted requires domains",
			Config: testutil.B2BProjectConfig + `
resource "stytch_b2b_organization" "org" {
  project_id = stytch_project.project.test_project_id
  name       = "bad"
  slug       = "bad-email-restricted"
  email_invites = "RESTRICTED"
  email_allowed_domains = []
}
`,
			Error: regexp.MustCompile("email_allowed_domains required"),
		},
		{
			Name: "mfa restricted requires allowed list",
			Config: testutil.B2BProjectConfig + `
resource "stytch_b2b_organization" "org" {
  project_id = stytch_project.project.test_project_id
  name       = "bad"
  slug       = "bad-mfa-restricted"
  mfa_methods = "RESTRICTED"
}
`,
			Error: regexp.MustCompile("allowed_mfa_methods required"),
		},
		{
			Name: "sso restricted requires connections",
			Config: testutil.B2BProjectConfig + `
resource "stytch_b2b_organization" "org" {
  project_id = stytch_project.project.test_project_id
  name       = "bad"
  slug       = "bad-sso-restricted"
  sso_jit_provisioning = "RESTRICTED"
}
`,
			Error: regexp.MustCompile("sso_jit_provisioning_allowed_connections required"),
		},
		{
			Name: "oauth restricted requires tenants",
			Config: testutil.B2BProjectConfig + `
resource "stytch_b2b_organization" "org" {
  project_id = stytch_project.project.test_project_id
  name       = "bad"
  slug       = "bad-oauth-restricted"
  oauth_tenant_jit_provisioning = "RESTRICTED"
}
`,
			Error: regexp.MustCompile("allowed_oauth_tenants required"),
		},
	} {
		ec.AssertErrorWith(t, ec.Error)
	}
}
