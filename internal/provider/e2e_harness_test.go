// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// The acceptance suite runs against a live NetBird deployment in containers
// (test/compose.yml): management, dashboard, signal, relay, a reverse proxy and
// three real agents. Nothing is written into the management store directly —
// the account is created through the product's own onboarding:
//
//	GET  /api/instance  -> setup_required
//	POST /api/setup     -> owner user + plaintext API token (create_pat)
//
// and every fixture below is then created through the public API with that
// token, or by a real agent registering with a setup key. That means a schema or
// behaviour change in management surfaces here as a failing bootstrap instead of
// silently diverging from a hand-written SQL seed.
//
// Environment knobs:
//
//	NB_E2E_MANAGEMENT_URL  point the suite at an already-running management
//	                       server and skip Docker entirely. Peer-dependent tests
//	                       skip unless that deployment already has the agents.
//	NB_E2E_TOKEN           API token for the above, when it is already set up.
//	NB_E2E_DASHBOARD_URL   dashboard URL for the above.
//	NB_E2E_TEARDOWN=1      compose down (and drop the volume) after the suite.
//	NB_E2E_KEEP_STATE=0    do not reuse a previous run's token.
const (
	e2eAdminEmail    = "admin@netbird.test"
	e2eAdminName     = "E2E Admin"
	e2eAdminPassword = "Netbird-e2e-Passw0rd!" //nolint:gosec // throwaway credential for a disposable test deployment

	// Fixture names. Names, not IDs, are the stable handle: IDs are assigned by
	// the server at creation time and differ on every bootstrap.
	e2eGroupAll     = "All"
	e2eGroupNotAll  = "NotAll"
	e2eNetworkName  = "tfaccnetwork"
	e2eResourceHost = "resource-host"
	e2eResourceNet  = "resource-subnet"
	e2eResourceDom  = "resource-domain"

	e2eComposeProject = "tfnetbird-e2e"
)

// e2ePeerNames are the NB_HOSTNAME values given to the agent containers in
// test/compose.yml, in the order tests expect to find them.
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

	// PeerIDs maps a compose hostname ("peer1") to the ID management assigned
	// the agent when it registered. Empty when the deployment has no agents.
	PeerIDs map[string]string
}

// e2eState is persisted next to the compose file so a second `go test` run
// against a still-running stack can reuse the token: /api/setup only ever
// succeeds once.
type e2eState struct {
	ManagementURL string `json:"management_url"`
	DashboardURL  string `json:"dashboard_url"`
	Token         string `json:"token"`
}

var (
	e2eOnce  sync.Once
	e2eEnv   *e2eStack
	e2eErr   error
	e2eOwned bool // true when this process brought the stack up
)

// testEnsureManagementRunning boots the stack once per test binary and points
// the provider under test at it. Every acceptance test calls this from PreCheck.
func testEnsureManagementRunning(t *testing.T) {
	t.Helper()
	testE2E(t)
}

// testE2E returns the shared stack, bootstrapping it on first use.
func testE2E(t *testing.T) *e2eStack {
	t.Helper()
	e2eOnce.Do(func() {
		e2eEnv, e2eErr = bootstrapE2E(context.Background())
	})
	if e2eErr != nil {
		t.Fatalf("e2e stack unavailable: %v", e2eErr)
	}
	t.Setenv("NB_PAT", e2eEnv.Token)
	t.Setenv("NB_MANAGEMENT_URL", e2eEnv.ManagementURL)
	return e2eEnv
}

// testPeerID returns the management-assigned ID of the agent that registered
// under the given compose hostname, skipping the test when the deployment under
// test has no agents (an externally supplied management URL, say).
func testPeerID(t *testing.T, hostname string) string {
	t.Helper()
	env := testE2E(t)
	id, ok := env.PeerIDs[hostname]
	if !ok {
		t.Skipf("no registered agent named %q in this deployment", hostname)
	}
	return id
}

