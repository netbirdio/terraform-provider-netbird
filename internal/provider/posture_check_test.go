package provider

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_postureCheckAPIToTerraform(t *testing.T) {
	cases := []struct {
		resource *api.PostureCheck
		expected PostureCheckModel
	}{
		{
			resource: &api.PostureCheck{
				Id:          "pc1",
				Description: valPtr("PC"),
				Name:        "PC",
				Checks: api.Checks{
					GeoLocationCheck: &api.GeoLocationCheck{
						Action: api.GeoLocationCheckActionAllow,
						Locations: []api.Location{
							{
								CityName:    valPtr(api.CityName("Cairo")),
								CountryCode: api.CountryCode("EG"),
							},
						},
					},
					NbVersionCheck: &api.MinVersionCheck{
						MinVersion: "0.40.0",
					},
					OsVersionCheck: &api.OSVersionCheck{
						Android: &api.MinVersionCheck{
							MinVersion: "0.0.1",
						},
						Darwin: &api.MinVersionCheck{
							MinVersion: "0.0.2",
						},
						Ios: &api.MinVersionCheck{
							MinVersion: "0.0.3",
						},
						Linux: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.4",
						},
						Windows: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.5",
						},
					},
					PeerNetworkRangeCheck: &api.PeerNetworkRangeCheck{
						Action: api.PeerNetworkRangeCheckActionAllow,
						Ranges: []string{"1.1.1.1/24"},
					},
					ProcessCheck: &api.ProcessCheck{
						Processes: []api.Process{
							{
								LinuxPath:   valPtr("/linux"),
								MacPath:     valPtr("/mac"),
								WindowsPath: valPtr("C:\\windows"),
							},
						},
					},
					CertificateCheck: &api.CertificateCheck{
						CaCertificates: []string{"ca-pem-1", "ca-pem-2"},
					},
				},
			},
			expected: PostureCheckModel{
				Id:                  types.StringValue("pc1"),
				Name:                types.StringValue("PC"),
				Description:         types.StringValue("PC"),
				NetbirdVersionCheck: types.ObjectValueMust(map[string]attr.Type{"min_version": types.StringType}, map[string]attr.Value{"min_version": types.StringValue("0.40.0")}),
				OSVersionCheck: types.ObjectValueMust(map[string]attr.Type{
					"android_min_version":        types.StringType,
					"ios_min_version":            types.StringType,
					"darwin_min_version":         types.StringType,
					"linux_min_kernel_version":   types.StringType,
					"windows_min_kernel_version": types.StringType,
				}, map[string]attr.Value{
					"android_min_version":        types.StringValue("0.0.1"),
					"darwin_min_version":         types.StringValue("0.0.2"),
					"ios_min_version":            types.StringValue("0.0.3"),
					"linux_min_kernel_version":   types.StringValue("0.0.4"),
					"windows_min_kernel_version": types.StringValue("0.0.5"),
				}),
				GeoLocationCheck: types.ObjectValueMust(map[string]attr.Type{
					"locations": types.ListType{
						ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"country_code": types.StringType,
								"city_name":    types.StringType,
							},
						},
					},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"locations": types.ListValueMust(types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						},
					}, []attr.Value{
						types.ObjectValueMust(map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						}, map[string]attr.Value{
							"country_code": types.StringValue("EG"),
							"city_name":    types.StringValue("Cairo"),
						}),
					}),
				}),
				PeerNetworkRangeCheck: types.ObjectValueMust(map[string]attr.Type{
					"ranges": types.ListType{ElemType: types.StringType},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"ranges": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("1.1.1.1/24")}),
				}),
				ProcessCheck: types.ListValueMust(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					},
				}, []attr.Value{
					types.ObjectValueMust(map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					}, map[string]attr.Value{
						"linux_path":   types.StringValue("/linux"),
						"mac_path":     types.StringValue("/mac"),
						"windows_path": types.StringValue("C:\\windows"),
					}),
				}),
				CertificateCheck: types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{
					"ca_certificates": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ca-pem-1"), types.StringValue("ca-pem-2")}),
				}),
			},
		},
	}

	for _, c := range cases {
		var out PostureCheckModel
		outDiag := postureCheckAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_postureCheckTerraformToAPI(t *testing.T) {
	cases := []struct {
		resource PostureCheckModel
		expected api.PostureCheckUpdate
	}{
		{
			resource: PostureCheckModel{
				Id:                  types.StringValue("pc1"),
				Name:                types.StringValue("PC"),
				Description:         types.StringValue("desc"),
				NetbirdVersionCheck: types.ObjectValueMust(map[string]attr.Type{"min_version": types.StringType}, map[string]attr.Value{"min_version": types.StringValue("0.40.0")}),
				OSVersionCheck: types.ObjectValueMust(map[string]attr.Type{
					"android_min_version":        types.StringType,
					"ios_min_version":            types.StringType,
					"darwin_min_version":         types.StringType,
					"linux_min_kernel_version":   types.StringType,
					"windows_min_kernel_version": types.StringType,
				}, map[string]attr.Value{
					"android_min_version":        types.StringValue("0.0.1"),
					"darwin_min_version":         types.StringValue("0.0.2"),
					"ios_min_version":            types.StringValue("0.0.3"),
					"linux_min_kernel_version":   types.StringValue("0.0.4"),
					"windows_min_kernel_version": types.StringValue("0.0.5"),
				}),
				GeoLocationCheck: types.ObjectValueMust(map[string]attr.Type{
					"locations": types.ListType{
						ElemType: types.ObjectType{
							AttrTypes: map[string]attr.Type{
								"country_code": types.StringType,
								"city_name":    types.StringType,
							},
						},
					},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"locations": types.ListValueMust(types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						},
					}, []attr.Value{
						types.ObjectValueMust(map[string]attr.Type{
							"country_code": types.StringType,
							"city_name":    types.StringType,
						}, map[string]attr.Value{
							"country_code": types.StringValue("EG"),
							"city_name":    types.StringValue("Cairo"),
						}),
					}),
				}),
				PeerNetworkRangeCheck: types.ObjectValueMust(map[string]attr.Type{
					"ranges": types.ListType{ElemType: types.StringType},
					"action": types.StringType,
				}, map[string]attr.Value{
					"action": types.StringValue("allow"),
					"ranges": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("1.1.1.1/24")}),
				}),
				ProcessCheck: types.ListValueMust(types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					},
				}, []attr.Value{
					types.ObjectValueMust(map[string]attr.Type{
						"linux_path":   types.StringType,
						"mac_path":     types.StringType,
						"windows_path": types.StringType,
					}, map[string]attr.Value{
						"linux_path":   types.StringValue("/linux"),
						"mac_path":     types.StringValue("/mac"),
						"windows_path": types.StringValue("C:\\windows"),
					}),
				}),
				CertificateCheck: types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{
					"ca_certificates": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("ca-pem-1"), types.StringValue("ca-pem-2")}),
				}),
			},
			expected: api.PostureCheckUpdate{
				Name:        "PC",
				Description: "desc",
				Checks: &api.Checks{
					GeoLocationCheck: &api.GeoLocationCheck{
						Action: api.GeoLocationCheckActionAllow,
						Locations: []api.Location{
							{
								CityName:    valPtr(api.CityName("Cairo")),
								CountryCode: api.CountryCode("EG"),
							},
						},
					},
					NbVersionCheck: &api.MinVersionCheck{
						MinVersion: "0.40.0",
					},
					OsVersionCheck: &api.OSVersionCheck{
						Android: &api.MinVersionCheck{
							MinVersion: "0.0.1",
						},
						Darwin: &api.MinVersionCheck{
							MinVersion: "0.0.2",
						},
						Ios: &api.MinVersionCheck{
							MinVersion: "0.0.3",
						},
						Linux: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.4",
						},
						Windows: &api.MinKernelVersionCheck{
							MinKernelVersion: "0.0.5",
						},
					},
					PeerNetworkRangeCheck: &api.PeerNetworkRangeCheck{
						Action: api.PeerNetworkRangeCheckActionAllow,
						Ranges: []string{"1.1.1.1/24"},
					},
					ProcessCheck: &api.ProcessCheck{
						Processes: []api.Process{
							{
								LinuxPath:   valPtr("/linux"),
								MacPath:     valPtr("/mac"),
								WindowsPath: valPtr("C:\\windows"),
							},
						},
					},
					CertificateCheck: &api.CertificateCheck{
						CaCertificates: []string{"ca-pem-1", "ca-pem-2"},
					},
				},
			},
		},
	}
	for _, c := range cases {
		out, outDiag := postureCheckTerraformToAPI(context.Background(), c.resource)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

// testCAPEM returns a freshly generated, self-signed CA certificate in PEM form,
// so tests never depend on a checked-in fixture. The trailing newline matches
// what file() returns for a PEM file on disk.
func testCAPEM(t *testing.T, commonName string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		t.Fatalf("generate CA serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func Test_postureCheckAPIToTerraform_CertificateCheck(t *testing.T) {
	ctx := context.Background()
	caList := func(values ...string) types.List {
		elems := make([]attr.Value, 0, len(values))
		for _, v := range values {
			elems = append(elems, types.StringValue(v))
		}
		return types.ListValueMust(types.StringType, elems)
	}
	certificateCheck := func(values ...string) types.Object {
		return types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{"ca_certificates": caList(values...)})
	}

	cases := []struct {
		name     string
		prior    types.Object
		api      *api.CertificateCheck
		expected types.Object
	}{
		{
			name:     "absent check is null",
			prior:    types.ObjectNull(certificateCheckAttrTypes),
			api:      nil,
			expected: types.ObjectNull(certificateCheckAttrTypes),
		},
		{
			name:     "removed out of band is null",
			prior:    certificateCheck("a"),
			api:      nil,
			expected: types.ObjectNull(certificateCheckAttrTypes),
		},
		{
			name:     "no prior order is sorted",
			prior:    types.ObjectNull(certificateCheckAttrTypes),
			api:      &api.CertificateCheck{CaCertificates: []string{"b", "a"}},
			expected: certificateCheck("a", "b"),
		},
		{
			name:     "configured order is kept",
			prior:    certificateCheck("b", "a"),
			api:      &api.CertificateCheck{CaCertificates: []string{"a", "b"}},
			expected: certificateCheck("b", "a"),
		},
		{
			name:     "PEM is kept byte for byte",
			prior:    types.ObjectNull(certificateCheckAttrTypes),
			api:      &api.CertificateCheck{CaCertificates: []string{"-----BEGIN CERTIFICATE-----\nX\n-----END CERTIFICATE-----\n"}},
			expected: certificateCheck("-----BEGIN CERTIFICATE-----\nX\n-----END CERTIFICATE-----\n"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := PostureCheckModel{CertificateCheck: c.prior}
			diags := postureCheckAPIToTerraform(ctx, &api.PostureCheck{Id: "pc", Name: "pc", Checks: api.Checks{CertificateCheck: c.api}}, &data)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if !data.CertificateCheck.Equal(c.expected) {
				t.Fatalf("expected %s, found %s", c.expected, data.CertificateCheck)
			}
		})
	}
}

func Test_postureCheckTerraformToAPI_CertificateCheck(t *testing.T) {
	ctx := context.Background()

	out, diags := postureCheckTerraformToAPI(ctx, PostureCheckModel{
		Name:             types.StringValue("pc"),
		CertificateCheck: types.ObjectNull(certificateCheckAttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if out.Checks.CertificateCheck != nil {
		t.Fatalf("expected no certificate check for a null block, found %#v", out.Checks.CertificateCheck)
	}

	out, diags = postureCheckTerraformToAPI(ctx, PostureCheckModel{
		Name: types.StringValue("pc"),
		CertificateCheck: types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{
			"ca_certificates": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("b"), types.StringValue("a")}),
		}),
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if out.Checks.CertificateCheck == nil || !reflect.DeepEqual(out.Checks.CertificateCheck.CaCertificates, []string{"b", "a"}) {
		t.Fatalf("expected the configured CAs in order, found %#v", out.Checks.CertificateCheck)
	}
}

func Test_validateCertificateCheck(t *testing.T) {
	ca := testCAPEM(t, "unit test CA")
	list := func(values ...attr.Value) types.Object {
		return types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{
			"ca_certificates": types.ListValueMust(types.StringType, values),
		})
	}

	cases := []struct {
		name      string
		value     types.Object
		errorPath path.Path
		errorText string
	}{
		{name: "absent block", value: types.ObjectNull(certificateCheckAttrTypes)},
		{name: "unknown block", value: types.ObjectUnknown(certificateCheckAttrTypes)},
		{name: "valid CA", value: list(types.StringValue(ca))},
		{name: "two CAs in one entry", value: list(types.StringValue(ca + testCAPEM(t, "second CA")))},
		{name: "unknown list", value: types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{
			"ca_certificates": types.ListUnknown(types.StringType),
		})},
		{name: "unknown entry from another resource", value: list(types.StringValue(ca), types.StringUnknown())},
		{
			name: "block without ca_certificates",
			value: types.ObjectValueMust(certificateCheckAttrTypes, map[string]attr.Value{
				"ca_certificates": types.ListNull(types.StringType),
			}),
			errorPath: path.Root("certificate_check").AtName("ca_certificates"),
			errorText: "at least one entry",
		},
		{
			name:      "not PEM",
			value:     list(types.StringValue(ca), types.StringValue("not a certificate")),
			errorPath: path.Root("certificate_check").AtName("ca_certificates").AtListIndex(1),
			errorText: "ca_certificates[1]",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := validateCertificateCheck(c.value)
			if c.errorText == "" {
				if diags.HasError() {
					t.Fatalf("expected no error, found %v", diags)
				}
				return
			}
			if diags.ErrorsCount() != 1 {
				t.Fatalf("expected one error, found %v", diags)
			}
			errDiag := diags.Errors()[0]
			withPath, ok := errDiag.(interface{ Path() path.Path })
			if !ok || !withPath.Path().Equal(c.errorPath) {
				t.Fatalf("expected the error at %s, found %v", c.errorPath, errDiag)
			}
			if !strings.Contains(errDiag.Detail(), c.errorText) {
				t.Fatalf("expected %q in the error, found %q", c.errorText, errDiag.Detail())
			}
		})
	}
}

// The API returns an unset description as "", while a configuration that omits
// it plans null; mapping "" to "" made every such check fail its first apply.
func Test_postureCheckAPIToTerraform_Description(t *testing.T) {
	cases := []struct {
		name     string
		prior    types.String
		api      *string
		expected types.String
	}{
		{name: "omitted and empty on the server", prior: types.StringNull(), api: valPtr(""), expected: types.StringNull()},
		{name: "omitted and absent on the server", prior: types.StringNull(), api: nil, expected: types.StringNull()},
		{name: "configured as empty", prior: types.StringValue(""), api: valPtr(""), expected: types.StringValue("")},
		{name: "cleared out of band", prior: types.StringValue("set"), api: valPtr(""), expected: types.StringValue("")},
		{name: "set on the server", prior: types.StringNull(), api: valPtr("set"), expected: types.StringValue("set")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := PostureCheckModel{Description: c.prior}
			diags := postureCheckAPIToTerraform(context.Background(), &api.PostureCheck{Id: "pc", Name: "pc", Description: c.api}, &data)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if !data.Description.Equal(c.expected) {
				t.Fatalf("expected %s, found %s", c.expected, data.Description)
			}
		})
	}
}
