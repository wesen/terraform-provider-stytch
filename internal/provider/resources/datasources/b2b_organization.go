package datasources

import (
    "context"
    "strings"
    "encoding/json"

    "github.com/hashicorp/terraform-plugin-framework/attr"
    "github.com/hashicorp/terraform-plugin-framework/datasource"
    "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
    "github.com/hashicorp/terraform-plugin-framework/types"
    "github.com/hashicorp/terraform-plugin-log/tflog"
    "github.com/stytchauth/stytch-go/v16/stytch/b2b/b2bstytchapi"
    "github.com/stytchauth/stytch-go/v16/stytch/b2b/organizations"
)

var _ datasource.DataSource = &b2bOrganizationDataSource{}

func NewB2BOrganizationDataSource() datasource.DataSource { return &b2bOrganizationDataSource{} }

type b2bOrganizationDataSource struct{ pd *ProviderData }

type b2bOrgModel struct {
    ProjectID      types.String `tfsdk:"project_id"`
    ProjectSecret  types.String `tfsdk:"project_secret"`
    OrganizationID types.String `tfsdk:"organization_id"`

    // Selected fields (expand as needed)
    ID        types.String `tfsdk:"id"`
    Name      types.String `tfsdk:"name"`
    Slug      types.String `tfsdk:"slug"`
    CreatedAt types.String `tfsdk:"created_at"`
    UpdatedAt types.String `tfsdk:"updated_at"`
    AllowedAuthMethods types.List `tfsdk:"allowed_auth_methods"`
    AuthMethods types.String `tfsdk:"auth_methods"`
    EmailAllowedDomains types.List `tfsdk:"email_allowed_domains"`
    EmailInvites types.String `tfsdk:"email_invites"`
    EmailJITProvisioning types.String `tfsdk:"email_jit_provisioning"`
    SSOJITProvisioning types.String `tfsdk:"sso_jit_provisioning"`
    OAuthTenantJITProvisioning types.String `tfsdk:"oauth_tenant_jit_provisioning"`
    FirstPartyConnectedAppsAllowedType types.String `tfsdk:"first_party_connected_apps_allowed_type"`
    ThirdPartyConnectedAppsAllowedType types.String `tfsdk:"third_party_connected_apps_allowed_type"`
    RBACEmailImplicitRoleAssignments types.Set `tfsdk:"rbac_email_implicit_role_assignments"`
    RawJSON   types.String `tfsdk:"raw_json"`
}

func (d *b2bOrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_b2b_organization"
}

func (d *b2bOrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Description: "Reads a Stytch B2B organization via the B2B Auth API.",
        Attributes: map[string]schema.Attribute{
            "project_id": schema.StringAttribute{Required: true, Description: "The project ID (live or test) owning the organization."},
            "project_secret": schema.StringAttribute{Optional: true, Sensitive: true, Description: "Optional project secret; if omitted, provider-level b2b secrets will be used if configured."},
            "organization_id": schema.StringAttribute{Optional: true, Description: "Organization ID. If omitted, provide slug."},
            "slug": schema.StringAttribute{Optional: true, Computed: true, Description: "Organization slug. If organization_id is omitted, lookup by slug."},

            "id": schema.StringAttribute{Computed: true, Description: "Organization ID."},
            "name": schema.StringAttribute{Computed: true, Description: "Organization name."},
            "created_at": schema.StringAttribute{Computed: true, Description: "Creation timestamp."},
            "updated_at": schema.StringAttribute{Computed: true, Description: "Update timestamp."},
            "allowed_auth_methods": schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Allowed auth methods."},
            "auth_methods": schema.StringAttribute{Computed: true, Description: "Auth methods policy."},
            "email_allowed_domains": schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Email allowed domains."},
            "email_invites": schema.StringAttribute{Computed: true, Description: "Email invites policy."},
            "email_jit_provisioning": schema.StringAttribute{Computed: true, Description: "Email JIT provisioning policy."},
            "sso_jit_provisioning": schema.StringAttribute{Computed: true, Description: "SSO JIT provisioning policy."},
            "oauth_tenant_jit_provisioning": schema.StringAttribute{Computed: true, Description: "OAuth tenant JIT provisioning policy."},
            "first_party_connected_apps_allowed_type": schema.StringAttribute{Computed: true, Description: "First-party connected apps allowed type."},
            "third_party_connected_apps_allowed_type": schema.StringAttribute{Computed: true, Description: "Third-party connected apps allowed type."},
            "rbac_email_implicit_role_assignments": schema.SetNestedAttribute{
                Computed:    true,
                Description: "Implicit role assignments by email domain.",
                NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
                    "domain": schema.StringAttribute{Computed: true},
                    "role_id": schema.StringAttribute{Computed: true},
                }},
            },
            "raw_json": schema.StringAttribute{Computed: true, Description: "Raw organization JSON for debugging."},
        },
    }
}

func (d *b2bOrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
    if req.ProviderData == nil { return }
    pd, ok := req.ProviderData.(*ProviderData)
    if ok { d.pd = pd }
}

