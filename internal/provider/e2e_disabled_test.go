// Copyright (c) HashiCorp, Inc.

//go:build !e2e

package provider

import (
	"testing"

	"github.com/netbirdio/netbird/shared/management/http/api"
)

// The acceptance tests need a live NetBird deployment, which the harness in
// e2e_stack_test.go starts in containers. That harness — and the whole Docker
// dependency behind it — is compiled only under the `e2e` build tag, so a plain
// `go test ./...` neither links testcontainers nor talks to a Docker socket.
//
// This file stands in for it in that build: every entry point skips, so the
// acceptance tests report as skipped instead of failing to compile. The fixture
// accessors are unreachable, because a test reaches its configuration only after
// taking the deployment, but they still have to exist for the package to build.
//
// Run the acceptance suite with:
//
//	TF_ACC=1 go test -tags e2e ./internal/provider/

// e2eStack mirrors the deployment description the harness fills in, so a test
// that reads a field from it still compiles here. Every value is zero: nothing
// was started, and no test gets this far without skipping first.
type e2eStack struct {
	ManagementURL string
	DashboardURL  string
	Token         string

	AccountID string
	UserID    string

	GroupAllID    string
	GroupNotAllID string

	NetworkID        string
	ResourceDomainID string
	ResourceSubnetID string
	ResourceHostID   string
}

// e2eEnv stays nil: testClient reads it, and only from a Check function inside a
// test that has already skipped.
var e2eEnv *e2eStack

const e2eSkipReason = "acceptance test skipped: build with -tags e2e (and set TF_ACC=1) to run it against a live NetBird deployment"

func testEnsureManagementRunning(t *testing.T) {
	t.Helper()
	t.Skip(e2eSkipReason)
}

func testE2E(t *testing.T) *e2eStack {
	t.Helper()
	t.Skip(e2eSkipReason)
	return nil
}

func testPeerID(t *testing.T, _ string) string {
	t.Helper()
	t.Skip(e2eSkipReason)
	return ""
}

func testRequireProxyCluster(t *testing.T) api.ProxyCluster {
	t.Helper()
	t.Skip(e2eSkipReason)
	return api.ProxyCluster{}
}

// mustE2E is what the fixture accessors and a few configuration builders read
// the deployment through. Nothing reaches it in this build.
func mustE2E() *e2eStack { return &e2eStack{} }

// The fixture IDs are assigned when the deployment is provisioned, so there is
// nothing to return here.
func e2eGroupAllID() string       { return "" }
func e2eGroupNotAllID() string    { return "" }
func e2eNetworkID() string        { return "" }
func e2eResourceDomainID() string { return "" }
func e2eResourceSubnetID() string { return "" }
func e2eResourceHostID() string   { return "" } //nolint:unused // completes the fixture set
