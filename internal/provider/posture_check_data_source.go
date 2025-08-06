// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &PostureCheckDataSource{}

func NewPostureCheckDataSource() datasource.DataSource {
	return &PostureCheckDataSource{}
}

// PostureCheckDataSource defines the data source implementation.
type PostureCheckDataSource struct {
	client *netbird.Client
}

func (d *PostureCheckDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_posture_check"
}

func (d *PostureCheckDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Read Posture Check settings",
		MarkdownDescription: "Read Posture Check settings, see [NetBird Docs](https://docs.netbird.io/how-to/manage-posture-checks) for more information.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "PostureCheck ID",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "PostureCheck Name",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "PostureCheck description",
				Computed:            true,
			},
			"netbird_version_check": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"min_version": schema.StringAttribute{
						Computed: true,
					},
				},
				Computed: true,
			},
			"os_version_check": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"android_min_version": schema.StringAttribute{
						Computed: true,
					},
					"ios_min_version": schema.StringAttribute{
						Computed: true,
					},
					"darwin_min_version": schema.StringAttribute{
						Computed: true,
					},
					"linux_min_kernel_version": schema.StringAttribute{
						Computed: true,
					},
					"windows_min_kernel_version": schema.StringAttribute{
						Computed: true,
					},
				},
				Computed: true,
			},
			"geo_location_check": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"locations": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"country_code": schema.StringAttribute{
									Computed: true,
								},
								"city_name": schema.StringAttribute{
									Computed: true,
								},
							},
						},
						Computed: true,
					},
					"action": schema.StringAttribute{
						Computed: true,
					},
				},
				Computed: true,
			},
			"peer_network_range_check": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"ranges": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
					"action": schema.StringAttribute{
						Computed: true,
					},
				},
				Computed: true,
			},
			"process_check": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"linux_path": schema.StringAttribute{
							Computed: true,
						},
						"mac_path": schema.StringAttribute{
							Computed: true,
						},
						"windows_path": schema.StringAttribute{
							Computed: true,
						},
					},
				},
				Computed: true,
			},
		},
	}
}

func (d *PostureCheckDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*netbird.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *netbird.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *PostureCheckDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PostureCheckModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if knownCount(data.Id, data.Name) == 0 {
		resp.Diagnostics.AddError("No selector", "Must add at least one of (id, name)")
		return
	}

	postureChecks, err := d.client.PostureChecks.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing PostureChecks", err.Error())
		return
	}

	var postureCheck *api.PostureCheck
	for _, p := range postureChecks {
		match := 0
		match += matchString(p.Id, data.Id)
		match += matchString(p.Name, data.Name)
		if match > 0 {
			if postureCheck != nil {
				resp.Diagnostics.AddError("Multiple Matches", "data source cannot match multiple posture checks")
			}
			postureCheck = &p
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	if postureCheck == nil {
		resp.Diagnostics.AddError("No match", "Posture Check matching parameters not found")
		return
	}

	resp.Diagnostics.Append(postureCheckAPIToTerraform(ctx, postureCheck, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