// mustE2E returns the bootstrapped stack for the HCL builders, which have no
// *testing.T to hand. Every acceptance test calls testE2E before building its
// configuration, so by the time a builder runs the fixtures exist.
func mustE2E() *e2eStack {
	if e2eEnv == nil {
		panic("e2e fixtures used before the stack was bootstrapped; call testE2E(t) at the top of the test")
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
	env := &e2eStack{PeerIDs: map[string]string{}}

	if url := os.Getenv("NB_E2E_MANAGEMENT_URL"); url != "" {
		env.ManagementURL = strings.TrimSuffix(url, "/")
		env.DashboardURL = strings.TrimSuffix(os.Getenv("NB_E2E_DASHBOARD_URL"), "/")
		env.Token = os.Getenv("NB_E2E_TOKEN")
	} else {
		if err := composeUpCore(ctx); err != nil {
			return nil, err
		}
		e2eOwned = true
		env.ManagementURL = fmt.Sprintf("http://127.0.0.1:%s", hostPort("NETBIRD_MGMT_HOST_PORT", "8080"))
		env.DashboardURL = fmt.Sprintf("http://127.0.0.1:%s", hostPort("NETBIRD_DASHBOARD_HOST_PORT", "8090"))
	}

	// Setup only succeeds once per deployment, so a second run against a stack
	// that is still up has to recover the token it minted the first time.
	if env.Token == "" {
		if prev, err := readE2EState(); err == nil && prev.ManagementURL == env.ManagementURL {
			env.Token = prev.Token
			if env.DashboardURL == "" {
				env.DashboardURL = prev.DashboardURL
			}
		}
	}

	if err := waitForInstance(ctx, env.ManagementURL, 3*time.Minute); err != nil {
		return nil, err
	}

	if err := activateAccount(ctx, env); err != nil {
		return nil, err
	}

	if err := writeE2EState(e2eState{
		ManagementURL: env.ManagementURL,
		DashboardURL:  env.DashboardURL,
		Token:         env.Token,
	}); err != nil {
		return nil, err
	}

	client := netbird.New(env.ManagementURL, env.Token)
	if err := discoverAccount(ctx, client, env); err != nil {
		return nil, err
	}
	if err := ensureFixtureGroups(ctx, client, env); err != nil {
		return nil, err
	}
	if e2eOwned {
		if err := startAgents(ctx, client); err != nil {
			return nil, err
		}
		if err := startReverseProxy(ctx); err != nil {
			return nil, err
		}
	}
	if err := discoverPeers(ctx, client, env); err != nil {
		return nil, err
	}
	if err := ensureFixtureNetwork(ctx, client, env); err != nil {
		return nil, err
	}
	return env, nil
}

// activateAccount performs the one-time instance setup, which creates the owner
// user and hands back the API token the provider authenticates with. When the
// instance reports it is already set up, the token has to come from a previous
// run's state file or the environment — the endpoint cannot be replayed.
func activateAccount(ctx context.Context, env *e2eStack) error {
	required, err := setupRequired(ctx, env.ManagementURL)
	if err != nil {
		return err
	}
	if !required {
		if env.Token == "" {
			return fmt.Errorf("management at %s is already set up but no API token is available; "+
				"pass NB_E2E_TOKEN, or reset the deployment with `docker compose -p %s -f test/compose.yml down -v`",
				env.ManagementURL, e2eComposeProject)
		}
		if err := tokenWorks(ctx, env.ManagementURL, env.Token); err != nil {
			return fmt.Errorf("stored API token rejected by %s: %w; reset the deployment with "+
				"`docker compose -p %s -f test/compose.yml down -v`", env.ManagementURL, err, e2eComposeProject)
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
		return errors.New("POST /api/setup returned no API token; management must run with NB_SETUP_PAT_ENABLED=true")
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

func tokenWorks(ctx context.Context, managementURL, token string) error {
	_, err := netbird.New(managementURL, token).Users.Current(ctx)
	return err
}

// waitForInstance polls the unauthenticated instance endpoint until management
// serves it, which is the earliest point the API is usable.
func waitForInstance(ctx context.Context, managementURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		if _, err := setupRequired(ctx, managementURL); err == nil {
			return nil
		} else {
			last = err
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("management API at %s not ready after %s: %w", managementURL, timeout, last)
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
// group the policy/route/DNS tests need. Both are idempotent so a reused stack
// converges instead of accumulating duplicates.
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

// startAgents mints a setup key through the API and brings the agent containers
// up with it, so the peers the tests operate on join exactly the way a real
// device does.
func startAgents(ctx context.Context, client *netbird.Client) error {
	if have, err := registeredPeers(ctx, client); err == nil && len(have) >= len(e2ePeerNames) {
		return nil
	}

	key, err := client.SetupKeys.Create(ctx, api.PostApiSetupKeysJSONRequestBody{
		Name:       "e2e-agents",
		Type:       "reusable",
		ExpiresIn:  86400,
		UsageLimit: 0,
		AutoGroups: []string{},
	})
	if err != nil {
		return fmt.Errorf("create setup key for the agents: %w", err)
	}

	args := []string{"--profile", "peers", "up", "-d"}
	for _, name := range e2ePeerNames {
		args = append(args, "netbird-"+name)
	}
	if _, err := compose(ctx, []string{"NB_E2E_SETUP_KEY=" + key.Key}, args...); err != nil {
		return fmt.Errorf("start agent containers: %w", err)
	}
	return nil
}

// startReverseProxy mints a proxy access token with the management CLI — the
// same path a real install uses — and starts the proxy with it.
func startReverseProxy(ctx context.Context) error {
	out, err := compose(ctx, nil, "exec", "-T", "netbird-mgmt", "sh", "-c",
		"/go/bin/netbird-mgmt token create --config /etc/netbird/management.json --name test-proxy --log-file console 2>/dev/null "+
			"| awk '/^Token:/ {print \"NB_PROXY_TOKEN=\"$2}' > /var/lib/netbird/proxy.env; cat /var/lib/netbird/proxy.env")
	if err != nil {
		return fmt.Errorf("mint proxy token: %w", err)
	}
	if !strings.Contains(out, "NB_PROXY_TOKEN=") {
		return fmt.Errorf("management CLI produced no proxy token, output: %s", out)
	}
	if _, err := compose(ctx, nil, "--profile", "proxy", "up", "-d", "--wait", "--wait-timeout", "120"); err != nil {
		return fmt.Errorf("start reverse proxy: %w", err)
	}
	return nil
}

// discoverPeers waits for the agents to appear in the API and records their IDs.
// A deployment that was handed to us without agents simply has none.
func discoverPeers(ctx context.Context, client *netbird.Client, env *e2eStack) error {
	timeout := 3 * time.Minute
	if !e2eOwned {
		// Nothing is starting agents on our behalf, so do not wait around.
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for {
		found, err := registeredPeers(ctx, client)
		if err != nil {
			return fmt.Errorf("list peers: %w", err)
		}
		env.PeerIDs = found
		if len(found) >= len(e2ePeerNames) || time.Now().After(deadline) {
			break
		}
		time.Sleep(3 * time.Second)
	}
	if e2eOwned && len(env.PeerIDs) < len(e2ePeerNames) {
		return fmt.Errorf("only %d of %d agents registered within the timeout; check `docker compose -p %s -f test/compose.yml logs netbird-peer1`",
			len(env.PeerIDs), len(e2ePeerNames), e2eComposeProject)
	}
	return nil
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

// composeUpCore starts the always-on half of the deployment and blocks until
// every container reports healthy.
func composeUpCore(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is required to run the acceptance suite (or set NB_E2E_MANAGEMENT_URL to an existing deployment): %w", err)
	}
	if _, err := compose(ctx, nil, "up", "-d", "--wait", "--wait-timeout", "180"); err != nil {
		return fmt.Errorf("start the core services: %w", err)
	}
	return nil
}

// compose runs docker compose against the suite's project and file, returning
// combined output. extraEnv entries are "KEY=value" pairs added to the child's
// environment for compose interpolation.
func compose(ctx context.Context, extraEnv []string, args ...string) (string, error) {
	root, err := GetProjectDir()
	if err != nil {
		return "", err
	}
	full := append([]string{"compose", "-p", composeProject(), "-f", filepath.Join(root, "test", "compose.yml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	cmd.Dir = filepath.Join(root, "test")
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker %s: %w\n%s", strings.Join(full, " "), err, string(out))
	}
	return string(out), nil
}

// composeExecHTTP fetches a URL from inside a service container, so a test can
// assert on what that container can reach rather than on what the host can.
// Images vary in which HTTP client they ship, so try both.
func composeExecHTTP(ctx context.Context, service, url string) (string, error) {
	var lastErr error
	for _, argv := range [][]string{
		{"wget", "-q", "-O", "-", url},
		{"curl", "-fsS", url},
	} {
		out, err := compose(ctx, nil, append([]string{"exec", "-T", service}, argv...)...)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func composeProject() string {
	if v := os.Getenv("COMPOSE_PROJECT_NAME"); v != "" {
		return v
	}
	return e2eComposeProject
}

func hostPort(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

func e2eStatePath() (string, error) {
	root, err := GetProjectDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "test", ".e2e-state.json"), nil
}

func readE2EState() (e2eState, error) {
	var s e2eState
	if os.Getenv("NB_E2E_KEEP_STATE") == "0" {
		return s, errors.New("state reuse disabled")
	}
	path, err := e2eStatePath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

func writeE2EState(s e2eState) error {
	path, err := e2eStatePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// httpGet is a small helper for the checks that assert on raw HTTP responses
// (the dashboard, mainly) rather than on the management API.
func httpGet(ctx context.Context, url string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	// The deployment is on loopback; a proxy configured for outbound traffic
	// must not intercept it.
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{Proxy: nil},
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	var body bytes.Buffer
	if _, err := body.ReadFrom(resp.Body); err != nil {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, body.String(), nil
}

// TestMain tears the deployment down after the suite when asked. The default is
// to leave it running: bootstrapping costs minutes, and a warm stack makes an
// edit/run cycle bearable.
func TestMain(m *testing.M) {
	code := m.Run()
	if e2eOwned && os.Getenv("NB_E2E_TEARDOWN") == "1" {
		if _, err := compose(context.Background(), nil, "--profile", "peers", "--profile", "proxy", "down", "-v"); err != nil {
			fmt.Fprintf(os.Stderr, "e2e teardown failed: %v\n", err)
		}
		if path, err := e2eStatePath(); err == nil {
			_ = os.Remove(path)
		}
	}
	os.Exit(code)
}
