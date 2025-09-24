package resources

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stytchauth/stytch-go/v16/stytch/b2b/organizations"
)

func TestMapExtendedOrgFields_NormalizesAllowlists(t *testing.T) {
	t.Parallel()

	// Build an organizations.Organization via JSON to ensure parity with SDK tags
	raw := []byte(`{
        "organization_id": "organization-live-123",
        "organization_name": "Example",
        "organization_slug": "example",
        "allowed_auth_methods": ["google_oauth"],
        "auth_methods": "ALL_ALLOWED",
        "email_allowed_domains": ["mento.co"],
        "email_invites": "ALL_ALLOWED",
        "email_jit_provisioning": "NOT_ALLOWED",
        "sso_jit_provisioning": "NOT_ALLOWED",
        "oauth_tenant_jit_provisioning": "NOT_ALLOWED",
        "allowed_oauth_tenants": {"slack": ["T123"]},
        "first_party_connected_apps_allowed_type": "ALL_ALLOWED",
        "third_party_connected_apps_allowed_type": "ALL_ALLOWED",
        "rbac_email_implicit_role_assignments": [{"domain":"mento.co","role_id":"staging_user"}],
        "mfa_policy": "OPTIONAL",
        "mfa_methods": "ALL_ALLOWED",
        "allowed_mfa_methods": ["totp"],
        "sso_jit_provisioning_allowed_connections": ["conn-1"]
    }`)

	var org organizations.Organization
	if err := json.Unmarshal(raw, &org); err != nil {
		t.Fatalf("unmarshal organization: %v", err)
	}

	var model b2bOrganizationModel
	mapExtendedOrgFields(context.Background(), &org, &model)

	if got := model.AuthMethods.ValueString(); got != "ALL_ALLOWED" {
		t.Fatalf("auth_methods got %q want ALL_ALLOWED", got)
	}
	// Normalized to empty when not RESTRICTED
	if !model.AllowedAuthMethods.Equal(types.ListNull(types.StringType)) && model.AllowedAuthMethods.Elements() != nil {
		// Convert to slice and assert zero length
		var vals []string
		_ = model.AllowedAuthMethods.ElementsAs(context.Background(), &vals, false)
		if len(vals) != 0 {
			t.Fatalf("allowed_auth_methods not normalized, got %v", vals)
		}
	}

	if got := model.OAuthTenantJITProvisioning.ValueString(); got != "NOT_ALLOWED" {
		t.Fatalf("oauth_tenant_jit_provisioning got %q want NOT_ALLOWED", got)
	}
	// Normalized to empty map when not RESTRICTED
	var tenants map[string][]string
	_ = model.AllowedOAuthTenants.ElementsAs(context.Background(), &tenants, false)
	if len(tenants) != 0 {
		t.Fatalf("allowed_oauth_tenants not normalized, got %v", tenants)
	}

	// Still maps other fields
	var domains []string
	_ = model.EmailAllowedDomains.ElementsAs(context.Background(), &domains, false)
	if len(domains) != 1 || domains[0] != "mento.co" {
		t.Fatalf("email_allowed_domains got %v", domains)
	}
}
