// Copyright (c) HashiCorp, Inc.

//go:build e2e

package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// The acceptance suite runs against the live deployment started in
// e2e_stack_test.go. Nothing is written into the management store directly: the
// account is created through the product's own onboarding,
//
//	GET  /api/instance  -> setup_required
//	POST /api/setup     -> owner user + plaintext API token (create_pat)
//
// and every fixture below is created through the public API with that token, or
// by a real agent registering with a setup key. A schema or behaviour change in
// management therefore surfaces here as a failing bootstrap instead of silently
// diverging from a hand-written seed.
//
// Environment knobs:
//
//	NB_E2E_MANAGEMENT_URL  point the suite at an already-running deployment and
//	                       skip Docker entirely. Tests that need agents or a
//	                       reverse proxy skip unless that deployment has them.
//	NB_E2E_TOKEN           API token for the above.
//	NB_E2E_DASHBOARD_URL   dashboard URL for the above.
//	NB_E2E_KEEP_STACK=1    leave the containers running after the suite.
//
// test/build-images.sh documents the knobs that select the images.

// e2ePeerNames are the hostnames the agent containers register under, in the
// order tests expect to find them.
var e2ePeerNames = []string{"peer1", "peer2", "peer3"}

// e2eStack is the live deployment plus the IDs of the fixtures created on it.
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

	// docker is the containers this run created, and is nil when the suite was
	// pointed at a deployment it does not own.
	docker *e2eDocker

	// peerIDs maps a fixture hostname to the ID management assigned the agent
	// when it registered, filled in on first use.
	peerIDs   map[string]string
	peersOnce sync.Once
	peersErr  error
}

var (
	e2eOnce sync.Once
	e2eEnv  *e2eStack
	e2eErr  error
)

// testEnsureManagementRunning boots the deployment once per test binary and
// points the provider under test at it. Every acceptance test calls this from
// PreCheck.
func testEnsureManagementRunning(t *testing.T) {
	t.Helper()
	testE2E(t)
}

// testE2E returns the shared deployment, bootstrapping it on first use.
//
// Whether a deployment can be started at all is decided by the `e2e` build tag:
// without it this file is not compiled and e2e_disabled_test.go skips instead, so
// a plain `go test ./...` links no Docker client and starts no containers.
//
// TF_ACC is terraform-plugin-testing's own switch, and it is honoured here so
// that `go test -tags e2e` without it skips rather than spending minutes on
// containers for tests that resource.Test would then skip anyway.
func testE2E(t *testing.T) *e2eStack {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test skipped; set TF_ACC=1 to run it against the deployment this build can start")
	}
	e2eOnce.Do(func() {
		e2eEnv, e2eErr = bootstrapE2E(context.Background())
	})
	if e2eErr != nil {
		t.Fatalf("e2e deployment unavailable: %v", e2eErr)
	}
	// The containers are gone by the time a run reports its failures, so a test
	// that fails takes the server's own log with it. Without this, a management
	// error reaches the report as the one sentence the API returned, and the
	// cause has to be reproduced locally to be seen at all.
	t.Cleanup(func() {
		if !t.Failed() || e2eEnv.docker == nil {
			return
		}
		t.Logf("netbird-server log (tail):\n%s", logTail(e2eEnv.docker.logs(context.Background(), e2eEnv.docker.server), 60))
	})
	t.Setenv("NB_PAT", e2eEnv.Token)
	t.Setenv("NB_MANAGEMENT_URL", e2eEnv.ManagementURL)
	return e2eEnv
}

