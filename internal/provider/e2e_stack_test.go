// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The deployment the acceptance suite runs against is started from Go, not from
// a compose file: one netbird-server container (management, signal, relay, STUN
// and the embedded IdP in a single process), the dashboard, a reverse proxy and
// the agents, all on a private Docker network created per run.
//
// The images come from the netbird version pinned in go.mod (see
// test/build-images.sh), so the server under test is the same revision the
// provider's client library was generated from. Nothing here reaches into the
// management store: the account is created through POST /api/setup and every
// fixture through the public API.
const (
	// e2eServerAlias is the server's alias on the Docker network and the
	// deployment's domain. Peers reach management, signal and relay under this
	// name, and the reverse proxy registers it as its cluster, so it has to be
	// the same value everywhere.
	e2eServerAlias = "netbird.local"
	e2eServerPort  = "8080/tcp"
	e2eServerURL   = "http://" + e2eServerAlias + ":8080"

	e2eDashboardAlias = "dashboard"
	e2eDashboardPort  = "80/tcp"
	e2eProxyAlias     = "proxy"

	// e2eRelaySecret is the shared secret the embedded relay authenticates peer
	// credentials with. Any value works as long as the server owns both ends.
	e2eRelaySecret = "e2e-relay-secret" //nolint:gosec // throwaway secret for a disposable test deployment

	// e2eContainerIssuer is the embedded IdP issuer. It is only used for
	// internal JWT validation (peers authenticate with setup keys and the proxy
	// with an access token, never through OIDC), so the in-container address is
	// enough.
	e2eContainerIssuer = "http://localhost:8080/oauth2"
)

// e2eServerConfig is the netbird-server configuration the suite runs with: one
// HTTP listener, the embedded signal, relay and STUN, and a sqlite store in the
// bind-mounted data dir. %s is the address peers use to reach the container,
// which has to match its network alias.
const e2eServerConfig = `server:
  listenAddress: ":8080"
  exposedAddress: "%s"
  healthcheckAddress: ":9000"
  metricsPort: 9090
  logLevel: "debug"
  logFile: "console"
  authSecret: "` + e2eRelaySecret + `"
  dataDir: "/nb/data"
  disableAnonymousMetrics: true
  auth:
    issuer: "` + e2eContainerIssuer + `"
  store:
    engine: "sqlite"
  reverseProxy:
    trustedPeers:
      - "100.64.0.0/10"
`

// serverEnv is the netbird-server container's environment.
//
// Geolocation is left enabled: the posture-check tests assert on location rules
// that management can only evaluate with the GeoLite database loaded, so turning
// the download off would make them pass without testing anything. The database
// comes from a URL we publish. NB_E2E_DISABLE_GEOLOCATION turns it off for a
// machine that cannot reach that URL, at the cost of those checks — the server
// needs the download even to name an already-downloaded database, so there is no
// offline middle ground.
func serverEnv() map[string]string {
	env := map[string]string{
		// Without this the setup endpoint refuses to mint the API token the
		// whole suite authenticates with.
		"NB_SETUP_PAT_ENABLED": "true",
	}
	if os.Getenv("NB_E2E_DISABLE_GEOLOCATION") == "1" {
		env["NB_DISABLE_GEOLOCATION"] = "true"
	}
	return env
}

// e2eImages are the images the stack runs, resolved once by
// test/build-images.sh.
type e2eImages struct {
	Server    string
	Proxy     string
	Client    string
	Dashboard string
}

// e2eDocker owns everything the suite created in Docker. The reverse proxy and
// the agents start on first use rather than up front, so a run that only needs
// the API does not pay for them.
type e2eDocker struct {
	images  e2eImages
	network *testcontainers.DockerNetwork
	workDir string

	server    testcontainers.Container
	dashboard testcontainers.Container

	proxyOnce sync.Once
	proxy     testcontainers.Container
	proxyErr  error

	peersOnce sync.Once
	peers     []testcontainers.Container
	peersErr  error
}

