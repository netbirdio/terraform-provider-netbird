// Copyright (c) HashiCorp, Inc.

//go:build e2e

// The acceptance tests need a live NetBird deployment, and this file brings one
// up. It is compiled only under the `e2e` build tag, and so is every test that
// calls into it — each resource's acceptance tests live in its own
// <resource>_acc_test.go alongside this one.
//
// A plain `go test ./...` compiles none of it: no deployment is started, nothing
// reaches for Docker, and the only tests in the binary are the ones that can
// answer without a server. The two suites are:
//
//	go test ./...                       # conversion and matching functions
//	TF_ACC=1 go test -tags e2e ./...    # everything above, against a deployment
//
// TF_ACC is terraform-plugin-testing's own switch: without it resource.Test
// skips, so the tag alone starts nothing.
//
// The deployment itself comes from netbird's own e2e harness, at the revision
// pinned in go.mod. That matters twice over: the server under test is built from
// the same revision as the client library the provider is compiled against, and
// the containers are the ones NetBird tests its own product with rather than a
// second arrangement maintained here. Nothing is written into the management
// store — the account is created through POST /api/setup and every fixture
// through the public API, so a change to onboarding or to the API contract fails
// the bootstrap instead of silently diverging from a hand-written seed.
//
// Environment knobs:
//
//	NB_E2E_COMBINED_IMAGE   use these images instead of building from the pinned
//	NB_E2E_CLIENT_IMAGE     module. A value containing a "/" is used as-is; a
//	NB_E2E_PROXY_IMAGE      bare tag is built under that name.
//	NB_E2E_BUILDX_CACHE     directory for buildx layer cache, so CI can persist it.

package provider

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/netbirdio/netbird/e2e/harness"
	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// The fixture the harness provisions, named rather than identified: IDs are
// assigned by the server at creation time and differ on every bootstrap, so the
// names are what the tests and the provisioning can agree on ahead of time.
const (
	e2eGroupAll    = "All"
	e2eGroupNotAll = "NotAll"

	e2eNetworkName  = "tfaccnetwork"
	e2eResourceDom  = "resource-domain"
	e2eResourceNet  = "resource-subnet"
	e2eResourceHost = "resource-host"
)

// e2ePeerNames are the hostnames the agent containers register under, in the
// order the tests expect to find them.
//
// They are split by purpose, because a test that manages a peer through its own
// lifecycle destroys it — and since Peer.Delete now really does deregister the
// device, a second test reaching for the same fixture finds nothing there.
// peer1 and peer2 are shared and read-only, addressed by name by the group,
// route, peers and reverse-proxy tests and expected to survive the whole run.
// peer3 to peer6 are consumable, one per test that manages a peer: create,
// delete, update, drift. One each rather than sharing, since sharing would make
// the later test depend on the order Go compiles the files in.
var e2ePeerNames = []string{"peer1", "peer2", "peer3", "peer4", "peer5", "peer6"}

