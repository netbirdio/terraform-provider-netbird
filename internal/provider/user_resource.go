// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &User{}
var _ resource.ResourceWithImportState = &User{}

func NewUser() resource.Resource {
	return &User{}
}

// User defines the resource implementation.
type User struct {
	client *netbird.Client
}

// UserModel describes the resource data model.
type UserModel struct {
	Id            types.String `tfsdk:"id"`
	Email         types.String `tfsdk:"email"`
	Name          types.String `tfsdk:"name"`
	LastLogin     types.String `tfsdk:"last_login"`
	Role          types.String `tfsdk:"role"`
	Status        types.String `tfsdk:"status"`
	Issued        types.String `tfsdk:"issued"`
	AutoGroups    types.List   `tfsdk:"auto_groups"`
	IsCurrent     types.Bool   `tfsdk:"is_current"`
	IsServiceUser types.Bool   `tfsdk:"is_service_user"`
	IsBlocked     types.Bool   `tfsdk:"is_blocked"`
}

func (r *User) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *User) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Invite and Manage Users",
		MarkdownDescription: "Invite and Manage Users, see [NetBird Docs](https://docs.netbird.io/how-to/add-users-to-your-network) for more information, existing users should be imported.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of a user",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "User Email",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "User Name",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_login": schema.StringAttribute{
				MarkdownDescription: "User Last Login timedate",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "User's NetBird account role (owner|admin|user|billing_admin|auditor|network_admin).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("user"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.OneOf("owner", "admin", "user", "billing_admin", "auditor", "network_admin")},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "User status (active or invited)",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"issued": schema.StringAttribute{
				MarkdownDescription: "User issue method",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"auto_groups": schema.ListAttribute{
				MarkdownDescription: "Group IDs to auto-assign to peers registered by this user",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				Default:             listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"is_current": schema.BoolAttribute{
				MarkdownDescription: "Set to true if the caller user is the same as the resource user",
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"is_service_user": schema.BoolAttribute{
				MarkdownDescription: "If set to true, creates a Service Account User",
				Required:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"is_blocked": schema.BoolAttribute{
				MarkdownDescription: "If set to true then user is blocked and can't use the system",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *User) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func userAPIToTerraform(ctx context.Context, user *api.User, data *UserModel) diag.Diagnostics {
	var ret diag.Diagnostics
	data.Id = types.StringValue(user.Id)
	data.Name = types.StringValue(user.Name)
	data.LastLogin = types.StringValue(user.LastLogin.Format(time.RFC3339))
	data.Email = types.StringValue(user.Email)
	data.IsBlocked = types.BoolValue(user.IsBlocked)
	data.IsCurrent = types.BoolValue(*user.IsCurrent)
	data.IsServiceUser = types.BoolValue(*user.IsServiceUser)
	data.Issued = types.StringValue(*user.Issued)
	data.Role = types.StringValue(user.Role)
	data.Status = types.StringValue(string(user.Status))
	l, diag := types.ListValueFrom(ctx, types.StringType, user.AutoGroups)
	ret.Append(diag...)
	data.AutoGroups = l
	return ret
}

func (r *User) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	userReq := api.UserCreateRequest{
		AutoGroups:    stringListDefault(ctx, data.AutoGroups, []string{}),
		IsServiceUser: data.IsServiceUser.ValueBool(),
		Role:          data.Role.ValueString(),
		Email:         stringDefaultPointer(data.Email, nil),
		Name:          stringDefaultPointer(data.Name, nil),
	}

	user, err := r.client.Users.Create(ctx, userReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating user", err.Error())
		return
	}

	resp.Diagnostics.Append(userAPIToTerraform(ctx, user, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *User) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	users, err := r.client.Users.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing users", err.Error())
		return
	}
	for _, u := range users {
		if u.Id == data.Id.ValueString() {
			resp.Diagnostics.Append(userAPIToTerraform(ctx, &u, &data)...)

			if resp.Diagnostics.HasError() {
				return
			}

			// Save updated data into Terraform state
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *User) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	user, err := r.client.Users.Update(ctx, data.Id.ValueString(), api.UserRequest{
		AutoGroups: stringListDefault(ctx, data.AutoGroups, []string{}),
		IsBlocked:  data.IsBlocked.ValueBool(),
		Role:       data.Role.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Error updating User", err.Error())
		return
	}

	resp.Diagnostics.Append(userAPIToTerraform(ctx, user, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update user, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *User) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Users.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting User", err.Error())
	}
}

func (r *User) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
