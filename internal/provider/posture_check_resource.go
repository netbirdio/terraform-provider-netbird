// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &PostureCheck{}
var _ resource.ResourceWithImportState = &PostureCheck{}

func NewPostureCheck() resource.Resource {
	return &PostureCheck{}
}

// PostureCheck defines the resource implementation.
type PostureCheck struct {
	client *netbird.Client
}

// PostureCheckModel describes the resource data model.
type PostureCheckModel struct {
	Id                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Description           types.String `tfsdk:"description"`
	NetbirdVersionCheck   types.Object `tfsdk:"netbird_version_check"`
	OSVersionCheck        types.Object `tfsdk:"os_version_check"`
	GeoLocationCheck      types.Object `tfsdk:"geo_location_check"`
	PeerNetworkRangeCheck types.Object `tfsdk:"peer_network_range_check"`
	ProcessCheck          types.List   `tfsdk:"process_check"`
}

func (r *PostureCheck) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check"
}

func (r *PostureCheck) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Create and Manage Posture Checks",
		MarkdownDescription: "Create and Manage Posture Checks, see [NetBird Docs](https://docs.netbird.io/how-to/manage-posture-checks) for more information.",

		Blocks: map[string]schema.Block{
			"netbird_version_check": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"min_version": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(version.VersionRegexpRaw), "Invalid NetBird Version")},
					},
				},
			},
			"os_version_check": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"android_min_version": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(version.VersionRegexpRaw), "Invalid NetBird Version")},
					},
					"ios_min_version": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(version.VersionRegexpRaw), "Invalid NetBird Version")},
					},
					"darwin_min_version": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(version.VersionRegexpRaw), "Invalid NetBird Version")},
					},
					"linux_min_kernel_version": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(version.VersionRegexpRaw), "Invalid NetBird Version")},
					},
					"windows_min_kernel_version": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(version.VersionRegexpRaw), "Invalid NetBird Version")},
					},
				},
			},
			"geo_location_check": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"locations": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"country_code": schema.StringAttribute{
									Required:   true,
									Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile("^[a-zA-Z]{2}$"), "country code must be 2 letters (ISO 3166-1 alpha-2 format)")},
								},
								"city_name": schema.StringAttribute{
									Optional: true,
								},
							},
						},
						Optional:   true,
						Validators: []validator.List{listvalidator.SizeAtLeast(1)},
					},
					"action": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.OneOf("allow", "deny")},
					},
				},
			},
			"peer_network_range_check": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"ranges": schema.ListAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
					},
					"action": schema.StringAttribute{
						Optional:   true,
						Validators: []validator.String{stringvalidator.OneOf("allow", "deny")},
					},
				},
			},
			"process_check": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"linux_path": schema.StringAttribute{
							Optional: true,
						},
						"mac_path": schema.StringAttribute{
							Optional: true,
						},
						"windows_path": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "PostureCheck ID",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "PostureCheck Name",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "PostureCheck description",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *PostureCheck) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func postureCheckAPIToTerraform(ctx context.Context, postureCheck *api.PostureCheck, data *PostureCheckModel) diag.Diagnostics {
	var ret diag.Diagnostics
	var d diag.Diagnostics
	data.Id = types.StringValue(postureCheck.Id)
	data.Name = types.StringValue(postureCheck.Name)
	if postureCheck.Description != nil {
		data.Description = types.StringValue(*postureCheck.Description)
	} else {
		data.Description = types.StringNull()
	}
	if postureCheck.Checks.NbVersionCheck != nil {
		data.NetbirdVersionCheck, d = types.ObjectValueFrom(
			ctx,
			map[string]attr.Type{"min_version": types.StringType},
			struct {
				MinVersion string `tfsdk:"min_version"`
			}{
				MinVersion: postureCheck.Checks.NbVersionCheck.MinVersion,
			},
		)
		ret.Append(d...)
	} else {
		data.NetbirdVersionCheck = types.ObjectNull(map[string]attr.Type{"min_version": types.StringType})
	}
	if postureCheck.Checks.OsVersionCheck != nil {
		osValues := struct {
			AndroidMinVersion       *string `tfsdk:"android_min_version"`
			IosMinVersion           *string `tfsdk:"ios_min_version"`
			DarwinMinVersion        *string `tfsdk:"darwin_min_version"`
			LinuxMinKernelVersion   *string `tfsdk:"linux_min_kernel_version"`
			WindowsMinKernelVersion *string `tfsdk:"windows_min_kernel_version"`
		}{
			AndroidMinVersion:       nil,
			IosMinVersion:           nil,
			DarwinMinVersion:        nil,
			LinuxMinKernelVersion:   nil,
			WindowsMinKernelVersion: nil,
		}
		if postureCheck.Checks.OsVersionCheck.Android != nil {
			osValues.AndroidMinVersion = &postureCheck.Checks.OsVersionCheck.Android.MinVersion
		}
		if postureCheck.Checks.OsVersionCheck.Ios != nil {
			osValues.IosMinVersion = &postureCheck.Checks.OsVersionCheck.Ios.MinVersion
		}
		if postureCheck.Checks.OsVersionCheck.Darwin != nil {
			osValues.DarwinMinVersion = &postureCheck.Checks.OsVersionCheck.Darwin.MinVersion
		}
		if postureCheck.Checks.OsVersionCheck.Linux != nil {
			osValues.LinuxMinKernelVersion = &postureCheck.Checks.OsVersionCheck.Linux.MinKernelVersion
		}
		if postureCheck.Checks.OsVersionCheck.Windows != nil {
			osValues.WindowsMinKernelVersion = &postureCheck.Checks.OsVersionCheck.Windows.MinKernelVersion
		}
		data.OSVersionCheck, d = types.ObjectValueFrom(ctx, map[string]attr.Type{
			"android_min_version":        types.StringType,
			"ios_min_version":            types.StringType,
			"darwin_min_version":         types.StringType,
			"linux_min_kernel_version":   types.StringType,
			"windows_min_kernel_version": types.StringType,
		}, osValues)
		ret.Append(d...)
	} else {
		data.OSVersionCheck = types.ObjectNull(map[string]attr.Type{
			"android_min_version":        types.StringType,
			"ios_min_version":            types.StringType,
			"darwin_min_version":         types.StringType,
			"linux_min_kernel_version":   types.StringType,
			"windows_min_kernel_version": types.StringType,
		})
	}

	if postureCheck.Checks.GeoLocationCheck != nil {
		geoValues := struct {
			Action    string `tfsdk:"action"`
			Locations []struct {
				CountryCode string  `tfsdk:"country_code"`
				CityName    *string `tfsdk:"city_name"`
			} `tfsdk:"locations"`
		}{
			Action: string(postureCheck.Checks.GeoLocationCheck.Action),
		}
		for _, v := range postureCheck.Checks.GeoLocationCheck.Locations {
			geoValues.Locations = append(geoValues.Locations, struct {
				CountryCode string  "tfsdk:\"country_code\""
				CityName    *string "tfsdk:\"city_name\""
			}{
				CountryCode: v.CountryCode,
				CityName:    v.CityName,
			})
		}
		data.GeoLocationCheck, d = types.ObjectValueFrom(
			ctx,
			map[string]attr.Type{
				"locations": types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						},
					},
				},
				"action": types.StringType,
			},
			geoValues,
		)
		ret.Append(d...)
	} else {
		data.GeoLocationCheck = types.ObjectNull(map[string]attr.Type{
			"locations": types.ListType{
				ElemType: types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"country_code": types.StringType,
						"city_name":    types.StringType,
					},
				},
			},
			"action": types.StringType,
		})
	}

	if postureCheck.Checks.PeerNetworkRangeCheck != nil {
		data.PeerNetworkRangeCheck, d = types.ObjectValueFrom(
			ctx,
			map[string]attr.Type{
				"ranges": types.ListType{ElemType: types.StringType},
				"action": types.StringType,
			},
			struct {
				Ranges []string `tfsdk:"ranges"`
				Action string   `tfsdk:"action"`
			}{
				Ranges: postureCheck.Checks.PeerNetworkRangeCheck.Ranges,
				Action: string(postureCheck.Checks.PeerNetworkRangeCheck.Action),
			},
		)
		ret.Append(d...)
	} else {
		data.PeerNetworkRangeCheck = types.ObjectNull(map[string]attr.Type{
			"ranges": types.ListType{ElemType: types.StringType},
			"action": types.StringType,
		})
	}

	if postureCheck.Checks.ProcessCheck != nil {
		var processData []struct {
			LinuxPath   *string `tfsdk:"linux_path"`
			MacPath     *string `tfsdk:"mac_path"`
			WindowsPath *string `tfsdk:"windows_path"`
		}
		for _, v := range postureCheck.Checks.ProcessCheck.Processes {
			i := struct {
				LinuxPath   *string "tfsdk:\"linux_path\""
				MacPath     *string "tfsdk:\"mac_path\""
				WindowsPath *string "tfsdk:\"windows_path\""
			}{
				LinuxPath:   v.LinuxPath,
				MacPath:     v.MacPath,
				WindowsPath: v.WindowsPath,
			}
			if i.LinuxPath != nil && *i.LinuxPath == "" {
				i.LinuxPath = nil
			}
			if i.MacPath != nil && *i.MacPath == "" {
				i.MacPath = nil
			}
			if i.WindowsPath != nil && *i.WindowsPath == "" {
				i.WindowsPath = nil
			}
			processData = append(processData, i)
		}
		data.ProcessCheck, d = types.ListValueFrom(
			ctx,
			types.ObjectType{
				AttrTypes: map[string]attr.Type{
					"linux_path":   types.StringType,
					"mac_path":     types.StringType,
					"windows_path": types.StringType,
				},
			},
			processData,
		)
		ret.Append(d...)
	} else {
		data.ProcessCheck = types.ListNull(types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"linux_path":   types.StringType,
				"mac_path":     types.StringType,
				"windows_path": types.StringType,
			},
		})
	}
	return ret
}