// resolveE2EImages runs test/build-images.sh, which builds anything missing from
// the pinned netbird module and prints the tags it settled on.
func resolveE2EImages(ctx context.Context) (e2eImages, error) {
	var images e2eImages

	root, err := GetProjectDir()
	if err != nil {
		return images, err
	}
	script := filepath.Join(root, "test", "build-images.sh")
	cmd := exec.CommandContext(ctx, script)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return images, fmt.Errorf("%s: %w", script, err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		component, image, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch component {
		case "server":
			images.Server = image
		case "proxy":
			images.Proxy = image
		case "client":
			images.Client = image
		case "dashboard":
			images.Dashboard = image
		}
	}
	if images.Server == "" || images.Proxy == "" || images.Client == "" || images.Dashboard == "" {
		return images, fmt.Errorf("%s did not report every image, got %+v", script, images)
	}
	return images, nil
}

// startE2EDocker brings up the server and the dashboard on a fresh network and
// returns the host-reachable management URL.
func startE2EDocker(ctx context.Context) (*e2eDocker, string, string, error) {
	images, err := resolveE2EImages(ctx)
	if err != nil {
		return nil, "", "", err
	}

	net, err := network.New(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("create the deployment network: %w", err)
	}
	d := &e2eDocker{images: images, network: net}

	// Under /tmp rather than the default temp dir so Docker Desktop, which does
	// not share macOS's /var/folders, can bind-mount it.
	d.workDir, err = os.MkdirTemp("/tmp", "nb-e2e-*")
	if err != nil {
		return nil, "", "", d.failed(ctx, fmt.Errorf("create the work dir: %w", err))
	}
	config := fmt.Sprintf(e2eServerConfig, e2eServerURL)
	if err := os.WriteFile(filepath.Join(d.workDir, "config.yaml"), []byte(config), 0o644); err != nil { //nolint:gosec // non-secret config read by the server container
		return nil, "", "", d.failed(ctx, fmt.Errorf("write the server config: %w", err))
	}
	if err := os.MkdirAll(filepath.Join(d.workDir, "data"), 0o755); err != nil {
		return nil, "", "", d.failed(ctx, fmt.Errorf("create the data dir: %w", err))
	}

	d.server, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          images.Server,
			ExposedPorts:   []string{e2eServerPort},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {e2eServerAlias}},
			Env:            serverEnv(),
			Cmd:            []string{"--config", "/nb/config.yaml"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.Binds = append(hc.Binds, d.workDir+":/nb")
			},
			WaitingFor: wait.ForHTTP("/api/instance").
				WithPort(e2eServerPort).
				WithStatusCodeMatcher(func(status int) bool { return status == 200 }).
				WithStartupTimeout(3 * time.Minute),
		},
	})
	if err != nil {
		return nil, "", "", d.failed(ctx, fmt.Errorf("start the netbird-server container: %w", err))
	}

	managementURL, err := mappedURL(ctx, d.server, e2eServerPort)
	if err != nil {
		return nil, "", "", d.failed(ctx, err)
	}

	d.dashboard, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          images.Dashboard,
			ExposedPorts:   []string{e2eDashboardPort},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {e2eDashboardAlias}},
			Env: map[string]string{
				// The endpoint under test, by container alias: the dashboard
				// calls it from inside the network, so this is the wiring the
				// deployment test asserts on.
				"NETBIRD_MGMT_API_ENDPOINT":      e2eServerURL,
				"NETBIRD_MGMT_GRPC_API_ENDPOINT": e2eServerURL,
				"AUTH_AUTHORITY":                 e2eServerURL + "/oauth2",
				"AUTH_AUDIENCE":                  "netbird-client",
				"AUTH_CLIENT_ID":                 "netbird-client",
				"AUTH_CLIENT_SECRET":             "",
				"AUTH_SUPPORTED_SCOPES":          "openid profile email offline_access api",
				"AUTH_REDIRECT_URI":              "/peers",
				"AUTH_SILENT_REDIRECT_URI":       "/add-peers",
				"USE_AUTH0":                      "false",
				"NETBIRD_TOKEN_SOURCE":           "accessToken",
				"NGINX_SSL_PORT":                 "443",
			},
			WaitingFor: wait.ForHTTP("/").
				WithPort(e2eDashboardPort).
				WithStatusCodeMatcher(func(status int) bool { return status < 500 }).
				WithStartupTimeout(2 * time.Minute),
		},
	})
	if err != nil {
		return nil, "", "", d.failed(ctx, fmt.Errorf("start the dashboard container: %w", err))
	}

	dashboardURL, err := mappedURL(ctx, d.dashboard, e2eDashboardPort)
	if err != nil {
		return nil, "", "", d.failed(ctx, err)
	}

	return d, managementURL, dashboardURL, nil
}

