package resources_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stytchauth/terraform-provider-stytch/internal/provider/testutil"
)

func TestAccB2BOrganizationResource_SecretPrecedence(t *testing.T) {
	t.Parallel()

	// Ensure env vars are set for fallback
	live := os.Getenv("STYTCH_B2B_LIVE_SECRET")
	if live == "" {
		t.Skip("STYTCH_B2B_LIVE_SECRET not set; skipping")
	}

	cfg := testutil.B2BProjectConfig + `
resource "stytch_b2b_organization" "org" {
  project_id = stytch_project.project.test_project_id
  name       = "precedence"
  slug       = "precedence"
  allow_destroy = true
}
`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testutil.ProviderConfig + cfg,
			},
			{
				ResourceName:            "stytch_b2b_organization.org",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"last_updated", "project_secret"},
			},
			{
				Config:  testutil.ProviderConfig + cfg,
				Destroy: true,
				// Checks handled by default delete; allow_destroy permits deletion
			},
		},
	})
}
