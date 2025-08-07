package provider

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

func Test_peerAPIToTerraform(t *testing.T) {
	timeNow := time.Now()

	cases := []struct {
		resource *api.Peer
		expected PeerModel
	}{
		{
			resource: &api.Peer{
				CityName:       "Cairo",
				ConnectionIp:   "1.2.3.4",
				CountryCode:    "EG",
				ExtraDnsLabels: []string{},
				GeonameId:      1234,
				Hostname:       "ip-1-2-3-4",
				Groups:         []api.GroupMinimum{},
				Id:             "p1",
				Ip:             "100.1.2.3",
				KernelVersion:  "6.8.0",
				LastLogin:      timeNow,
				LastSeen:       timeNow,
				Name:           "ip-1-2-3-4",
				Os:             "Ubuntu 22.04",
				SerialNumber:   "1234",
				UserId:         "12345-abc",
				Version:        "0.41.0",
			},
			expected: PeerModel{
				Id:                          types.StringValue("p1"),
				Name:                        types.StringValue("ip-1-2-3-4"),
				Ip:                          types.StringValue("100.1.2.3"),
				ConnectionIp:                types.StringValue("1.2.3.4"),
				Connected:                   types.BoolValue(false),
				LastSeen:                    types.StringValue(timeNow.Format(time.RFC3339)),
				Os:                          types.StringValue("Ubuntu 22.04"),
				KernelVersion:               types.StringValue("6.8.0"),
				GeonameId:                   types.Int32Value(1234),
				Version:                     types.StringValue("0.41.0"),
				Groups:                      types.ListValueMust(types.StringType, []attr.Value{}),
				SshEnabled:                  types.BoolValue(false),
				InactivityExpirationEnabled: types.BoolValue(false),
				ApprovalRequired:            types.BoolValue(false),
				DnsLabel:                    types.StringValue(""),
				UserId:                      types.StringValue("12345-abc"),
				Hostname:                    types.StringValue("ip-1-2-3-4"),
				UiVersion:                   types.StringValue(""),
				LoginExpirationEnabled:      types.BoolValue(false),
				LoginExpired:                types.BoolValue(false),
				LastLogin:                   types.StringValue(timeNow.Format(time.RFC3339)),
				CountryCode:                 types.StringValue("EG"),
				CityName:                    types.StringValue("Cairo"),
				SerialNumber:                types.StringValue("1234"),
				ExtraDnsLabels:              types.ListValueMust(types.StringType, []attr.Value{}),
			},
		},
		{
			resource: &api.Peer{
				CityName:                    "Berlin",
				ConnectionIp:                "1.2.3.5",
				CountryCode:                 "DE",
				ExtraDnsLabels:              []string{"test.example.local"},
				GeonameId:                   1234,
				Hostname:                    "ip-1-2-3-5",
				Groups:                      []api.GroupMinimum{{Id: "g1"}, {Id: "g2"}},
				Id:                          "p2",
				Ip:                          "100.1.2.4",
				KernelVersion:               "6.8.0",
				LastLogin:                   timeNow,
				LastSeen:                    timeNow,
				Name:                        "ip-1-2-3-5",
				Os:                          "Ubuntu 22.04",
				SerialNumber:                "1234",
				UserId:                      "12345-abc",
				Version:                     "0.41.0",
				ApprovalRequired:            true,
				Connected:                   true,
				DnsLabel:                    "test",
				InactivityExpirationEnabled: true,
				LoginExpirationEnabled:      true,
				LoginExpired:                true,
				SshEnabled:                  true,
				UiVersion:                   "0.41.0",
			},
			expected: PeerModel{
				Id:                          types.StringValue("p2"),
				Name:                        types.StringValue("ip-1-2-3-5"),
				Ip:                          types.StringValue("100.1.2.4"),
				ConnectionIp:                types.StringValue("1.2.3.5"),
				Connected:                   types.BoolValue(true),
				LastSeen:                    types.StringValue(timeNow.Format(time.RFC3339)),
				Os:                          types.StringValue("Ubuntu 22.04"),
				KernelVersion:               types.StringValue("6.8.0"),
				GeonameId:                   types.Int32Value(1234),
				Version:                     types.StringValue("0.41.0"),
				Groups:                      types.ListValueMust(types.StringType, []attr.Value{types.StringValue("g1"), types.StringValue("g2")}),
				SshEnabled:                  types.BoolValue(true),
				InactivityExpirationEnabled: types.BoolValue(true),
				ApprovalRequired:            types.BoolValue(true),
				DnsLabel:                    types.StringValue("test"),
				UserId:                      types.StringValue("12345-abc"),
				Hostname:                    types.StringValue("ip-1-2-3-5"),
				UiVersion:                   types.StringValue("0.41.0"),
				LoginExpirationEnabled:      types.BoolValue(true),
				LoginExpired:                types.BoolValue(true),
				LastLogin:                   types.StringValue(timeNow.Format(time.RFC3339)),
				CountryCode:                 types.StringValue("DE"),
				CityName:                    types.StringValue("Berlin"),
				SerialNumber:                types.StringValue("1234"),
				ExtraDnsLabels:              types.ListValueMust(types.StringType, []attr.Value{types.StringValue("test.example.local")}),
			},
		},
	}

	for _, c := range cases {
		var out PeerModel
		outDiag := peerAPIToTerraform(context.Background(), c.resource, &out)
		if outDiag.HasError() {
			t.Fatalf("Expected no error diagnostics, found %d errors", outDiag.ErrorsCount())
		}

		if !reflect.DeepEqual(out, c.expected) {
			t.Fatalf("Expected:\n%#v\nFound:\n%#v", c.expected, out)
		}
	}
}

func Test_Peer_Create(t *testing.T) {
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_peer." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPeerResource(rName, `peer2`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						peer, err := testClient().Peers.Get(context.Background(), pID)
						if err != nil {
							return err
						}

						if peer.Name != rName {
							return fmt.Errorf("Peer name mismatch, expected %s, found %s on management server", rName, peer.Name)
						}

						return nil
					},
				),
			},
		},
	})
}

func Test_Peer_Update(t *testing.T) {
	rName := "p" + acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	rNameFull := "netbird_peer." + rName
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testEnsureManagementRunning(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName: rName,
				Config:       testPeerResource(rName, `peer3`, "meow"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
				),
			},
			{
				ResourceName: rName,
				Config:       testPeerResource(rName, `peer3`, rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(rNameFull, "id"),
					resource.TestCheckResourceAttr(rNameFull, "name", rName),
					func(s *terraform.State) error {
						pID := s.RootModule().Resources[rNameFull].Primary.Attributes["id"]
						peer, err := testClient().Peers.Get(context.Background(), pID)
						if err != nil {
							return err
						}
						if peer.Name != rName {
							return fmt.Errorf("Peer name mismatch, expected %s, found %s on management server", rName, peer.Name)
						}
						return nil
					},
				),
			},
		},
	})
}

func testPeerResource(rName, id, name string) string {
	return fmt.Sprintf(`resource "netbird_peer" "%s" {
	id = "%s"
	name = "%s"
}`, rName, id, name)
}