func postureCheckTerraformToAPI(ctx context.Context, data PostureCheckModel) (api.PostureCheckUpdate, diag.Diagnostics) {
	var ret diag.Diagnostics
	postureCheckReq := api.PostureCheckUpdate{
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
	}

	postureCheckReq.Checks = &api.Checks{}
	if !data.GeoLocationCheck.IsNull() && !data.GeoLocationCheck.IsUnknown() {
		geoLocationAction, ok := data.GeoLocationCheck.Attributes()["action"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.geo_location_check.action expected to be types.String, found %T", data.GeoLocationCheck.Attributes()["action"]))
			return postureCheckReq, ret
		}
		postureCheckReq.Checks.GeoLocationCheck = &api.GeoLocationCheck{
			Action: api.GeoLocationCheckAction(geoLocationAction.ValueString()),
		}
		geoLocations, ok := data.GeoLocationCheck.Attributes()["locations"].(types.List)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.geo_location_check.locations expected to be types.List, found %T", data.GeoLocationCheck.Attributes()["locations"]))
			return postureCheckReq, ret
		}
		for i, v := range geoLocations.Elements() {
			vObj, ok := v.(types.Object)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.geo_location_check.locations[%d] expected to be types.Object, found %T", i, v))
				return postureCheckReq, ret
			}
			vCountryCode, ok := vObj.Attributes()["country_code"].(types.String)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.geo_location_check.locations[%d].country_code expected to be types.String, found %T", i, vObj.Attributes()["country_code"]))
				return postureCheckReq, ret
			}
			vCityName, ok := vObj.Attributes()["city_name"].(types.String)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.geo_location_check.locations[%d].city_name expected to be types.String, found %T", i, vObj.Attributes()["city_name"]))
				return postureCheckReq, ret
			}
			postureCheckReq.Checks.GeoLocationCheck.Locations = append(postureCheckReq.Checks.GeoLocationCheck.Locations, api.Location{
				CountryCode: vCountryCode.ValueString(),
				CityName:    vCityName.ValueStringPointer(),
			})
		}
	}

	if !data.NetbirdVersionCheck.IsNull() && !data.NetbirdVersionCheck.IsUnknown() {
		minVersion, ok := data.NetbirdVersionCheck.Attributes()["min_version"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.netbird_version_check.min_version expected to be types.String, found %T", data.NetbirdVersionCheck.Attributes()["min_version"]))
			return postureCheckReq, ret
		}
		postureCheckReq.Checks.NbVersionCheck = &api.MinVersionCheck{
			MinVersion: minVersion.ValueString(),
		}
	}

	if !data.OSVersionCheck.IsNull() && !data.OSVersionCheck.IsUnknown() {
		postureCheckReq.Checks.OsVersionCheck = &api.OSVersionCheck{}
		androidMinVersion, ok := data.OSVersionCheck.Attributes()["android_min_version"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.os_version_check.android_min_version expected to be types.String, found %T", data.OSVersionCheck.Attributes()["android_min_version"]))
			return postureCheckReq, ret
		}
		iosMinVersion, ok := data.OSVersionCheck.Attributes()["ios_min_version"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.os_version_check.ios_min_version expected to be types.String, found %T", data.OSVersionCheck.Attributes()["ios_min_version"]))
			return postureCheckReq, ret
		}
		darwinMinVersion, ok := data.OSVersionCheck.Attributes()["darwin_min_version"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.os_version_check.darwin_min_version expected to be types.String, found %T", data.OSVersionCheck.Attributes()["darwin_min_version"]))
			return postureCheckReq, ret
		}
		linuxMinKernelVersion, ok := data.OSVersionCheck.Attributes()["linux_min_kernel_version"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.os_version_check.linux_min_kernel_version expected to be types.String, found %T", data.OSVersionCheck.Attributes()["linux_min_kernel_version"]))
			return postureCheckReq, ret
		}
		windowsMinKernelVersion, ok := data.OSVersionCheck.Attributes()["windows_min_kernel_version"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.os_version_check.windows_min_kernel_version expected to be types.String, found %T", data.OSVersionCheck.Attributes()["windows_min_kernel_version"]))
			return postureCheckReq, ret
		}
		if !androidMinVersion.IsNull() && !androidMinVersion.IsUnknown() {
			postureCheckReq.Checks.OsVersionCheck.Android = &api.MinVersionCheck{
				MinVersion: androidMinVersion.ValueString(),
			}
		}
		if !iosMinVersion.IsNull() && !iosMinVersion.IsUnknown() {
			postureCheckReq.Checks.OsVersionCheck.Ios = &api.MinVersionCheck{
				MinVersion: iosMinVersion.ValueString(),
			}
		}
		if !darwinMinVersion.IsNull() && !darwinMinVersion.IsUnknown() {
			postureCheckReq.Checks.OsVersionCheck.Darwin = &api.MinVersionCheck{
				MinVersion: darwinMinVersion.ValueString(),
			}
		}
		if !linuxMinKernelVersion.IsNull() && !linuxMinKernelVersion.IsUnknown() {
			postureCheckReq.Checks.OsVersionCheck.Linux = &api.MinKernelVersionCheck{
				MinKernelVersion: linuxMinKernelVersion.ValueString(),
			}
		}
		if !windowsMinKernelVersion.IsNull() && !windowsMinKernelVersion.IsUnknown() {
			postureCheckReq.Checks.OsVersionCheck.Windows = &api.MinKernelVersionCheck{
				MinKernelVersion: windowsMinKernelVersion.ValueString(),
			}
		}
	}

	if !data.PeerNetworkRangeCheck.IsNull() && !data.PeerNetworkRangeCheck.IsUnknown() {
		action, ok := data.PeerNetworkRangeCheck.Attributes()["action"].(types.String)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.peer_network_range_check.action expected to be types.String, found %T", data.PeerNetworkRangeCheck.Attributes()["action"]))
			return postureCheckReq, ret
		}
		postureCheckReq.Checks.PeerNetworkRangeCheck = &api.PeerNetworkRangeCheck{
			Action: api.PeerNetworkRangeCheckAction(action.ValueString()),
		}
		ranges, ok := data.PeerNetworkRangeCheck.Attributes()["ranges"].(types.List)
		if !ok {
			ret.AddError("Unexpected Value", fmt.Sprintf("data.peer_network_range_check.ranges expected to be types.List, found %T", data.PeerNetworkRangeCheck.Attributes()["ranges"]))
			return postureCheckReq, ret
		}
		d := ranges.ElementsAs(ctx, &postureCheckReq.Checks.PeerNetworkRangeCheck.Ranges, false)
		ret.Append(d...)
	}

	if !data.ProcessCheck.IsNull() && !data.ProcessCheck.IsUnknown() {
		postureCheckReq.Checks.ProcessCheck = &api.ProcessCheck{}
		for i, v := range data.ProcessCheck.Elements() {
			vObj, ok := v.(types.Object)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.process_check[%d] expected to be types.Object, found %T", i, v))
				return postureCheckReq, ret
			}
			vLinuxPath, ok := vObj.Attributes()["linux_path"].(types.String)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.process_check[%d].linux_path expected to be types.String, found %T", i, vObj.Attributes()["linux_path"]))
				return postureCheckReq, ret
			}
			vMacPath, ok := vObj.Attributes()["mac_path"].(types.String)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.process_check[%d].mac_path expected to be types.String, found %T", i, vObj.Attributes()["mac_path"]))
				return postureCheckReq, ret
			}
			vWindowsPath, ok := vObj.Attributes()["windows_path"].(types.String)
			if !ok {
				ret.AddError("Unexpected Value", fmt.Sprintf("data.process_check[%d].windows_path expected to be types.String, found %T", i, vObj.Attributes()["windows_path"]))
				return postureCheckReq, ret
			}
			postureCheckReq.Checks.ProcessCheck.Processes = append(postureCheckReq.Checks.ProcessCheck.Processes, api.Process{
				LinuxPath:   vLinuxPath.ValueStringPointer(),
				MacPath:     vMacPath.ValueStringPointer(),
				WindowsPath: vWindowsPath.ValueStringPointer(),
			})
		}
	}

	return postureCheckReq, ret
}

