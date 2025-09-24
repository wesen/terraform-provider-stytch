package datasources

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/stytchauth/stytch-go/v16/stytch/b2b/b2bstytchapi"
	"github.com/stytchauth/stytch-go/v16/stytch/b2b/organizations"
)

var _ datasource.DataSource = &b2bOrganizationsDataSource{}

func NewB2BOrganizationsDataSource() datasource.DataSource { return &b2bOrganizationsDataSource{} }

type b2bOrganizationsDataSource struct{ pd *ProviderData }

type b2bOrganizationsModel struct {
	ProjectID     types.String `tfsdk:"project_id"`
	ProjectSecret types.String `tfsdk:"project_secret"`

	Organizations []b2bOrganizationsItem `tfsdk:"organizations"`
	TotalCount    types.Int64            `tfsdk:"total_count"`
}

type b2bOrganizationsItem struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Slug      types.String `tfsdk:"slug"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *b2bOrganizationsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_b2b_organizations"
}

func (d *b2bOrganizationsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists Stytch B2B organizations via the B2B Auth API.",
		Attributes: map[string]schema.Attribute{
			"project_id":     schema.StringAttribute{Required: true, Description: "The project ID (live or test)."},
			"project_secret": schema.StringAttribute{Optional: true, Sensitive: true, Description: "Optional project secret; if omitted, provider-level b2b secrets will be used if configured."},
			"total_count":    schema.Int64Attribute{Computed: true, Description: "Total organizations returned."},
			"organizations": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Organizations returned by the search.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":         schema.StringAttribute{Computed: true},
					"name":       schema.StringAttribute{Computed: true},
					"slug":       schema.StringAttribute{Computed: true},
					"created_at": schema.StringAttribute{Computed: true},
					"updated_at": schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *b2bOrganizationsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*ProviderData)
	if ok {
		d.pd = pd
	}
}

func (d *b2bOrganizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data b2bOrganizationsModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.pd == nil {
		resp.Diagnostics.AddError("provider data missing", "internal provider data not available")
		return
	}

	secret := data.ProjectSecret.ValueString()
	if secret == "" {
		if isLiveProjectID(data.ProjectID.ValueString()) {
			secret = d.pd.B2BLiveSecret
		} else {
			secret = d.pd.B2BTestSecret
		}
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

	// Collect all organizations via cursoring
	cursor := ""
	var items []b2bOrganizationsItem
	for {
		params := &organizations.SearchParams{Limit: 1000}
		if cursor != "" {
			params.Cursor = cursor
		}
		res, err := b2bClient.Organizations.Search(ctx, params)
		if err != nil {
			resp.Diagnostics.AddError("search failed", err.Error())
			return
		}
		for i := range res.Organizations {
			org := res.Organizations[i]
			item := b2bOrganizationsItem{ID: types.StringValue(org.OrganizationID), Name: types.StringValue(org.OrganizationName)}
			if org.OrganizationSlug != "" {
				item.Slug = types.StringValue(org.OrganizationSlug)
			}
			if org.CreatedAt != nil {
				item.CreatedAt = types.StringValue(org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
			}
			if org.UpdatedAt != nil {
				item.UpdatedAt = types.StringValue(org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
			}
			items = append(items, item)
		}
		if res.ResultsMetadata.NextCursor == "" {
			break
		}
		cursor = res.ResultsMetadata.NextCursor
	}

	data.Organizations = items
	data.TotalCount = types.Int64Value(int64(len(items)))

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