// e2eStack is the live deployment plus the IDs of the fixtures created on it.
type e2eStack struct {
	ManagementURL string
	Token         string

	AccountID string
	UserID    string

	GroupAllID    string
	GroupNotAllID string

	NetworkID        string
	ResourceDomainID string
	ResourceSubnetID string
	ResourceHostID   string

	srv *harness.Combined

	// The reverse proxy and the agents start on first use rather than up front,
	// so a run that only touches the API does not pay for them.
	proxyOnce sync.Once
	proxyErr  error

	peersOnce sync.Once
	peerIDs   map[string]string
	peersErr  error

	clients []*harness.Client
	proxy   *harness.Proxy
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
	// that fails takes the server's log with it. Without this a management error
	// reaches the report as the one sentence the API returned, and the cause has
	// to be reproduced locally to be seen at all.
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		t.Logf("netbird-server log (tail):\n%s", logTail(e2eEnv.srv.Logs(context.Background()), 60))
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

// bootstrapE2E starts the deployment, activates the account through the
// product's own onboarding, and creates every fixture through the public API.
func bootstrapE2E(ctx context.Context) (*e2eStack, error) {
	// Geolocation stays on: the posture-check tests assert on location rules,
	// which management can only evaluate with the GeoLite database loaded, so
	// turning the download off would make them pass without testing anything.
	//
	// The cost is that the server refuses to start when it cannot fetch the
	// database, and the failure is opaque — the container exits before it serves,
	// so there is no log to read. NB_E2E_DISABLE_GEOLOCATION=1 is the escape
	// hatch for a machine that cannot reach the download, at the price of the
	// geolocation posture checks, which then fail rather than passing vacuously.
	var opts []harness.CombinedOption
	if os.Getenv("NB_E2E_DISABLE_GEOLOCATION") != "1" {
		opts = append(opts, harness.WithGeolocation())
	}
	srv, err := harness.StartCombined(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("start the netbird deployment "+
			"(a container that exits before serving is usually the GeoLite download failing; "+
			"see NB_E2E_DISABLE_GEOLOCATION): %w", err)
	}
	env := &e2eStack{srv: srv, ManagementURL: srv.BaseURL}

	//	GET  /api/instance -> setup_required
	//	POST /api/setup    -> owner user + plaintext API token
	//
	// which is how an installation comes up, and the token every test
	// authenticates with.
	token, err := srv.Bootstrap(ctx)
	if err != nil {
		return nil, env.failed(ctx, fmt.Errorf("activate the account through /api/setup: %w", err))
	}
	env.Token = token

	client := netbird.New(env.ManagementURL, env.Token)
	if err := env.discoverAccount(ctx, client); err != nil {
		return nil, env.failed(ctx, err)
	}
	if err := env.ensureFixtureGroups(ctx, client); err != nil {
		return nil, env.failed(ctx, err)
	}
	if err := env.ensureFixtureNetwork(ctx, client); err != nil {
		return nil, env.failed(ctx, err)
	}
	return env, nil
}

// discoverAccount records the account and owner the setup call created.
func (env *e2eStack) discoverAccount(ctx context.Context, client *netbird.Client) error {
	me, err := client.Users.Current(ctx)
	if err != nil {
		return fmt.Errorf("read the owner the setup call created: %w", err)
	}
	env.UserID = me.Id

	accounts, err := client.Accounts.List(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) != 1 {
		return fmt.Errorf("expected exactly one account on a freshly activated instance, found %d", len(accounts))
	}
	env.AccountID = accounts[0].Id
	return nil
}

// ensureFixtureGroups records the built-in All group and creates the second
// group the tests need to reference something that is not everything.
func (env *e2eStack) ensureFixtureGroups(ctx context.Context, client *netbird.Client) error {
	all, err := client.Groups.GetByName(ctx, e2eGroupAll)
	if err != nil {
		return fmt.Errorf("look up the built-in %q group: %w", e2eGroupAll, err)
	}
	env.GroupAllID = all.Id

	created, err := client.Groups.Create(ctx, api.PostApiGroupsJSONRequestBody{Name: e2eGroupNotAll})
	if err != nil {
		return fmt.Errorf("create the %q group: %w", e2eGroupNotAll, err)
	}
	env.GroupNotAllID = created.Id
	return nil
}

// ensureFixtureNetwork creates the network and the three resources the network
// and policy tests address.
func (env *e2eStack) ensureFixtureNetwork(ctx context.Context, client *netbird.Client) error {
	network, err := client.Networks.Create(ctx, api.PostApiNetworksJSONRequestBody{Name: e2eNetworkName})
	if err != nil {
		return fmt.Errorf("create the %q network: %w", e2eNetworkName, err)
	}
	env.NetworkID = network.Id

	resources := client.Networks.Resources(env.NetworkID)
	for _, r := range []struct {
		name    string
		address string
		into    *string
	}{
		{e2eResourceDom, "mock1.com", &env.ResourceDomainID},
		{e2eResourceNet, "192.168.0.0/16", &env.ResourceSubnetID},
		{e2eResourceHost, "10.0.0.5/32", &env.ResourceHostID},
	} {
		created, err := resources.Create(ctx, api.NetworkResourceRequest{
			Name:    r.name,
			Address: r.address,
			Enabled: true,
		})
		if err != nil {
			return fmt.Errorf("create the %q network resource: %w", r.name, err)
		}
		*r.into = created.Id
	}
	return nil
}

// testPeerID returns the management-assigned ID of the agent registered under
// the given hostname, starting the agents on first use.
func testPeerID(t *testing.T, hostname string) string {
	t.Helper()
	env := testE2E(t)
	if err := env.ensurePeers(context.Background()); err != nil {
		t.Fatalf("registering the agent fixtures: %v", err)
	}
	id, ok := env.peerIDs[hostname]
	if !ok {
		// Fatal, not a skip. ensurePeers registers a fixed set of hostnames and
		// has already reported success, so a missing one is a harness bug — and a
		// skip here would quietly drop every test that needs that agent while CI
		// stayed green.
		t.Fatalf("no registered agent named %q in this deployment; ensurePeers registers %v", hostname, slices.Sorted(maps.Keys(env.peerIDs)))
	}
	return id
}

// ensurePeers starts one agent container per fixture hostname, each joining with
// a setup key exactly the way a device does, and waits for management to report
// them. The name reaches the container's hostname, which is what the agent
// reports at registration and therefore the name the peer carries in the API.
func (env *e2eStack) ensurePeers(ctx context.Context) error {
	env.peersOnce.Do(func() {
		client := netbird.New(env.ManagementURL, env.Token)
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
		for _, name := range e2ePeerNames {
			c, err := harness.StartClient(ctx, env.srv, key.Key, harness.WithClientName(name))
			if err != nil {
				env.peersErr = fmt.Errorf("start the %q agent: %w", name, err)
				return
			}
			env.clients = append(env.clients, c)
		}
		env.peerIDs, env.peersErr = env.waitForPeers(ctx, client)
	})
	return env.peersErr
}

// waitForPeers polls until every agent has registered, so a test addressing
// peer3 does not race the agent still logging in.
func (env *e2eStack) waitForPeers(ctx context.Context, client *netbird.Client) (map[string]string, error) {
	const timeout = 3 * time.Minute
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
		if want[p.Hostname] {
			found[p.Hostname] = p.Id
		}
	}
	return found, nil
}

