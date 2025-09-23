package datasources

import (
    "context"
    "strings"

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
            "organization_id": schema.StringAttribute{Required: true, Description: "Organization ID."},

            "id": schema.StringAttribute{Computed: true, Description: "Organization ID."},
            "name": schema.StringAttribute{Computed: true, Description: "Organization name."},
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
    for {
        params := &organizations.SearchParams{Limit: 1000}
        if cursor != "" { params.Cursor = cursor }
        res, err := b2bClient.Organizations.Search(ctx, params)
        if err != nil { resp.Diagnostics.AddError("search failed", err.Error()); return }
        for i := range res.Organizations {
            if res.Organizations[i].OrganizationID == targetID { org = &res.Organizations[i]; break }
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

    diags = resp.State.Set(ctx, &data)
    resp.Diagnostics.Append(diags...)
}

func isLiveProjectID(projectID string) bool {
    return strings.Contains(projectID, "-live-")
}



