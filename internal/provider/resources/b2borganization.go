package resources

import (
    "context"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/hashicorp/terraform-plugin-framework/path"
    "github.com/hashicorp/terraform-plugin-framework/resource"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
                Required:    true,
                Sensitive:   true,
                Description: "Stytch B2B project secret for the given project_id.",
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
        },
    }
}

func (r *b2bOrganizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
    // No provider data required; resource uses B2B Auth SDK via project_id/secret from plan/state
}

func (r *b2bOrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var plan b2bOrganizationModel
    diags := req.Plan.Get(ctx, &plan)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    ctx = tflog.SetField(ctx, "project_id", plan.ProjectID.ValueString())
    tflog.Info(ctx, "Creating B2B organization")

    secret := plan.ProjectSecret.ValueString()
    if secret == "" {
        if strings.Contains(plan.ProjectID.ValueString(), "-live-") {
            secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
        } else {
            secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
        }
    }
    client, err := b2bstytchapi.NewClient(plan.ProjectID.ValueString(), secret)
    if err != nil { resp.Diagnostics.AddError("failed to create b2b client", err.Error()); return }

    params := &organizations.CreateParams{ OrganizationName: plan.Name.ValueString() }
    if !plan.Slug.IsNull() && !plan.Slug.IsUnknown() && plan.Slug.ValueString() != "" {
        params.OrganizationSlug = plan.Slug.ValueString()
    }
    createResp, err := client.Organizations.Create(ctx, params)
    if err != nil { resp.Diagnostics.AddError("failed to create organization", err.Error()); return }

    plan.ID = types.StringValue(createResp.Organization.OrganizationID)
    if plan.Slug.IsNull() || plan.Slug.IsUnknown() {
        if createResp.Organization.OrganizationSlug != "" { plan.Slug = types.StringValue(createResp.Organization.OrganizationSlug) }
    }
    if createResp.Organization.CreatedAt != nil { plan.CreatedAt = types.StringValue(createResp.Organization.CreatedAt.Format(time.RFC3339)) }
    if createResp.Organization.UpdatedAt != nil { plan.UpdatedAt = types.StringValue(createResp.Organization.UpdatedAt.Format(time.RFC3339)) }
    plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

    diags = resp.State.Set(ctx, plan)
    resp.Diagnostics.Append(diags...)
}