// testRequireProxyCluster returns an online reverse proxy cluster, starting the
// proxy on first use. Reverse proxy services need a cluster to be assigned to.
func testRequireProxyCluster(t *testing.T) api.ProxyCluster {
	t.Helper()
	env := testE2E(t)
	cluster, err := env.proxyCluster(context.Background())
	if err != nil {
		// Fatal, not a skip. This run owns the deployment, so every way this can
		// fail — the token, the image build, the container, registration, the
		// timeout waiting for the cluster — is a defect rather than a deployment
		// that happens not to offer a proxy. Skipping would take all nine reverse
		// proxy tests with it and leave the run green.
		t.Fatalf("bringing up the reverse proxy: %v", err)
	}
	return cluster
}

// proxyCluster starts the reverse proxy and waits for its cluster to come
// online. The cluster appears a moment after the proxy registers, so a single
// read is too early.
func (env *e2eStack) proxyCluster(ctx context.Context) (api.ProxyCluster, error) {
	env.proxyOnce.Do(func() {
		// A token created without an account scope is global, so the proxy
		// serves the whole cluster.
		token, err := env.srv.CreateProxyTokenCLI(ctx, "e2e-proxy")
		if err != nil {
			env.proxyErr = fmt.Errorf("mint a proxy access token: %w", err)
			return
		}
		env.proxy, env.proxyErr = harness.StartProxy(ctx, env.srv, token)
	})
	if env.proxyErr != nil {
		return api.ProxyCluster{}, env.proxyErr
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
		if time.Now().After(deadline) {
			return api.ProxyCluster{}, errors.New("no cluster reported online")
		}
		time.Sleep(3 * time.Second)
	}
}

// failed tears a half-built deployment down and returns the error that caused
// it, so a bootstrap failure does not leak containers into the next run. The
// server's log travels with the error, because by the time the run reports the
// failure the container is gone.
func (env *e2eStack) failed(ctx context.Context, err error) error {
	log := logTail(env.srv.Logs(ctx), 60)
	env.terminate(ctx)
	return fmt.Errorf("%w\nnetbird-server log (tail):\n%s", err, log)
}

// terminate removes every container. Individual failures are ignored: there is
// nothing useful to do about them, and the reaper cleans up whatever is left.
func (env *e2eStack) terminate(ctx context.Context) {
	for _, c := range env.clients {
		_ = c.Terminate(ctx)
	}
	if env.proxy != nil {
		_ = env.proxy.Terminate(ctx)
	}
	if env.srv != nil {
		_ = env.srv.Terminate(ctx)
	}
}

// mustE2E returns the bootstrapped deployment for the configuration builders,
// which have no *testing.T to skip with. Every test reaches its configuration
// only after testE2E has run, so this is never called before the bootstrap.
func mustE2E() *e2eStack {
	if e2eEnv == nil {
		panic("the e2e deployment was read before it was bootstrapped; the test's PreCheck must call testEnsureManagementRunning")
	}
	return e2eEnv
}

