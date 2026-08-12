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

	attempts := 0
	backoff := 1 * time.Second
	for attempts < 5 {
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

	t.Fatal("Management Server not started")
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