func (r *b2bOrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    var state b2bOrganizationModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    secret := state.ProjectSecret.ValueString()
    if secret == "" {
        if strings.Contains(state.ProjectID.ValueString(), "-live-") {
            secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
        } else {
            secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
        }
    }
    client, err := b2bstytchapi.NewClient(state.ProjectID.ValueString(), secret)
    if err != nil { resp.Diagnostics.AddError("failed to create b2b client", err.Error()); return }

    // Prefer Get by ID; fallback to Search if needed
    if !state.ID.IsNull() && state.ID.ValueString() != "" {
        getResp, err := client.Organizations.Get(ctx, &organizations.GetParams{ OrganizationID: state.ID.ValueString() })
        if err == nil {
            // refresh name/slug from remote
            state.Name = types.StringValue(getResp.Organization.OrganizationName)
            if getResp.Organization.OrganizationSlug != "" { state.Slug = types.StringValue(getResp.Organization.OrganizationSlug) }
            if getResp.Organization.CreatedAt != nil { state.CreatedAt = types.StringValue(getResp.Organization.CreatedAt.Format(time.RFC3339)) }
            if getResp.Organization.UpdatedAt != nil { state.UpdatedAt = types.StringValue(getResp.Organization.UpdatedAt.Format(time.RFC3339)) }
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
        sr, err := client.Organizations.Search(ctx, &organizations.SearchParams{ Limit: 1000, Cursor: cursor })
        if err != nil { resp.Diagnostics.AddError("search failed", err.Error()); return }
        for i := range sr.Organizations {
            if sr.Organizations[i].OrganizationID == state.ID.ValueString() || ( !state.Slug.IsNull() && sr.Organizations[i].OrganizationSlug == state.Slug.ValueString() ) {
                found = &sr.Organizations[i]; break
            }
        }
        if found != nil || sr.ResultsMetadata.NextCursor == "" { break }
        cursor = sr.ResultsMetadata.NextCursor
    }
    if found == nil {
        // Orphaned: organization deleted outside Terraform
        resp.State.RemoveResource(ctx)
        return
    }
    state.ID = types.StringValue(found.OrganizationID)
    state.Name = types.StringValue(found.OrganizationName)
    if found.OrganizationSlug != "" { state.Slug = types.StringValue(found.OrganizationSlug) }
    if found.CreatedAt != nil { state.CreatedAt = types.StringValue(found.CreatedAt.Format(time.RFC3339)) }
    if found.UpdatedAt != nil { state.UpdatedAt = types.StringValue(found.UpdatedAt.Format(time.RFC3339)) }
    diags = resp.State.Set(ctx, state)
    resp.Diagnostics.Append(diags...)
}

func (r *b2bOrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    var plan b2bOrganizationModel
    var state b2bOrganizationModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }

    secret := state.ProjectSecret.ValueString()
    if secret == "" {
        if strings.Contains(state.ProjectID.ValueString(), "-live-") {
            secret = os.Getenv("STYTCH_B2B_LIVE_SECRET")
        } else {
            secret = os.Getenv("STYTCH_B2B_TEST_SECRET")
        }
    }
    client, err := b2bstytchapi.NewClient(state.ProjectID.ValueString(), secret)
    if err != nil { resp.Diagnostics.AddError("failed to create b2b client", err.Error()); return }

    // Minimal update: name and slug
    upd := &organizations.UpdateParams{ OrganizationID: state.ID.ValueString() }
    if !plan.Name.IsNull() && plan.Name.ValueString() != state.Name.ValueString() { upd.OrganizationName = plan.Name.ValueString() }
    if !plan.Slug.IsNull() && plan.Slug.ValueString() != state.Slug.ValueString() { upd.OrganizationSlug = plan.Slug.ValueString() }

    if upd.OrganizationName == "" && upd.OrganizationSlug == "" {
        // nothing to change
        return
    }
    ur, err := client.Organizations.Update(ctx, upd)
    if err != nil { resp.Diagnostics.AddError("failed to update organization", err.Error()); return }

    state.Name = types.StringValue(ur.Organization.OrganizationName)
    if ur.Organization.OrganizationSlug != "" { state.Slug = types.StringValue(ur.Organization.OrganizationSlug) }
    state.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))
    resp.Diagnostics.Append(resp.State.Set(ctx, state)...)    
}

func (r *b2bOrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
    var state b2bOrganizationModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    // Safety lever
    if state.AllowDestroy.IsNull() || !state.AllowDestroy.ValueBool() {
        resp.Diagnostics.AddError("destroy blocked", "Set allow_destroy=true to permit deleting this organization")
        return
    }

    client, err := b2bstytchapi.NewClient(state.ProjectID.ValueString(), state.ProjectSecret.ValueString())
    if err != nil { resp.Diagnostics.AddError("failed to create b2b client", err.Error()); return }

    _, err = client.Organizations.Delete(ctx, &organizations.DeleteParams{ OrganizationID: state.ID.ValueString() })
    if err != nil { resp.Diagnostics.AddError("failed to delete organization", fmt.Sprintf("%v", err)); return }
}

func (r *b2bOrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
    // Support import ID as "project_id,organization_id"
    id := req.ID
    var projectID, orgID string
    for i := 0; i < len(id); i++ {
        if id[i] == ',' { projectID = id[:i]; orgID = id[i+1:]; break }
    }
    if projectID == "" || orgID == "" {
        resp.Diagnostics.AddError("invalid import id", "expected 'project_id,organization_id'")
        return
    }
    resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...) 
    resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), orgID)...) 
}