// mappedURL is the http:// address the host can reach a container's port on.
func mappedURL(ctx context.Context, c testcontainers.Container, port string) (string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", fmt.Errorf("container host: %w", err)
	}
	mapped, err := c.MappedPort(ctx, nat.Port(port))
	if err != nil {
		return "", fmt.Errorf("mapped port %s: %w", port, err)
	}
	return fmt.Sprintf("http://%s:%s", host, mapped.Port()), nil
}

// startProxy mints a proxy access token through the server's own CLI, the same
// path an install uses, and runs the reverse proxy with it. Certificates are
// self-signed: the deployment's domain does not resolve publicly, so ACME could
// only ever fail, and nothing in the suite validates a chain.
func (d *e2eDocker) startProxy(ctx context.Context) error {
	d.proxyOnce.Do(func() {
		token, err := d.mintProxyToken(ctx)
		if err != nil {
			d.proxyErr = err
			return
		}

		certDir := filepath.Join(d.workDir, "certs")
		if err := os.MkdirAll(certDir, 0o755); err != nil {
			d.proxyErr = fmt.Errorf("create the certificate dir: %w", err)
			return
		}
		if err := writeSelfSignedCert(certDir, []string{e2eServerAlias, "*." + e2eServerAlias}); err != nil {
			d.proxyErr = err
			return
		}

		d.proxy, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:          d.images.Proxy,
				Networks:       []string{d.network.Name},
				NetworkAliases: map[string][]string{d.network.Name: {e2eProxyAlias}},
				Env: map[string]string{
					"NB_PROXY_TOKEN":                 token,
					"NB_PROXY_MANAGEMENT_ADDRESS":    e2eServerURL,
					"NB_PROXY_DOMAIN":                e2eServerAlias,
					"NB_PROXY_ADDRESS":               ":8443",
					"NB_PROXY_CERTIFICATE_DIRECTORY": "/certs",
					"NB_PROXY_HEALTH_ADDRESS":        ":8081",
					"NB_PROXY_LOG_LEVEL":             "debug",
					// Serve private services too, so the suite can point one at
					// a peer and at a network resource.
					"NB_PROXY_PRIVATE": "true",
					// Management speaks plain HTTP inside the network, so the
					// access token has to be allowed to ride a non-TLS
					// connection.
					"NB_PROXY_ALLOW_INSECURE": "true",
					// The server multiplexes the relay over WebSocket on its one
					// listener and has no QUIC listener. The proxy's embedded
					// client defaults to QUIC, which flaps the relay link and
					// churns the proxy peer so its cluster never settles.
					"NB_RELAY_TRANSPORT": "ws",
				},
				HostConfigModifier: func(hc *container.HostConfig) {
					hc.Binds = append(hc.Binds, certDir+":/certs")
					hc.CapAdd = append(hc.CapAdd, "NET_ADMIN", "SYS_ADMIN", "SYS_RESOURCE", "NET_BIND_SERVICE")
				},
				WaitingFor: wait.ForLog("Initial mapping sync complete").WithStartupTimeout(2 * time.Minute),
			},
		})
		if err != nil {
			d.proxyErr = fmt.Errorf("start the reverse proxy container: %w\n%s", err, d.logs(ctx, d.proxy))
		}
	})
	return d.proxyErr
}

