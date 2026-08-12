// Copyright (c) HashiCorp, Inc.

//go:build e2e

// The acceptance tests need a live NetBird deployment, and this file is what
// brings one up. It is compiled only under the `e2e` build tag, and so is every
// test that calls into it — each resource's acceptance tests live in its own
// <resource>_acc_test.go alongside this one.
//
// That means a plain `go test ./...` compiles none of it: no deployment is
// started, nothing shells out to Docker, and the only tests in the binary are the
// ones that can answer without a server. The two suites are:
//
//	go test ./...                            # conversion and matching functions
//	TF_ACC=1 go test -tags e2e ./...         # everything above, against a deployment
//
// TF_ACC is terraform-plugin-testing's own switch: without it resource.Test
// skips, so the tag alone starts nothing.

package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

const apiToken = "nbp_apTmlmUXHSC4PKmHwtIZNaGr8eqcVI2gMURp"

const managementURL = "http://127.0.0.1:8080"

// GetProjectDir will return the directory where the project is.
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.ReplaceAll(wd, "/internal/provider", "")
	return wd, nil
}

func testEnsureManagementRunning(t *testing.T) {
	_, err := testClient().Accounts.List(context.Background())
	if err == nil {
		t.Log("Management API Up")
		t.Setenv("NB_PAT", apiToken)
		t.Setenv("NB_MANAGEMENT_URL", managementURL)
		return
	}

	cmd := exec.Command("docker", "compose", "up", "-d")
	curDir, err := GetProjectDir()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Dir = path.Join(curDir, "test")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Log(string(out))
		t.Fatal(err)
	}

	// The counter has to advance, or the loop never ends and the Fatal below is
	// unreachable: a management server that comes up but never serves would hang
	// the run rather than failing it.
	backoff := 1 * time.Second
	for attempts := 0; attempts < 5; attempts++ {
		_, err = testClient().Accounts.List(context.Background())
		if err == nil {
			t.Log("Management API Up")
			t.Setenv("NB_PAT", apiToken)
			t.Setenv("NB_MANAGEMENT_URL", managementURL)
			return
		}

		time.Sleep(backoff)

		backoff *= 2
	}

	t.Fatalf("Management Server not started; last error: %v", err)
}

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"netbird": providerserver.NewProtocol6WithError(New("test")()),
}

func testClient() *netbird.Client {
	return netbird.New(managementURL, apiToken)
}

func matchPairs(pairs map[string][]any) error {
	for name, p := range pairs {
		if reflect.ValueOf(p[1]).Kind() == reflect.Pointer {
			if reflect.ValueOf(p[0]).Kind() == reflect.Pointer {
				if reflect.ValueOf(p[1]).IsNil() != reflect.ValueOf(p[0]).IsNil() {
					return fmt.Errorf("Mistmatch, expected field %s to be %v, found nil", name, p[0])
				}
				if reflect.ValueOf(p[0]).IsNil() {
					continue
				}
			}
			if !reflect.ValueOf(p[0]).IsValid() && reflect.ValueOf(p[1]).IsNil() {
				continue
			} else if !reflect.ValueOf(p[0]).IsValid() {
				return fmt.Errorf("Mismatch, expected field %s to be nil, found %v", name, reflect.ValueOf(p[1]).Elem().Interface())
			}
			if v := reflect.ValueOf(p[1]).Elem().Interface(); v != p[0] {
				return fmt.Errorf("Mismatch, expected field %s to be %v, found %v", name, p[0], v)
			}
		} else if p[0] != p[1] {
			return fmt.Errorf("Mismatch, expected field %s to be %v, found %v", name, p[0], p[1])
		}
	}
	return nil
}

// sameGroupIDs reports whether got holds exactly the wanted group IDs, in any
// order and with duplicates counted.
//
// Comparing the set matters: the assertions that call this used to be written as
// `len(got) != 2 || (A && B)`, and && binds tighter than ||, so a single wrong ID
// left the other one correct, made the && false, and passed.
func sameGroupIDs(got []api.GroupMinimum, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	remaining := make(map[string]int, len(want))
	for _, w := range want {
		remaining[w]++
	}
	for _, g := range got {
		if remaining[g.Id] == 0 {
			return false
		}
		remaining[g.Id]--
	}
	return true
}

// The case that made the original condition wrong: exactly one ID correct. The
// old form, len(got) != 2 || (A && B), passed here because the correct ID made
// its half of the && false.
func Test_sameGroupIDs(t *testing.T) {
	g := func(ids ...string) []api.GroupMinimum {
		out := make([]api.GroupMinimum, 0, len(ids))
		for _, id := range ids {
			out = append(out, api.GroupMinimum{Id: id})
		}
		return out
	}
	for _, tc := range []struct {
		name string
		got  []api.GroupMinimum
		want bool
	}{
		{name: "both correct", got: g("group-all", "group-notall"), want: true},
		{name: "both correct, other order", got: g("group-notall", "group-all"), want: true},
		{name: "one wrong", got: g("group-all", "group-wrong")},
		{name: "the other wrong", got: g("group-wrong", "group-notall")},
		{name: "both wrong", got: g("a", "b")},
		{name: "duplicate stands in for the missing one", got: g("group-all", "group-all")},
		{name: "too few", got: g("group-all")},
		{name: "too many", got: g("group-all", "group-notall", "group-all")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameGroupIDs(tc.got, "group-all", "group-notall"); got != tc.want {
				t.Errorf("sameGroupIDs(%v) = %v, want %v", tc.got, got, tc.want)
			}
		})
	}
}