// logTail is the last n lines of a container log, which is as much as a failure
// report can carry usefully.
func logTail(log string, n int) string {
	lines := strings.Split(strings.TrimRight(log, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// testPeerID returns the management-assigned ID of the agent registered under
// the given hostname, starting the agents on first use. It skips the test when
// the deployment under test has no agents, which is the case for an externally
// supplied management URL.
func testPeerID(t *testing.T, hostname string) string {
	t.Helper()
	env := testE2E(t)
	if err := env.ensurePeers(context.Background()); err != nil {
		t.Fatalf("registering the agent fixtures: %v", err)
	}
	id, ok := env.peerIDs[hostname]
	if !ok {
		t.Skipf("no registered agent named %q in this deployment", hostname)
	}
	return id
}

// testRequireProxyCluster returns an online reverse proxy cluster, starting the
// proxy on first use. Reverse proxy services need a cluster to be assigned to.
func testRequireProxyCluster(t *testing.T) api.ProxyCluster {
	t.Helper()
	env := testE2E(t)
	cluster, err := env.proxyCluster(context.Background())
	if err != nil {
		t.Skipf("no reverse proxy cluster in this deployment: %v", err)
	}
	return cluster
}

// ensurePeers starts the agent containers and waits for management to report
// them. A deployment the suite does not own is taken as it is.
func (env *e2eStack) ensurePeers(ctx context.Context) error {
	env.peersOnce.Do(func() {
		client := netbird.New(env.ManagementURL, env.Token)
		if env.docker != nil {
			key, err := client.SetupKeys.Create(ctx, api.PostApiSetupKeysJSONRequestBody{
				Name:       "e2e-agents",
				Type:       "reusable",
				ExpiresIn:  86400,
				UsageLimit: 0,
				AutoGroups: []string{},
			})
			if err != nil {
				env.peersErr = fmt.Errorf("create a setup key for the agents: %w", err)
				return
			}
			if err := env.docker.startPeers(ctx, key.Key); err != nil {
				env.peersErr = err
				return
			}
		}
		env.peerIDs, env.peersErr = env.waitForPeers(ctx, client)
	})
	return env.peersErr
}

// waitForPeers polls until every agent has registered, so a test that addresses
// peer3 does not race the agent that is still logging in.
func (env *e2eStack) waitForPeers(ctx context.Context, client *netbird.Client) (map[string]string, error) {
	timeout := 3 * time.Minute
	if env.docker == nil {
		// Nothing is starting agents on our behalf, so do not wait around.
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		found, err := registeredPeers(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("list peers: %w", err)
		}
		if len(found) >= len(e2ePeerNames) {
			return found, nil
		}
		if time.Now().After(deadline) {
			if env.docker == nil {
				return found, nil
			}
			return nil, fmt.Errorf("only %d of %d agents registered within %s", len(found), len(e2ePeerNames), timeout)
		}
		time.Sleep(3 * time.Second)
	}
}

// registeredPeers maps the fixture hostnames to the peer IDs management holds.
func registeredPeers(ctx context.Context, client *netbird.Client) (map[string]string, error) {
	peers, err := client.Peers.List(ctx)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, n := range e2ePeerNames {
		want[n] = true
	}
	found := map[string]string{}
	for _, p := range peers {
		for _, candidate := range []string{p.Hostname, p.Name} {
			if want[candidate] {
				if _, seen := found[candidate]; !seen {
					found[candidate] = p.Id
				}
			}
		}
	}
	return found, nil
}

// proxyCluster starts the reverse proxy if this run owns the deployment, then
// waits for its cluster to come online. The cluster appears a moment after the
// proxy's first mapping sync, so a single read is too early.
func (env *e2eStack) proxyCluster(ctx context.Context) (api.ProxyCluster, error) {
	if env.docker != nil {
		if err := env.docker.startProxy(ctx); err != nil {
			return api.ProxyCluster{}, err
		}
	}
	client := netbird.New(env.ManagementURL, env.Token)
	deadline := time.Now().Add(2 * time.Minute)
	for {
		clusters, err := client.ReverseProxyClusters.List(ctx)
		if err != nil {
			return api.ProxyCluster{}, fmt.Errorf("list reverse proxy clusters: %w", err)
		}
		for _, c := range clusters {
			if c.Online {
				return c, nil
			}
		}
		if env.docker == nil || time.Now().After(deadline) {
			return api.ProxyCluster{}, errors.New("no cluster reported online")
		}
		time.Sleep(3 * time.Second)
	}
}

// mustE2E returns the bootstrapped deployment for the HCL builders, which have
// no *testing.T to hand. Every acceptance test calls testE2E before building its
// configuration, so by the time a builder runs the fixtures exist.
func mustE2E() *e2eStack {
	if e2eEnv == nil {
		panic("e2e fixtures used before the deployment was bootstrapped; call testE2E(t) at the top of the test")
	}
	return e2eEnv
}

func e2eGroupAllID() string       { return mustE2E().GroupAllID }
func e2eGroupNotAllID() string    { return mustE2E().GroupNotAllID }
func e2eNetworkID() string        { return mustE2E().NetworkID }
func e2eResourceDomainID() string { return mustE2E().ResourceDomainID }
func e2eResourceSubnetID() string { return mustE2E().ResourceSubnetID }
func e2eResourceHostID() string   { return mustE2E().ResourceHostID } //nolint:unused // completes the fixture set

// bootstrapE2E brings the deployment up and returns it fully provisioned.
func bootstrapE2E(ctx context.Context) (*e2eStack, error) {
	env := &e2eStack{peerIDs: map[string]string{}}

	if url := os.Getenv("NB_E2E_MANAGEMENT_URL"); url != "" {
		env.ManagementURL = strings.TrimSuffix(url, "/")
		env.DashboardURL = strings.TrimSuffix(os.Getenv("NB_E2E_DASHBOARD_URL"), "/")
		env.Token = os.Getenv("NB_E2E_TOKEN")
	} else {
		docker, managementURL, dashboardURL, err := startE2EDocker(ctx)
		if err != nil {
			return nil, err
		}
		env.docker = docker
		env.ManagementURL = managementURL
		env.DashboardURL = dashboardURL
	}

	if err := activateAccount(ctx, env); err != nil {
		if env.docker != nil {
			return nil, fmt.Errorf("%w\n%s", err, env.docker.logs(ctx, env.docker.server))
		}
		return nil, err
	}

	client := netbird.New(env.ManagementURL, env.Token)
	if err := discoverAccount(ctx, client, env); err != nil {
		return nil, err
	}
	if err := ensureFixtureGroups(ctx, client, env); err != nil {
		return nil, err
	}
	if err := ensureFixtureNetwork(ctx, client, env); err != nil {
		return nil, err
	}
	return env, nil
}

// activateAccount performs the one-time instance setup, which creates the owner
// user and hands back the API token the provider authenticates with. The
// endpoint cannot be replayed, so a deployment that reports itself set up
// already must come with a token.
func activateAccount(ctx context.Context, env *e2eStack) error {
	required, err := setupRequired(ctx, env.ManagementURL)
	if err != nil {
		return err
	}
	if !required {
		if env.Token == "" {
			return fmt.Errorf("management at %s is already set up but no API token is available; pass NB_E2E_TOKEN",
				env.ManagementURL)
		}
		if _, err := netbird.New(env.ManagementURL, env.Token).Users.Current(ctx); err != nil {
			return fmt.Errorf("the API token for %s was rejected: %w", env.ManagementURL, err)
		}
		return nil
	}

	createPAT := true
	expireDays := 1
	// The setup endpoint is unauthenticated by design: it is the only way in
	// before an owner exists.
	setupClient := netbird.NewWithOptions(netbird.WithManagementURL(env.ManagementURL))
	resp, err := setupClient.Instance.Setup(ctx, api.PostApiSetupJSONRequestBody{
		Email:       e2eAdminEmail,
		Name:        e2eAdminName,
		Password:    e2eAdminPassword,
		CreatePat:   &createPAT,
		PatExpireIn: &expireDays,
	})
	if err != nil {
		return fmt.Errorf("POST /api/setup: %w", err)
	}
	if resp.PersonalAccessToken == nil || *resp.PersonalAccessToken == "" {
		return errors.New("POST /api/setup returned no API token; the server must run with NB_SETUP_PAT_ENABLED=true")
	}
	env.Token = *resp.PersonalAccessToken
	env.UserID = resp.UserId
	return nil
}

func setupRequired(ctx context.Context, managementURL string) (bool, error) {
	status, err := netbird.NewWithOptions(netbird.WithManagementURL(managementURL)).Instance.GetStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("GET /api/instance: %w", err)
	}
	return status.SetupRequired, nil
}

// discoverAccount records the IDs the tests assert against. They are generated
// per deployment, so nothing may hardcode them.
func discoverAccount(ctx context.Context, client *netbird.Client, env *e2eStack) error {
	user, err := client.Users.Current(ctx)
	if err != nil {
		return fmt.Errorf("read current user: %w", err)
	}
	env.UserID = user.Id

	accounts, err := client.Accounts.List(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) == 0 {
		return errors.New("no account on a freshly set-up management server")
	}
	env.AccountID = accounts[0].Id
	return nil
}

// ensureFixtureGroups resolves the built-in "All" group and creates the second
// group the policy, route and DNS tests need. Both are idempotent so a reused
// deployment converges instead of accumulating duplicates.
func ensureFixtureGroups(ctx context.Context, client *netbird.Client, env *e2eStack) error {
	all, err := client.Groups.GetByName(ctx, e2eGroupAll)
	if err != nil {
		return fmt.Errorf("look up the built-in %q group: %w", e2eGroupAll, err)
	}
	env.GroupAllID = all.Id

	if existing, err := client.Groups.GetByName(ctx, e2eGroupNotAll); err == nil && existing != nil {
		env.GroupNotAllID = existing.Id
		return nil
	}
	created, err := client.Groups.Create(ctx, api.PostApiGroupsJSONRequestBody{Name: e2eGroupNotAll})
	if err != nil {
		return fmt.Errorf("create the %q group: %w", e2eGroupNotAll, err)
	}
	env.GroupNotAllID = created.Id
	return nil
}

// ensureFixtureNetwork creates the network and the three resource shapes
// (domain, subnet, host) that the network-resource, router and policy tests
// reference.
func ensureFixtureNetwork(ctx context.Context, client *netbird.Client, env *e2eStack) error {
	networks, err := client.Networks.List(ctx)
	if err != nil {
		return fmt.Errorf("list networks: %w", err)
	}
	for _, n := range networks {
		if n.Name == e2eNetworkName {
			env.NetworkID = n.Id
			break
		}
	}
	if env.NetworkID == "" {
		created, err := client.Networks.Create(ctx, api.PostApiNetworksJSONRequestBody{Name: e2eNetworkName})
		if err != nil {
			return fmt.Errorf("create the %q network: %w", e2eNetworkName, err)
		}
		env.NetworkID = created.Id
	}

	resources := client.Networks.Resources(env.NetworkID)
	existing, err := resources.List(ctx)
	if err != nil {
		return fmt.Errorf("list network resources: %w", err)
	}
	byName := map[string]string{}
	for _, r := range existing {
		byName[r.Name] = r.Id
	}

	wanted := []struct {
		name    string
		address string
		target  *string
	}{
		{e2eResourceDom, "mock1.com", &env.ResourceDomainID},
		{e2eResourceNet, "192.168.0.0/16", &env.ResourceSubnetID},
		{e2eResourceHost, "10.0.0.5/32", &env.ResourceHostID},
	}
	for _, w := range wanted {
		if id, ok := byName[w.name]; ok {
			*w.target = id
			continue
		}
		created, err := resources.Create(ctx, api.NetworkResourceRequest{
			Name:    w.name,
			Address: w.address,
			Enabled: true,
			Groups:  []string{env.GroupNotAllID},
		})
		if err != nil {
			return fmt.Errorf("create network resource %q: %w", w.name, err)
		}
		*w.target = created.Id
	}
	return nil
}

// TestMain tears the deployment down after the suite. NB_E2E_KEEP_STACK leaves
// it up for inspection, which is worth doing when a failure needs the server
// logs and the containers are cheaper to keep than to reproduce.
func TestMain(m *testing.M) {
	code := m.Run()
	if e2eEnv != nil && e2eEnv.docker != nil && os.Getenv("NB_E2E_KEEP_STACK") != "1" {
		e2eEnv.docker.terminate(context.Background())
	}
	os.Exit(code)
}
