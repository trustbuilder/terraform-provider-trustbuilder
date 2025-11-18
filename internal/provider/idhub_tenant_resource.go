package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trustbuilder/terraform-provider-trustbuilder/internal/apiclient"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &idhubTenantResource{}
)

// idhubTenantResource is the resource implementation.
type idhubTenantResource struct {
	url    string
	client *apiclient.APIClient
}

// idhubTenantResourceModel maps the resource schema data.
type idhubTenantResourceModel struct {
	Headers        types.Map    `tfsdk:"headers"`
	LastUpdated    types.String `tfsdk:"last_updated"`
	Id             types.String `tfsdk:"id"`
	Tenant         types.String `tfsdk:"tenant"`
	RepoNamePrefix types.String `tfsdk:"repo_name_prefix"`
	Path           types.String `tfsdk:"path"`
	Data           types.String `tfsdk:"data"`
}

// NewtenantResource is a helper function to simplify the provider implementation.
func NewTenantResource() resource.Resource {
	return &idhubTenantResource{}
}

// Metadata returns the resource type name.
func (r *idhubTenantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_idhub_tenant"
}

// Schema defines the schema for the resource.
func (r *idhubTenantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Resource managing the creation of an idhub tenant.",
		Attributes: map[string]schema.Attribute{
			"headers": schema.MapAttribute{
				Description: "A map of header names and values to set on all outbound requests.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"last_updated": schema.StringAttribute{
				Description: "Resource update date in RFC850 format.",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Description: "The UUID of this resource.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant": schema.StringAttribute{
				Description: "Tenant name used as identifier.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"repo_name_prefix": schema.StringAttribute{
				Description: "Another identifier of the tenant.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description: "The API path on top of the base URL set in the provider that represents objects of this type on the API server.",
				Required:    true,
			},
			"data": schema.StringAttribute{
				Description: "Valid JSON object that this provider will manage with the API server.",
				Required:    true,
				WriteOnly:   true,
			},
		},
	}
}

// Create a new resource.
func (r *idhubTenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var planResource idhubTenantResourceModel
	var dataAttribute types.String

	diags := req.Plan.Get(ctx, &planResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = req.Config.GetAttribute(ctx, path.Root("data"), &dataAttribute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if dataAttribute.IsNull() || dataAttribute.IsUnknown() {
		resp.Diagnostics.AddError(
			"Missing data attribute",
			"The 'data' attribute must be provided.",
		)
		return
	}

	responseData, err := r.client.SendRequest("POST", planResource.Path.ValueString(), dataAttribute.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Create request error", fmt.Sprintf("Creation request returned the error: %s", err))
		return
	}
	if err := (&planResource).update_computed_fields(responseData); err != nil {
		resp.Diagnostics.AddError("Missing attribute in create API response", fmt.Sprintf("Missing attribute in the creation response : %s", err))
		return
	}

	planResource.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, planResource)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information.
func (r *idhubTenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var stateResource idhubTenantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &stateResource)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := strings.TrimRight(stateResource.Path.ValueString(), "/") + "?identifier=" + stateResource.Tenant.ValueString()
	responseData, err := r.client.SendRequest("GET", path, "")
	if err != nil {
		resp.Diagnostics.AddError("Read request error", fmt.Sprintf("Read request returned the error: %s on the path: %s", err, path))
		return
	}
	if err := (&stateResource).update_computed_fields(responseData); err != nil {
		resp.Diagnostics.AddError("Missing attribute in read API response", fmt.Sprintf("Missing attribute in the read response : %s", err))
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *idhubTenantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var planResource idhubTenantResourceModel
	diags := req.Plan.Get(ctx, &planResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	planResource.LastUpdated = types.StringValue(time.Now().Format(time.RFC3339))
	state := idhubTenantResourceModel{
		Headers:        planResource.Headers,
		LastUpdated:    planResource.LastUpdated,
		Id:             planResource.Id,
		Tenant:         planResource.Tenant,
		RepoNamePrefix: planResource.RepoNamePrefix,
		Path:           planResource.Path,
		//omit Data
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *idhubTenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *idhubTenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: path,tenant. Got: %q", req.ID),
		)
		return
	}

	tenantPath := idParts[0]
	tenantName := idParts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), tenantPath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant"), tenantName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("last_updated"), time.Now().Format(time.RFC3339))...)

	requestPath := strings.TrimRight(tenantPath, "/") + "?identifier=" + tenantName
	//Get data from API
	responseData, err := r.client.SendRequest("GET", requestPath, "")
	if err != nil {
		resp.Diagnostics.AddError("Import request error", fmt.Sprintf("Import request returned the error: %s on the path: %s", err, requestPath))
		return
	}
	//Delete the array, to have only the object
	mapData, err := apiclient.JsonDecodeApiResponse(responseData)
	if err != nil {
		resp.Diagnostics.AddError("Import request error", fmt.Sprintf("JSON decoding issue on the API response: %s", err))
		return
	}

	id, ok := mapData["id"].(string)
	if !ok {
		resp.Diagnostics.AddError("Missing attribute in import API response", "Missing id attribute or it is not a string")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	repoNamePrefix, ok := mapData["repo_name_prefix"].(string)
	if !ok {
		resp.Diagnostics.AddError("Missing attribute in import API response", "Missing repo_name_prefix attribute or it is not a string")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("repo_name_prefix"), repoNamePrefix)...)
}

// Configure adds the provider configured client to the resource.
func (r *idhubTenantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {

	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apiclient.APIClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected string, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
	r.url = client.Uri
}

func (m *idhubTenantResourceModel) update_computed_fields(jsonData string) error {
	var id string
	var tenant string
	var repoNamePrefix string
	var err error

	id, err = apiclient.GetKeyValue(jsonData, "id")
	if err != nil {
		return err
	}
	tenant, err = apiclient.GetKeyValue(jsonData, "identifier")
	if err != nil {
		return err
	}
	repoNamePrefix, err = apiclient.GetKeyValue(jsonData, "repo_name_prefix")
	if err != nil {
		return err
	}

	m.Id = types.StringValue(id)
	m.Tenant = types.StringValue(tenant)
	m.RepoNamePrefix = types.StringValue(repoNamePrefix)
	return nil
}