func (d *b2bOrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
    var data b2bOrgModel
    diags := req.Config.Get(ctx, &data)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    if d.pd == nil {
        resp.Diagnostics.AddError("provider data missing", "internal provider data not available")
        return
    }

    // Resolve secret precedence
    secret := data.ProjectSecret.ValueString()
    if secret == "" {
        if isLiveProjectID(data.ProjectID.ValueString()) { secret = d.pd.B2BLiveSecret } else { secret = d.pd.B2BTestSecret }
    }
    if secret == "" {
        resp.Diagnostics.AddError("missing project secret", "provide project_secret in the data source or configure provider-level b2b_live_secret/b2b_test_secret")
        return
    }

    tflog.Info(ctx, "Creating B2B client", map[string]any{"project_id": data.ProjectID.ValueString()})
    b2bClient, err := b2bstytchapi.NewClient(data.ProjectID.ValueString(), secret)
    if err != nil {
        resp.Diagnostics.AddError("failed to create b2b client", err.Error())
        return
    }

    // Direct get by ID via search (SDK primarily exposes Search)
    var org *organizations.Organization
    cursor := ""
    targetID := data.OrganizationID.ValueString()
    targetSlug := data.Slug.ValueString()
    for {
        params := &organizations.SearchParams{Limit: 1000}
        if cursor != "" { params.Cursor = cursor }
        res, err := b2bClient.Organizations.Search(ctx, params)
        if err != nil { resp.Diagnostics.AddError("search failed", err.Error()); return }
        for i := range res.Organizations {
            if (targetID != "" && res.Organizations[i].OrganizationID == targetID) || (targetID == "" && targetSlug != "" && res.Organizations[i].OrganizationSlug == targetSlug) {
                org = &res.Organizations[i]; break
            }
        }
        if org != nil || res.ResultsMetadata.NextCursor == "" { break }
        cursor = res.ResultsMetadata.NextCursor
    }

    if org == nil {
        resp.Diagnostics.AddError("not found", "organization not found")
        return
    }

    // Map fields (best-effort names; adjust to exact SDK fields)
    data.ID = types.StringValue(org.OrganizationID)
    data.Name = types.StringValue(org.OrganizationName)
    if org.OrganizationSlug != "" { data.Slug = types.StringValue(org.OrganizationSlug) }
    if org.CreatedAt != nil { data.CreatedAt = types.StringValue(org.CreatedAt.Format("2006-01-02T15:04:05Z07:00")) }
    if org.UpdatedAt != nil { data.UpdatedAt = types.StringValue(org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")) }
    // Map additional fields via generic JSON to avoid SDK type drift
    var generic map[string]any
    if b, err := json.Marshal(org); err == nil { data.RawJSON = types.StringValue(string(b)); _ = json.Unmarshal(b, &generic) }
    // helpers
    toString := func(v any) string { if s, ok := v.(string); ok { return s }; return "" }
    toStringSlice := func(v any) []string {
        arr, ok := v.([]any); if !ok { if sarr, ok2 := v.([]string); ok2 { return sarr } ; return nil }
        out := make([]string, 0, len(arr)); for _, e := range arr { if s, ok := e.(string); ok { out = append(out, s) } }
        return out
    }
    setList := func(vals []string) types.List { lv, _ := types.ListValueFrom(ctx, types.StringType, vals); return lv }
    // Populate extended fields
    if v, ok := generic["allowed_auth_methods"]; ok { data.AllowedAuthMethods = setList(toStringSlice(v)) }
    if v, ok := generic["auth_methods"]; ok { data.AuthMethods = types.StringValue(toString(v)) }
    if v, ok := generic["email_allowed_domains"]; ok { data.EmailAllowedDomains = setList(toStringSlice(v)) }
    if v, ok := generic["email_invites"]; ok { data.EmailInvites = types.StringValue(toString(v)) }
    if v, ok := generic["email_jit_provisioning"]; ok { data.EmailJITProvisioning = types.StringValue(toString(v)) }
    if v, ok := generic["sso_jit_provisioning"]; ok { data.SSOJITProvisioning = types.StringValue(toString(v)) }
    if v, ok := generic["oauth_tenant_jit_provisioning"]; ok { data.OAuthTenantJITProvisioning = types.StringValue(toString(v)) }
    if v, ok := generic["first_party_connected_apps_allowed_type"]; ok { data.FirstPartyConnectedAppsAllowedType = types.StringValue(toString(v)) }
    if v, ok := generic["third_party_connected_apps_allowed_type"]; ok { data.ThirdPartyConnectedAppsAllowedType = types.StringValue(toString(v)) }
    if v, ok := generic["rbac_email_implicit_role_assignments"]; ok {
        // v is []any of map[string]any with domain, role_id
        arr, _ := v.([]any)
        type item struct{ Domain string `json:"domain"`; RoleID string `json:"role_id"` }
        items := make([]item, 0, len(arr))
        for _, e := range arr { if m, ok := e.(map[string]any); ok { items = append(items, item{ Domain: toString(m["domain"]), RoleID: toString(m["role_id"]) }) } }
        // build set value
        objType := map[string]attr.Type{ "domain": types.StringType, "role_id": types.StringType }
        elems := make([]attr.Value, 0, len(items))
        for _, it := range items {
            ov, _ := types.ObjectValue(objType, map[string]attr.Value{ "domain": types.StringValue(it.Domain), "role_id": types.StringValue(it.RoleID) })
            elems = append(elems, ov)
        }
        sv, _ := types.SetValue(types.ObjectType{AttrTypes: objType}, elems)
        data.RBACEmailImplicitRoleAssignments = sv
    }

    diags = resp.State.Set(ctx, &data)
    resp.Diagnostics.Append(diags...)
}

func isLiveProjectID(projectID string) bool {
    return strings.Contains(projectID, "-live-")
}