// mintProxyToken runs `netbird-server token create` inside the server container
// and returns the plaintext token. A token created without an account scope is
// global, so the proxy serves the whole cluster.
func (d *e2eDocker) mintProxyToken(ctx context.Context) (string, error) {
	code, reader, err := d.server.Exec(ctx,
		[]string{"/go/bin/netbird-server", "token", "create", "--name", "e2e-proxy", "--config", "/nb/config.yaml"},
		tcexec.Multiplexed())
	if err != nil {
		return "", fmt.Errorf("exec token create: %w", err)
	}
	out, _ := io.ReadAll(reader)
	if code != 0 {
		return "", fmt.Errorf("token create exited %d: %s", code, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if token, ok := strings.CutPrefix(strings.TrimSpace(line), "Token:"); ok {
			if token = strings.TrimSpace(token); token != "" {
				return token, nil
			}
		}
	}
	return "", fmt.Errorf("no token in the CLI output: %s", out)
}

// startPeers runs one agent container per fixture hostname, each joining with
// the given setup key exactly the way a device does.
//
// The image's own entrypoint is replaced by test/agent-entrypoint.sh, which
// documents why `netbird up` alone does not register a peer against this server
// and what the fallback has to do about it.
func (d *e2eDocker) startPeers(ctx context.Context, setupKey string) error {
	root, err := GetProjectDir()
	if err != nil {
		return err
	}
	entrypoint := filepath.Join(root, "test", "agent-entrypoint.sh")

	d.peersOnce.Do(func() {
		for _, name := range e2ePeerNames {
			c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				Started: true,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:          d.images.Client,
					Networks:       []string{d.network.Name},
					NetworkAliases: map[string][]string{d.network.Name: {name}},
					// The container's own hostname, so the peer reports it as its
					// hostname and not just as its name. Tests assert on both.
					Hostname:   name,
					Entrypoint: []string{"/bin/sh", "/etc/netbird/agent-entrypoint.sh"},
					Files: []testcontainers.ContainerFile{{
						HostFilePath:      entrypoint,
						ContainerFilePath: "/etc/netbird/agent-entrypoint.sh",
						FileMode:          0o755,
					}},
					Env: map[string]string{
						"NB_MANAGEMENT_URL": e2eServerURL,
						"NB_SETUP_KEY":      setupKey,
						// Without this the peer registers under the container's
						// generated hostname and the tests cannot address it.
						"NB_HOSTNAME":  name,
						"NB_LOG_LEVEL": "info",
						// Match the proxy: the embedded relay is WebSocket only.
						"NB_RELAY_TRANSPORT": "ws",
					},
					HostConfigModifier: func(hc *container.HostConfig) {
						// The agent brings up a WireGuard interface, writes
						// routes and DNS, and raises the memlock limit for its
						// eBPF proxy, so it needs both the capabilities and the
						// tun device.
						hc.CapAdd = append(hc.CapAdd, "NET_ADMIN", "SYS_ADMIN", "SYS_RESOURCE")
						hc.Devices = append(hc.Devices, container.DeviceMapping{
							PathOnHost:        "/dev/net/tun",
							PathInContainer:   "/dev/net/tun",
							CgroupPermissions: "rwm",
						})
					},
					WaitingFor: wait.ForLog("Netbird engine started").WithStartupTimeout(2 * time.Minute),
				},
			})
			if err != nil {
				d.peersErr = fmt.Errorf("start the %s agent container: %w\n%s", name, err, d.logs(ctx, c))
				return
			}
			d.peers = append(d.peers, c)
		}
	})
	return d.peersErr
}

// logs reads a container's output, for diagnostics on a failure that leaves no
// container behind to inspect.
func (d *e2eDocker) logs(ctx context.Context, c testcontainers.Container) string {
	if c == nil {
		return ""
	}
	r, err := c.Logs(ctx)
	if err != nil {
		return fmt.Sprintf("<container logs unavailable: %v>", err)
	}
	defer r.Close()
	b, _ := io.ReadAll(io.LimitReader(r, 512<<10))
	return string(b)
}

// exec runs a command in one of the deployment's containers, so a test can
// assert on what that container reaches rather than on what the host reaches.
func (d *e2eDocker) exec(ctx context.Context, c testcontainers.Container, argv ...string) (string, error) {
	code, reader, err := c.Exec(ctx, argv, tcexec.Multiplexed())
	if err != nil {
		return "", err
	}
	out, _ := io.ReadAll(reader)
	if code != 0 {
		return string(out), fmt.Errorf("%s exited %d", strings.Join(argv, " "), code)
	}
	return string(out), nil
}

// failed tears a half-built stack down and returns the error that caused it, so
// a bootstrap failure does not leak containers into the next run.
func (d *e2eDocker) failed(ctx context.Context, err error) error {
	d.terminate(ctx)
	return err
}

// terminate removes every container and the network. Individual failures are
// ignored: there is nothing useful to do about them, and the reaper cleans up
// whatever is left.
func (d *e2eDocker) terminate(ctx context.Context) {
	for _, c := range append([]testcontainers.Container{d.server, d.dashboard, d.proxy}, d.peers...) {
		if c != nil {
			_ = c.Terminate(ctx)
		}
	}
	if d.network != nil {
		_ = d.network.Remove(ctx)
	}
	if d.workDir != "" {
		_ = os.RemoveAll(d.workDir)
	}
}
