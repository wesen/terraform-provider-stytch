package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	listvalidator "github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stytchauth/stytch-go/v16/stytch/b2b/b2bstytchapi"
	"github.com/stytchauth/stytch-go/v16/stytch/b2b/organizations"
)

var _ resource.Resource = &b2bOrganizationResource{}
var _ resource.ResourceWithImportState = &b2bOrganizationResource{}

func NewB2BOrganizationResource() resource.Resource { return &b2bOrganizationResource{} }

type b2bOrganizationResource struct{}

type b2bOrganizationModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	ProjectSecret types.String `tfsdk:"project_secret"`
	Name          types.String `tfsdk:"name"`
	Slug          types.String `tfsdk:"slug"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
	LastUpdated   types.String `tfsdk:"last_updated"`
	AllowDestroy  types.Bool   `tfsdk:"allow_destroy"`

	// Expanded policy fields (currently read-only reflection; consider updatable in future)
	AllowedAuthMethods                   types.List   `tfsdk:"allowed_auth_methods"`
	AuthMethods                          types.String `tfsdk:"auth_methods"`
	EmailAllowedDomains                  types.List   `tfsdk:"email_allowed_domains"`
	EmailInvites                         types.String `tfsdk:"email_invites"`
	EmailJITProvisioning                 types.String `tfsdk:"email_jit_provisioning"`
	SSOJITProvisioning                   types.String `tfsdk:"sso_jit_provisioning"`
	OAuthTenantJITProvisioning           types.String `tfsdk:"oauth_tenant_jit_provisioning"`
	AllowedOAuthTenants                  types.Map    `tfsdk:"allowed_oauth_tenants"`
	FirstPartyConnectedAppsAllowedType   types.String `tfsdk:"first_party_connected_apps_allowed_type"`
	ThirdPartyConnectedAppsAllowedType   types.String `tfsdk:"third_party_connected_apps_allowed_type"`
	RBACEmailImplicitRoleAssignments     types.Set    `tfsdk:"rbac_email_implicit_role_assignments"`
	MFAPolicy                            types.String `tfsdk:"mfa_policy"`
	MFAMethods                           types.String `tfsdk:"mfa_methods"`
	AllowedMFAMethods                    types.List   `tfsdk:"allowed_mfa_methods"`
	SSOJITProvisioningAllowedConnections types.List   `tfsdk:"sso_jit_provisioning_allowed_connections"`
}

func (r *b2bOrganizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_b2b_organization"
}

func (r *b2bOrganizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Stytch B2B organization via the B2B Auth API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Organization ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Stytch project ID (live or test) to create the organization in.",
			},
			"project_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Optional Stytch B2B project secret. If omitted, the resource will use STYTCH_B2B_LIVE_SECRET/TEST env vars based on project_id.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Organization name.",
			},
			"slug": schema.StringAttribute{
				Optional:    true,
				Description: "Organization slug. If omitted, Stytch may derive one.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp returned by Stytch.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp returned by Stytch.",
			},
			"last_updated": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp of the last update by Terraform.",
			},
			"allow_destroy": schema.BoolAttribute{
				Optional:    true,
				Description: "Explicitly allow Terraform to destroy this organization (safety lever).",
			},
			// Expanded policy fields, exposed as computed for now
			"allowed_auth_methods": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Allowed auth methods.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf(
						"sso", "magic_link", "email_otp", "password", "google_oauth", "microsoft_oauth", "slack_oauth", "github_oauth", "hubspot_oauth",
					)),
				},
			},
			"auth_methods": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Auth methods policy.",
				Validators:  []validator.String{stringvalidator.OneOf("ALL_ALLOWED", "RESTRICTED")},
			},
			"email_allowed_domains": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Email allowed domains."},
			"email_invites": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Email invites policy.",
				Validators:  []validator.String{stringvalidator.OneOf("ALL_ALLOWED", "RESTRICTED", "NOT_ALLOWED")},
			},
			"email_jit_provisioning": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Email JIT provisioning policy.",
				Validators:  []validator.String{stringvalidator.OneOf("ALL_ALLOWED", "RESTRICTED", "NOT_ALLOWED")},
			},
			"sso_jit_provisioning": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "SSO JIT provisioning policy.",
				Validators:  []validator.String{stringvalidator.OneOf("ALL_ALLOWED", "RESTRICTED", "NOT_ALLOWED")},
			},
			"oauth_tenant_jit_provisioning": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "OAuth tenant JIT provisioning policy.",
				Validators:  []validator.String{stringvalidator.OneOf("RESTRICTED", "NOT_ALLOWED")},
			},
			"allowed_oauth_tenants": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.ListType{ElemType: types.StringType},
				Description: "Map of allowed OAuth tenants (keys: slack, hubspot, github) used when oauth_tenant_jit_provisioning is RESTRICTED. Values are lists of tenant identifiers per provider.",
			},
			"first_party_connected_apps_allowed_type": schema.StringAttribute{Optional: true, Computed: true, Description: "First-party connected apps allowed type."},
			"third_party_connected_apps_allowed_type": schema.StringAttribute{Optional: true, Computed: true, Description: "Third-party connected apps allowed type."},
			"rbac_email_implicit_role_assignments": schema.SetNestedAttribute{
				Computed:    true,
				Description: "Implicit role assignments by email domain.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"domain":  schema.StringAttribute{Computed: true},
					"role_id": schema.StringAttribute{Computed: true},
				}},
			},
			"mfa_policy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "MFA policy.",
				Validators:  []validator.String{stringvalidator.OneOf("REQUIRED_FOR_ALL", "OPTIONAL")},
			},
			"mfa_methods": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "MFA methods policy.",
				Validators:  []validator.String{stringvalidator.OneOf("ALL_ALLOWED", "RESTRICTED")},
			},
			"allowed_mfa_methods": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Allowed MFA methods when mfa_methods=RESTRICTED.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("sms_otp", "totp")),
				},
			},
			"sso_jit_provisioning_allowed_connections": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Allowed SSO connection IDs when sso_jit_provisioning=RESTRICTED.",
			},
		},
	}
}

func (r *b2bOrganizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// No provider data required; resource uses B2B Auth SDK via project_id/secret from plan/state
}

func (r *b2bOrganizationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg b2bOrganizationModel
	diags := req.Config.Get(ctx, &cfg)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If email_* is RESTRICTED, require at least one email_allowed_domains
	restrictedEmail := (cfg.EmailInvites.ValueString() == "RESTRICTED") || (cfg.EmailJITProvisioning.ValueString() == "RESTRICTED")
	if restrictedEmail {
		if cfg.EmailAllowedDomains.IsNull() || cfg.EmailAllowedDomains.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("email_allowed_domains"), "email_allowed_domains required when RESTRICTED", "Provide at least one domain when email_invites or email_jit_provisioning is RESTRICTED.")
		} else {
			var domains []string
			_ = cfg.EmailAllowedDomains.ElementsAs(ctx, &domains, false)
			if len(domains) == 0 {
				resp.Diagnostics.AddAttributeError(path.Root("email_allowed_domains"), "email_allowed_domains required when RESTRICTED", "Provide at least one domain when email_invites or email_jit_provisioning is RESTRICTED.")
			}
		}
	}

	// If oauth_tenant_jit_provisioning is RESTRICTED, we should require allowed_oauth_tenants (not yet modeled). For now, hint.
	if cfg.OAuthTenantJITProvisioning.ValueString() == "RESTRICTED" {
		if cfg.AllowedOAuthTenants.IsNull() || cfg.AllowedOAuthTenants.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("allowed_oauth_tenants"), "allowed_oauth_tenants required when oauth_tenant_jit_provisioning is RESTRICTED", "Provide at least one allowed OAuth tenant (keys: slack, hubspot, github). See Stytch docs.")
		} else {
			var tenants map[string][]string
			_ = cfg.AllowedOAuthTenants.ElementsAs(ctx, &tenants, false)
			if len(tenants) == 0 {
				resp.Diagnostics.AddAttributeError(path.Root("allowed_oauth_tenants"), "allowed_oauth_tenants required when oauth_tenant_jit_provisioning is RESTRICTED", "Provide at least one allowed OAuth tenant (keys: slack, hubspot, github). See Stytch docs.")
			} else {
				// ensure at least one provider has a non-empty list and keys are supported
				hasAny := false
				for k, v := range tenants {
					if k != "slack" && k != "hubspot" && k != "github" {
						resp.Diagnostics.AddAttributeError(path.Root("allowed_oauth_tenants"), "unsupported oauth tenant provider key", "Supported keys are slack, hubspot, github.")
						break
					}
					if len(v) > 0 {
						hasAny = true
					}
				}
				if !hasAny {
					resp.Diagnostics.AddAttributeError(path.Root("allowed_oauth_tenants"), "allowed_oauth_tenants must contain at least one tenant id", "Provide a non-empty list for at least one provider key.")
				}
			}
		}
	}

	// If mfa_methods is RESTRICTED, require allowed_mfa_methods non-empty
	if cfg.MFAMethods.ValueString() == "RESTRICTED" {
		if cfg.AllowedMFAMethods.IsNull() || cfg.AllowedMFAMethods.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("allowed_mfa_methods"), "allowed_mfa_methods required when mfa_methods is RESTRICTED", "Provide at least one MFA method: sms_otp or totp.")
		} else {
			var mlist []string
			_ = cfg.AllowedMFAMethods.ElementsAs(ctx, &mlist, false)
			if len(mlist) == 0 {
				resp.Diagnostics.AddAttributeError(path.Root("allowed_mfa_methods"), "allowed_mfa_methods required when mfa_methods is RESTRICTED", "Provide at least one MFA method: sms_otp or totp.")
			}
		}
	}

	// If sso_jit_provisioning is RESTRICTED, require sso_jit_provisioning_allowed_connections non-empty
	if cfg.SSOJITProvisioning.ValueString() == "RESTRICTED" {
		if cfg.SSOJITProvisioningAllowedConnections.IsNull() || cfg.SSOJITProvisioningAllowedConnections.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("sso_jit_provisioning_allowed_connections"), "sso_jit_provisioning_allowed_connections required when sso_jit_provisioning is RESTRICTED", "Provide at least one allowed SSO connection id.")
		} else {
			var clist []string
			_ = cfg.SSOJITProvisioningAllowedConnections.ElementsAs(ctx, &clist, false)
			if len(clist) == 0 {
				resp.Diagnostics.AddAttributeError(path.Root("sso_jit_provisioning_allowed_connections"), "sso_jit_provisioning_allowed_connections required when sso_jit_provisioning is RESTRICTED", "Provide at least one allowed SSO connection id.")
			}
		}
	}
}

func (r *b2bOrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan b2bOrganizationModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "project_id", plan.ProjectID.ValueString())
	tflog.Info(ctx, "Creating B2B organization")

	secret := plan.ProjectSecret.ValueString()
	if secret == "" {
		// provider-level precedence over env
		if isLiveProjectID(plan.ProjectID.ValueString()) {
			if ProviderB2BLiveSecret != "" {
				secret = ProviderB2BLiveSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
			}
		} else {
			if ProviderB2BTestSecret != "" {
				secret = ProviderB2BTestSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
			}
		}
	}
	client, err := b2bstytchapi.NewClient(plan.ProjectID.ValueString(), secret)
	if err != nil {
		resp.Diagnostics.AddError("failed to create b2b client", summarizeStytchError(err))
		return
	}

	params := &organizations.CreateParams{OrganizationName: plan.Name.ValueString()}
	if !plan.Slug.IsNull() && !plan.Slug.IsUnknown() && plan.Slug.ValueString() != "" {
		params.OrganizationSlug = plan.Slug.ValueString()
	}
	createResp, err := client.Organizations.Create(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("failed to create organization", summarizeStytchError(err))
		return
	}

	plan.ID = types.StringValue(createResp.Organization.OrganizationID)
	if plan.Slug.IsNull() || plan.Slug.IsUnknown() {
		if createResp.Organization.OrganizationSlug != "" {
			plan.Slug = types.StringValue(createResp.Organization.OrganizationSlug)
		}
	}
	if createResp.Organization.CreatedAt != nil {
		plan.CreatedAt = types.StringValue(createResp.Organization.CreatedAt.Format(time.RFC3339))
	}
	if createResp.Organization.UpdatedAt != nil {
		plan.UpdatedAt = types.StringValue(createResp.Organization.UpdatedAt.Format(time.RFC3339))
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Map expanded fields from response JSON
	mapExtendedOrgFields(ctx, &createResp.Organization, &plan)

	// Do not persist secrets in state
	plan.ProjectSecret = types.StringNull()

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *b2bOrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state b2bOrganizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	secret := state.ProjectSecret.ValueString()
	if secret == "" {
		if isLiveProjectID(state.ProjectID.ValueString()) {
			if ProviderB2BLiveSecret != "" {
				secret = ProviderB2BLiveSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
			}
		} else {
			if ProviderB2BTestSecret != "" {
				secret = ProviderB2BTestSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
			}
		}
	}
	client, err := b2bstytchapi.NewClient(state.ProjectID.ValueString(), secret)
	if err != nil {
		resp.Diagnostics.AddError("failed to create b2b client", summarizeStytchError(err))
		return
	}

	// Prefer Get by ID; fallback to Search if needed
	if !state.ID.IsNull() && state.ID.ValueString() != "" {
		getResp, err := client.Organizations.Get(ctx, &organizations.GetParams{OrganizationID: state.ID.ValueString()})
		if err == nil {
			// refresh name/slug from remote
			state.Name = types.StringValue(getResp.Organization.OrganizationName)
			if getResp.Organization.OrganizationSlug != "" {
				state.Slug = types.StringValue(getResp.Organization.OrganizationSlug)
			}
			if getResp.Organization.CreatedAt != nil {
				state.CreatedAt = types.StringValue(getResp.Organization.CreatedAt.Format(time.RFC3339))
			}
			if getResp.Organization.UpdatedAt != nil {
				state.UpdatedAt = types.StringValue(getResp.Organization.UpdatedAt.Format(time.RFC3339))
			}
			mapExtendedOrgFields(ctx, &getResp.Organization, &state)
			// Do not persist secrets in state
			state.ProjectSecret = types.StringNull()
			diags = resp.State.Set(ctx, state)
			resp.Diagnostics.Append(diags...)
			return
		}
		// else try search below
	}

	// Search by slug if present
	var found *organizations.Organization
	cursor := ""
	for {
		sr, err := client.Organizations.Search(ctx, &organizations.SearchParams{Limit: 1000, Cursor: cursor})
		if err != nil {
			resp.Diagnostics.AddError("search failed", summarizeStytchError(err))
			return
		}
		for i := range sr.Organizations {
			if sr.Organizations[i].OrganizationID == state.ID.ValueString() || (!state.Slug.IsNull() && sr.Organizations[i].OrganizationSlug == state.Slug.ValueString()) {
				found = &sr.Organizations[i]
				break
			}
		}
		if found != nil || sr.ResultsMetadata.NextCursor == "" {
			break
		}
		cursor = sr.ResultsMetadata.NextCursor
	}
	if found == nil {
		// Orphaned: organization deleted outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}
	state.ID = types.StringValue(found.OrganizationID)
	state.Name = types.StringValue(found.OrganizationName)
	if found.OrganizationSlug != "" {
		state.Slug = types.StringValue(found.OrganizationSlug)
	}
	if found.CreatedAt != nil {
		state.CreatedAt = types.StringValue(found.CreatedAt.Format(time.RFC3339))
	}
	if found.UpdatedAt != nil {
		state.UpdatedAt = types.StringValue(found.UpdatedAt.Format(time.RFC3339))
	}
	mapExtendedOrgFields(ctx, found, &state)
	// Do not persist secrets in state
	state.ProjectSecret = types.StringNull()
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *b2bOrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan b2bOrganizationModel
	var state b2bOrganizationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secret := state.ProjectSecret.ValueString()
	if secret == "" {
		if isLiveProjectID(state.ProjectID.ValueString()) {
			if ProviderB2BLiveSecret != "" {
				secret = ProviderB2BLiveSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
			}
		} else {
			if ProviderB2BTestSecret != "" {
				secret = ProviderB2BTestSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
			}
		}
	}
	client, err := b2bstytchapi.NewClient(state.ProjectID.ValueString(), secret)
	if err != nil {
		resp.Diagnostics.AddError("failed to create b2b client", summarizeStytchError(err))
		return
	}

	// Minimal update: name, slug, and selected policy fields
	upd := &organizations.UpdateParams{OrganizationID: state.ID.ValueString()}
	if !plan.Name.IsNull() && plan.Name.ValueString() != state.Name.ValueString() {
		upd.OrganizationName = plan.Name.ValueString()
	}
	if !plan.Slug.IsNull() && plan.Slug.ValueString() != state.Slug.ValueString() {
		upd.OrganizationSlug = plan.Slug.ValueString()
	}
	if !plan.AuthMethods.IsNull() && plan.AuthMethods.ValueString() != "" {
		upd.AuthMethods = plan.AuthMethods.ValueString()
	}
	if !plan.EmailInvites.IsNull() && plan.EmailInvites.ValueString() != "" {
		upd.EmailInvites = plan.EmailInvites.ValueString()
	}
	if !plan.EmailJITProvisioning.IsNull() && plan.EmailJITProvisioning.ValueString() != "" {
		upd.EmailJITProvisioning = plan.EmailJITProvisioning.ValueString()
	}
	if !plan.SSOJITProvisioning.IsNull() && plan.SSOJITProvisioning.ValueString() != "" {
		upd.SSOJITProvisioning = plan.SSOJITProvisioning.ValueString()
	}
	if !plan.OAuthTenantJITProvisioning.IsNull() && plan.OAuthTenantJITProvisioning.ValueString() != "" {
		upd.OAuthTenantJITProvisioning = plan.OAuthTenantJITProvisioning.ValueString()
	}
	// Note: connected apps allowed type may be enum types in the SDK; skip until modeled precisely
	if !plan.AllowedAuthMethods.IsNull() {
		var list []string
		_ = plan.AllowedAuthMethods.ElementsAs(ctx, &list, false)
		// send even when empty to clear remote when policy allows
		upd.AllowedAuthMethods = list
	}
	if !plan.EmailAllowedDomains.IsNull() {
		var dlist []string
		_ = plan.EmailAllowedDomains.ElementsAs(ctx, &dlist, false)
		// send even when empty to clear remote when policy allows
		upd.EmailAllowedDomains = dlist
	}
	if !plan.AllowedOAuthTenants.IsNull() {
		// Expect map[string][]string; filter keys
		var tenants map[string][]string
		_ = plan.AllowedOAuthTenants.ElementsAs(ctx, &tenants, false)
		filtered := map[string]any{}
		for k, v := range tenants {
			if k == "slack" || k == "hubspot" || k == "github" {
				filtered[k] = v
			}
		}
		// Only set when policy is RESTRICTED or list is non-empty; otherwise omit to avoid unexpected null diffs
		if plan.OAuthTenantJITProvisioning.ValueString() == "RESTRICTED" || len(filtered) > 0 {
			upd.AllowedOAuthTenants = filtered
		}
	}
	if !plan.MFAPolicy.IsNull() && plan.MFAPolicy.ValueString() != "" {
		upd.MFAPolicy = plan.MFAPolicy.ValueString()
	}
	if !plan.MFAMethods.IsNull() && plan.MFAMethods.ValueString() != "" {
		upd.MFAMethods = plan.MFAMethods.ValueString()
	}
	if !plan.AllowedMFAMethods.IsNull() {
		var mlist []string
		_ = plan.AllowedMFAMethods.ElementsAs(ctx, &mlist, false)
		if len(mlist) > 0 {
			upd.AllowedMFAMethods = mlist
		}
	}
	if !plan.SSOJITProvisioningAllowedConnections.IsNull() {
		var connlist []string
		_ = plan.SSOJITProvisioningAllowedConnections.ElementsAs(ctx, &connlist, false)
		if len(connlist) > 0 {
			upd.SSOJITProvisioningAllowedConnections = connlist
		}
	}

	if upd.OrganizationName == "" && upd.OrganizationSlug == "" && upd.AuthMethods == "" && len(upd.AllowedAuthMethods) == 0 && upd.EmailInvites == "" && upd.EmailJITProvisioning == "" && len(upd.EmailAllowedDomains) == 0 && upd.SSOJITProvisioning == "" && upd.OAuthTenantJITProvisioning == "" && len(upd.AllowedOAuthTenants) == 0 && upd.MFAPolicy == "" && upd.MFAMethods == "" && len(upd.AllowedMFAMethods) == 0 && len(upd.SSOJITProvisioningAllowedConnections) == 0 {
		// Persist allow_destroy even if no remote changes are needed
		state.AllowDestroy = plan.AllowDestroy
		// Refresh computed fields to avoid unknowns
		if state.ID.ValueString() != "" {
			if gr, err := client.Organizations.Get(ctx, &organizations.GetParams{OrganizationID: state.ID.ValueString()}); err == nil {
				// Update timestamps and extended fields
				if gr.Organization.CreatedAt != nil {
					state.CreatedAt = types.StringValue(gr.Organization.CreatedAt.Format(time.RFC3339))
				}
				if gr.Organization.UpdatedAt != nil {
					state.UpdatedAt = types.StringValue(gr.Organization.UpdatedAt.Format(time.RFC3339))
				}
				mapExtendedOrgFields(ctx, &gr.Organization, &state)
			}
		}
		state.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))
		// Do not persist secrets in state
		state.ProjectSecret = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		return
	}
	ur, err := client.Organizations.Update(ctx, upd)
	if err != nil {
		resp.Diagnostics.AddError("failed to update organization", summarizeStytchError(err))
		return
	}

	state.Name = types.StringValue(ur.Organization.OrganizationName)
	if ur.Organization.OrganizationSlug != "" {
		state.Slug = types.StringValue(ur.Organization.OrganizationSlug)
	}
	if ur.Organization.CreatedAt != nil {
		state.CreatedAt = types.StringValue(ur.Organization.CreatedAt.Format(time.RFC3339))
	}
	if ur.Organization.UpdatedAt != nil {
		state.UpdatedAt = types.StringValue(ur.Organization.UpdatedAt.Format(time.RFC3339))
	}
	// Refresh computed policy fields from the response and ensure deterministic empty values
	mapExtendedOrgFields(ctx, &ur.Organization, &state)
	// Persist allow_destroy from plan to state so deletes can be enabled via config
	state.AllowDestroy = plan.AllowDestroy
	state.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))
	// Do not persist secrets in state
	state.ProjectSecret = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *b2bOrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state b2bOrganizationModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Safety lever
	if state.AllowDestroy.IsNull() || !state.AllowDestroy.ValueBool() {
		resp.Diagnostics.AddError("destroy blocked", "Set allow_destroy=true to permit deleting this organization")
		return
	}

	// Resolve secret with provider-level precedence over env
	secret := state.ProjectSecret.ValueString()
	if secret == "" {
		if isLiveProjectID(state.ProjectID.ValueString()) {
			if ProviderB2BLiveSecret != "" {
				secret = ProviderB2BLiveSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
			}
		} else {
			if ProviderB2BTestSecret != "" {
				secret = ProviderB2BTestSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
			}
		}
	}
	if secret == "" {
		resp.Diagnostics.AddError("missing project secret", "provide project_secret in the resource or configure provider-level b2b secrets or set STYTCH_B2B_LIVE_SECRET/TEST env vars")
		return
	}

	client, err := b2bstytchapi.NewClient(state.ProjectID.ValueString(), secret)
	if err != nil {
		resp.Diagnostics.AddError("failed to create b2b client", summarizeStytchError(err))
		return
	}

	_, err = client.Organizations.Delete(ctx, &organizations.DeleteParams{OrganizationID: state.ID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("failed to delete organization", summarizeStytchError(err))
		return
	}
}

func (r *b2bOrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Support import as:
	//  - "project_id,organization_id"
	//  - "project_id,slug" (resolve to organization_id)
	id := req.ID
	var projectID, second string
	for i := 0; i < len(id); i++ {
		if id[i] == ',' {
			projectID = id[:i]
			second = id[i+1:]
			break
		}
	}
	if projectID == "" || second == "" {
		resp.Diagnostics.AddError("invalid import id", "expected 'project_id,organization_id' or 'project_id,slug'")
		return
	}
	orgID := second
	if !strings.HasPrefix(second, "organization-") {
		// treat as slug; resolve to ID using provider/env secrets
		secret := ""
		if isLiveProjectID(projectID) {
			if ProviderB2BLiveSecret != "" {
				secret = ProviderB2BLiveSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
			}
		} else {
			if ProviderB2BTestSecret != "" {
				secret = ProviderB2BTestSecret
			} else {
				secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
			}
		}
		if secret == "" {
			resp.Diagnostics.AddError("missing secret", "set provider b2b secrets or env vars for import")
			return
		}
		client, err := b2bstytchapi.NewClient(projectID, secret)
		if err != nil {
			resp.Diagnostics.AddError("failed to create b2b client", summarizeStytchError(err))
			return
		}
		cursor := ""
		var found *organizations.Organization
		for {
			sr, err := client.Organizations.Search(ctx, &organizations.SearchParams{Limit: 1000, Cursor: cursor})
			if err != nil {
				resp.Diagnostics.AddError("search failed", summarizeStytchError(err))
				return
			}
			for i := range sr.Organizations {
				if sr.Organizations[i].OrganizationSlug == second {
					found = &sr.Organizations[i]
					break
				}
			}
			if found != nil || sr.ResultsMetadata.NextCursor == "" {
				break
			}
			cursor = sr.ResultsMetadata.NextCursor
		}
		if found == nil {
			resp.Diagnostics.AddError("not found", "organization with provided slug not found")
			return
		}
		orgID = found.OrganizationID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), orgID)...)
}

// mapExtendedOrgFields maps additional organization policy fields into the resource model.
func mapExtendedOrgFields(ctx context.Context, org *organizations.Organization, m *b2bOrganizationModel) {
	if org == nil {
		return
	}
	// Initialize to known empty values so Terraform never sees unknowns after apply
	emptyList := func() types.List { lv, _ := types.ListValueFrom(ctx, types.StringType, []string{}); return lv }
	emptySet := func() types.Set {
		objType := map[string]attr.Type{"domain": types.StringType, "role_id": types.StringType}
		sv, _ := types.SetValue(types.ObjectType{AttrTypes: objType}, []attr.Value{})
		return sv
	}
	emptyMap := func() types.Map {
		mv, _ := types.MapValue(types.ListType{ElemType: types.StringType}, map[string]attr.Value{})
		return mv
	}
	m.AllowedAuthMethods = emptyList()
	m.AuthMethods = types.StringValue("")
	m.EmailAllowedDomains = emptyList()
	m.EmailInvites = types.StringValue("")
	m.EmailJITProvisioning = types.StringValue("")
	m.SSOJITProvisioning = types.StringValue("")
	m.OAuthTenantJITProvisioning = types.StringValue("")
	m.AllowedOAuthTenants = emptyMap()
	m.FirstPartyConnectedAppsAllowedType = types.StringValue("")
	m.ThirdPartyConnectedAppsAllowedType = types.StringValue("")
	m.RBACEmailImplicitRoleAssignments = emptySet()
	var generic map[string]any
	if b, err := json.Marshal(org); err == nil {
		_ = json.Unmarshal(b, &generic)
	}
	// helpers
	toString := func(v any) string {
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
	toStringSlice := func(v any) []string {
		arr, ok := v.([]any)
		if !ok {
			if sarr, ok2 := v.([]string); ok2 {
				return sarr
			}
			return nil
		}
		out := make([]string, 0, len(arr))
		for _, e := range arr {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	setList := func(vals []string) types.List { lv, _ := types.ListValueFrom(ctx, types.StringType, vals); return lv }

	if v, ok := generic["auth_methods"]; ok {
		m.AuthMethods = types.StringValue(toString(v))
	}
	if v, ok := generic["allowed_auth_methods"]; ok {
		m.AllowedAuthMethods = setList(toStringSlice(v))
	}
	if v, ok := generic["email_allowed_domains"]; ok {
		m.EmailAllowedDomains = setList(toStringSlice(v))
	}
	if v, ok := generic["email_invites"]; ok {
		m.EmailInvites = types.StringValue(toString(v))
	}
	if v, ok := generic["email_jit_provisioning"]; ok {
		m.EmailJITProvisioning = types.StringValue(toString(v))
	}
	if v, ok := generic["sso_jit_provisioning"]; ok {
		m.SSOJITProvisioning = types.StringValue(toString(v))
	}
	if v, ok := generic["oauth_tenant_jit_provisioning"]; ok {
		m.OAuthTenantJITProvisioning = types.StringValue(toString(v))
	}
	if v, ok := generic["allowed_oauth_tenants"]; ok {
		if mp, ok2 := v.(map[string]any); ok2 {
			mv := map[string]attr.Value{}
			for k, e := range mp {
				lv, _ := types.ListValueFrom(ctx, types.StringType, toStringSlice(e))
				mv[k] = lv
			}
			mvVal, _ := types.MapValue(types.ListType{ElemType: types.StringType}, mv)
			m.AllowedOAuthTenants = mvVal
		}
	}
	if v, ok := generic["first_party_connected_apps_allowed_type"]; ok {
		m.FirstPartyConnectedAppsAllowedType = types.StringValue(toString(v))
	}
	if v, ok := generic["third_party_connected_apps_allowed_type"]; ok {
		m.ThirdPartyConnectedAppsAllowedType = types.StringValue(toString(v))
	}
	if v, ok := generic["rbac_email_implicit_role_assignments"]; ok {
		arr, _ := v.([]any)
		type item struct {
			Domain string `json:"domain"`
			RoleID string `json:"role_id"`
		}
		items := make([]item, 0, len(arr))
		for _, e := range arr {
			if mp, ok := e.(map[string]any); ok {
				items = append(items, item{Domain: toString(mp["domain"]), RoleID: toString(mp["role_id"])})
			}
		}
		objType := map[string]attr.Type{"domain": types.StringType, "role_id": types.StringType}
		elems := make([]attr.Value, 0, len(items))
		for _, it := range items {
			ov, _ := types.ObjectValue(objType, map[string]attr.Value{"domain": types.StringValue(it.Domain), "role_id": types.StringValue(it.RoleID)})
			elems = append(elems, ov)
		}
		sv, _ := types.SetValue(types.ObjectType{AttrTypes: objType}, elems)
		m.RBACEmailImplicitRoleAssignments = sv
	}
	if v, ok := generic["mfa_policy"]; ok {
		m.MFAPolicy = types.StringValue(toString(v))
	}
	if v, ok := generic["mfa_methods"]; ok {
		m.MFAMethods = types.StringValue(toString(v))
	}
	if v, ok := generic["allowed_mfa_methods"]; ok {
		m.AllowedMFAMethods = setList(toStringSlice(v))
	}
	if v, ok := generic["sso_jit_provisioning_allowed_connections"]; ok {
		m.SSOJITProvisioningAllowedConnections = setList(toStringSlice(v))
	}
	// Normalize: when policies are not RESTRICTED, clear corresponding allowlists to avoid plan/apply inconsistencies
	if m.AuthMethods.ValueString() != "RESTRICTED" {
		m.AllowedAuthMethods = emptyList()
	}
	if m.OAuthTenantJITProvisioning.ValueString() != "RESTRICTED" {
		m.AllowedOAuthTenants = emptyMap()
	}
	// Debug summary for troubleshooting mapping
	tflog.Debug(ctx, "mapped extended organization fields", map[string]any{
		"org_id":                    org.OrganizationID,
		"has_allowed_auth_methods":  generic != nil && generic["allowed_auth_methods"] != nil,
		"has_email_allowed_domains": generic != nil && generic["email_allowed_domains"] != nil,
		"has_rbac_implicit_roles":   generic != nil && generic["rbac_email_implicit_role_assignments"] != nil,
		"auth_methods":              m.AuthMethods.ValueString(),
		"email_invites":             m.EmailInvites.ValueString(),
		"email_jit_provisioning":    m.EmailJITProvisioning.ValueString(),
		"sso_jit_provisioning":      m.SSOJITProvisioning.ValueString(),
	})
}

// isLiveProjectID returns true if the project id matches the live prefix pattern.
func isLiveProjectID(projectID string) bool {
	return strings.HasPrefix(projectID, "project-live-")
}

// summarizeStytchError extracts a concise error summary from Stytch errors when possible.
func summarizeStytchError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	typeIdx := strings.Index(msg, "type:")
	if typeIdx >= 0 {
		rest := msg[typeIdx:]
		comma := strings.Index(rest, ",")
		if comma > 0 {
			rest = rest[:comma]
		}
		return fmt.Sprintf("%s (%s)", msg, strings.TrimSpace(rest))
	}
	return msg
}