// The fixture IDs are assigned when the deployment is provisioned, so the
// configuration builders read them through here rather than hardcoding them.
func e2eGroupAllID() string       { return mustE2E().GroupAllID }
func e2eGroupNotAllID() string    { return mustE2E().GroupNotAllID }
func e2eNetworkID() string        { return mustE2E().NetworkID }
func e2eResourceDomainID() string { return mustE2E().ResourceDomainID }
func e2eResourceSubnetID() string { return mustE2E().ResourceSubnetID }
func e2eResourceHostID() string   { return mustE2E().ResourceHostID } //nolint:unused // completes the fixture set

// TestMain tears the deployment down after the suite. Without this the
// containers outlive the run whenever the reaper is disabled.
func TestMain(m *testing.M) {
	code := m.Run()
	if e2eEnv != nil {
		e2eEnv.terminate(context.Background())
	}
	os.Exit(code)
}

// testAccProtoV6ProviderFactories is used to instantiate a provider during
// acceptance testing. The factory is called for each Terraform CLI command to
// create a provider server the CLI connects to.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"netbird": providerserver.NewProtocol6WithError(New("test")()),
}

// testClient talks to the deployment the suite bootstrapped. It is called from
// Check functions that have no *testing.T to hand, so it reads the deployment
// testEnsureManagementRunning already brought up rather than starting one.
func testClient() *netbird.Client {
	env := mustE2E()
	return netbird.New(env.ManagementURL, env.Token)
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

// valOr dereferences an optional API field, falling back to a zero-ish default
// when the server omitted it.
func valOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

// sameIDSet reports whether the group references the server returned are exactly
// the wanted IDs, regardless of order — the API does not promise one.
func sameIDSet(got []api.GroupMinimum, want ...string) bool {
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

// The case that made the original assertions wrong: exactly one ID correct. They
// were written as len(got) != 2 || (A && B), and && binds tighter than ||, so the
// correct ID made its half of the && false and the whole condition passed.
func Test_sameIDSet(t *testing.T) {
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
		{name: "both correct", got: g("a", "b"), want: true},
		{name: "both correct, other order", got: g("b", "a"), want: true},
		{name: "one wrong", got: g("a", "wrong")},
		{name: "the other wrong", got: g("wrong", "b")},
		{name: "both wrong", got: g("x", "y")},
		{name: "duplicate stands in for the missing one", got: g("a", "a")},
		{name: "too few", got: g("a")},
		{name: "too many", got: g("a", "b", "a")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameIDSet(tc.got, "a", "b"); got != tc.want {
				t.Errorf("sameIDSet(%v) = %v, want %v", tc.got, got, tc.want)
			}
		})
	}
}

// Destroy coverage.
//
// The destroy half of a resource's lifecycle went almost entirely unchecked: 3
// of 65 acceptance tests asserted anything about it, so a resource that failed
// to delete server-side looked exactly like one that deleted cleanly. Terraform
// removes the resource from state either way, and the next test creates its own
// randomly named fixture, so nothing downstream notices the leak.
//
// Asking the server for the object by ID is stricter than listing objects and
// matching a name. It catches a delete that removed the wrong object, and it
// separates "the object is gone" from "the check could not tell" — a not-found
// is the only answer that proves a delete, where any other error means the
// check itself failed and must not be read as success.

// testRecordID captures a resource's server-assigned ID from state while the
// resource still exists. CheckDestroy runs once Terraform has forgotten the
// resource, so the ID has to be taken during the test rather than after it.
func testRecordID(resourceName string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s is not in state, so its ID cannot be recorded for the destroy check", resourceName)
		}
		id := rs.Primary.Attributes["id"]
		if id == "" {
			return fmt.Errorf("%s has no id attribute in state", resourceName)
		}
		*into = id
		return nil
	}
}

// testRecordAttr is testRecordID for any other attribute, which a destroy check
// needs when the object it must ask about is addressed through a parent — a DNS
// record lives under the zone in its zone_id, for instance.
func testRecordAttr(resourceName, attr string, into *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s is not in state, so its %s cannot be recorded for the destroy check", resourceName, attr)
		}
		v := rs.Primary.Attributes[attr]
		if v == "" {
			return fmt.Errorf("%s has no %s attribute in state", resourceName, attr)
		}
		*into = v
		return nil
	}
}