func (r *PostureCheck) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PostureCheckModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	postureCheckReq, d := postureCheckTerraformToAPI(ctx, data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	postureCheck, err := r.client.PostureChecks.Create(ctx, postureCheckReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating postureCheck", err.Error())
		return
	}

	resp.Diagnostics.Append(postureCheckAPIToTerraform(ctx, postureCheck, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PostureCheck) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PostureCheckModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	postureCheck, err := r.client.PostureChecks.Get(ctx, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		} else {
			resp.Diagnostics.AddError("Error getting PostureCheck", err.Error())
		}
		return
	}

	resp.Diagnostics.Append(postureCheckAPIToTerraform(ctx, postureCheck, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PostureCheck) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PostureCheckModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.ValueString() == "" {
		r.Create(ctx, resource.CreateRequest{Config: req.Config, Plan: req.Plan, ProviderMeta: req.Config}, (*resource.CreateResponse)(resp))
		return
	}

	postureCheckReq, d := postureCheckTerraformToAPI(ctx, data)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	postureCheck, err := r.client.PostureChecks.Update(ctx, data.Id.ValueString(), postureCheckReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating PostureCheck", err.Error())
		return
	}

	resp.Diagnostics.Append(postureCheckAPIToTerraform(ctx, postureCheck, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PostureCheck) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PostureCheckModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.PostureChecks.Delete(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting PostureCheck", err.Error())
	}
}

func (r *PostureCheck) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