// testCheckGone builds a CheckDestroy that requires the recorded object to be
// absent from the management server. Pass the API's own getter, which for most
// resources is a method value such as testClient().Groups.Get.
func testCheckGone[T any](get func(context.Context, string) (T, error), id *string) func(*terraform.State) error {
	return func(*terraform.State) error {
		if *id == "" {
			// An empty ID means the test never reached the step that records it,
			// so reporting success here would assert nothing at all.
			return errors.New("the destroy check has no ID to look up; the test never recorded one")
		}
		_, err := get(context.Background(), *id)
		if err == nil {
			return fmt.Errorf("%s still exists on the management server after destroy", *id)
		}
		if !netbird.IsNotFound(err) {
			return fmt.Errorf("checking that %s was deleted: %w", *id, err)
		}
		return nil
	}
}

// testCheckAbsentFromList is the destroy check for a resource whose API has no
// by-ID getter, so the only way to ask is to list and look. Weaker than
// testCheckGone — a list error is indistinguishable from an empty list only if
// it is ignored, so it is returned instead.
func testCheckAbsentFromList[T any](list func(context.Context) ([]T, error), id func(T) string, want *string) func(*terraform.State) error {
	return func(*terraform.State) error {
		if *want == "" {
			return errors.New("the destroy check has no ID to look for; the test never recorded one")
		}
		items, err := list(context.Background())
		if err != nil {
			return fmt.Errorf("listing to check that %s was deleted: %w", *want, err)
		}
		for _, it := range items {
			if id(it) == *want {
				return fmt.Errorf("%s still exists on the management server after destroy", *want)
			}
		}
		return nil
	}
}

// testImportIDFrom builds a composite import ID out of attributes in state, for
// the resources whose ImportState expects more than the object's own ID — a
// network resource is addressed as network_id/id, a token as user_id/id.
//
// The separator is a parameter because the provider is not consistent about it:
// dns_record splits on ":" while the others split on "/".
func testImportIDFrom(resourceName, sep string, attrs ...string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("%s is not in state, so no import ID can be built", resourceName)
		}
		parts := make([]string, 0, len(attrs))
		for _, a := range attrs {
			v := rs.Primary.Attributes[a]
			if v == "" {
				return "", fmt.Errorf("%s has no %s in state", resourceName, a)
			}
			parts = append(parts, v)
		}
		return strings.Join(parts, sep), nil
	}
}

// testIDChanged asserts the object was replaced rather than updated in place, by
// comparing against the ID recorded in an earlier step. It is the assertion that
// makes a RequiresReplace declaration mean something: without it a provider that
// quietly updated in place would look identical to one that recreated.
func testIDChanged(resourceName string, previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if *previous == "" {
			return errors.New("no earlier ID was recorded, so a replacement cannot be detected")
		}
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s is not in state", resourceName)
		}
		if got := rs.Primary.Attributes["id"]; got == *previous {
			return fmt.Errorf("%s kept id %s across a change the schema marks RequiresReplace, so it was updated in place instead of being recreated", resourceName, got)
		}
		return nil
	}
}

// testExpectUpdateInPlace asserts the planned change to a resource is an update
// rather than a replacement.
//
// Comparing IDs across steps proves an object was replaced, but only after the
// fact and without saying why. This reads the plan itself, so a failure names
// the action Terraform chose and the attributes that forced it — which is the
// part that has to change to fix it. Only the attributes that can force a
// replacement are reported, both because the rest cannot be the cause and
// because a full dump of the object would carry the plaintext key with it.
func testExpectUpdateInPlace(address string) plancheck.PlanCheck {
	return updateInPlace{address: address}
}

type updateInPlace struct {
	address string
}

func (c updateInPlace) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address != c.address {
			continue
		}
		if slices.Equal(rc.Change.Actions, tfjson.Actions{tfjson.ActionUpdate}) {
			return
		}
		resp.Error = fmt.Errorf("%s: planned %v rather than an in-place update; replacement forced by %v; %s",
			c.address, rc.Change.Actions, rc.Change.ReplacePaths, describeReplaceable(rc.Change))
		return
	}
	resp.Error = fmt.Errorf("%s is not in the plan", c.address)
}

// describeReplaceable reports the before and after of every attribute the setup
// key schema marks RequiresReplace.
func describeReplaceable(change *tfjson.Change) string {
	before, _ := change.Before.(map[string]any)
	after, _ := change.After.(map[string]any)
	var parts []string
	for _, attr := range []string{"name", "type", "expiry_seconds", "usage_limit", "ephemeral", "allow_extra_dns_labels"} {
		parts = append(parts, fmt.Sprintf("%s: %v -> %v", attr, before[attr], after[attr]))
	}
	return strings.Join(parts, ", ")
}

// updatesInPlace is testExpectUpdateInPlace as a step's plan checks.
func updatesInPlace(address string) resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{testExpectUpdateInPlace(address)},
	}
}
