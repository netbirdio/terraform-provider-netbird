# End-to-end test deployment

`compose.yml` stands up a real NetBird deployment for the provider's acceptance
tests. Nothing writes into the management store directly: the account is created
the way an operator creates it, and the peers the tests operate on are peers that
real agents registered.

```
                    POST /api/setup (create_pat)
  go test ──────────────────────────────────────▶ management ◀── dashboard
     │                    ▲                          ▲   ▲
     │  terraform apply   │ API token                │   │
     └────── provider ────┘                   agents─┘   └─reverse proxy
                                        (setup key)      (CLI-minted token)
```

## Bootstrap

`internal/provider/e2e_harness_test.go` runs once per test binary:

1. `docker compose up -d --wait` — management, dashboard, signal, relay.
2. Poll `GET /api/instance` until it answers, then check `setup_required`.
3. `POST /api/setup` with `create_pat: true` — creates the owner user and
   returns a plaintext API token. This works because management runs with
   `NB_SETUP_PAT_ENABLED=true`; without it the server ignores `create_pat` and
   the harness has no way to authenticate.
4. Create the fixture groups through the API.
5. Mint a setup key through the API, start the agent containers with it, and
   wait for `peer1`/`peer2`/`peer3` to appear in `GET /api/peers`.
6. Mint a proxy token with the management CLI and start the reverse proxy.
7. Create the fixture network and its domain/subnet/host resources.

Every fixture ID is assigned by the server, so tests reference them through the
harness (`e2eGroupAllID()`, `testPeerID(t, "peer1")`, …) rather than by literal.

`POST /api/setup` succeeds only once per deployment, so the token is cached in
`test/.e2e-state.json` (gitignored) and reused when a later run finds the same
stack still up.

## Running

```sh
# Full acceptance suite; brings the stack up on first use and leaves it running.
TF_ACC=1 go test ./internal/provider/

# Unit tests only — no Docker, no deployment.
go test ./internal/provider/

# Tear the deployment down (and drop its volume) when the suite finishes.
TF_ACC=1 NB_E2E_TEARDOWN=1 go test ./internal/provider/

# Reset a deployment so the next run bootstraps from scratch.
docker compose -p tfnetbird-e2e -f test/compose.yml --profile peers --profile proxy down -v
rm -f test/.e2e-state.json
```

Ports default to `8080` for the management API and `8090` for the dashboard;
override with `NETBIRD_MGMT_HOST_PORT` / `NETBIRD_DASHBOARD_HOST_PORT`.
Image tags default to `latest` and are overridable per component
(`NETBIRD_MANAGEMENT_TAG`, `NETBIRD_DASHBOARD_TAG`, `NETBIRD_CLIENT_TAG`,
`NETBIRD_SIGNAL_TAG`, `NETBIRD_RELAY_TAG`, `NETBIRD_PROXY_TAG`).

### Against a deployment you already have

Set `NB_E2E_MANAGEMENT_URL` and the harness skips Docker entirely, bootstrapping
that server instead. Add `NB_E2E_TOKEN` if it is already set up, and
`NB_E2E_DASHBOARD_URL` to exercise the dashboard checks. Tests that need
registered agents skip when the deployment has none.

```sh
TF_ACC=1 NB_E2E_MANAGEMENT_URL=http://127.0.0.1:18080 go test ./internal/provider/
```

That deployment does not have to be the compose one. Where Docker is unavailable,
the server components build straight out of the pinned netbird module and cover
everything except the agents:

```sh
NB=$(go env GOMODCACHE)/github.com/netbirdio/netbird@<version-from-go.mod>
cp -r "$NB" /tmp/nbsrc && chmod -R u+w /tmp/nbsrc   # build inside the module: it
cd /tmp/nbsrc                                       # carries the replace directives
for c in management signal relay; do go build -o /tmp/nb/netbird-$c ./$c; done
go build -o /tmp/nb/netbird-proxy ./proxy/cmd/proxy
```

Run signal, relay and management (each needs its own `--metrics-port`; they all
default to 9090), point `Datadir` at `/var/lib/netbird` so the `token create` CLI
and the server agree on the store path, then mint a proxy token and start the
proxy exactly as the compose file does.

The client builds the same way (`go build -o /tmp/nb/netbird ./client`), so the
agents can be supplied too: run `agent-entrypoint.sh` once per peer with
`NETBIRD_BIN` pointed at that binary. The client profile lives at a fixed path
rather than under `--config`, so on a single host the peers have to be registered
one at a time, wiping `/var/lib/netbird/default.json` in between. Wrap each one in
`unshare -u --fork sh -c "hostname peerN && exec …"` so it gets its own hostname,
the way `hostname: peerN` does for the containers — without that all three peers
register under the host's name and the harness cannot tell them apart.

## Things worth knowing

- **Geolocation stays enabled.** The posture-check tests create
  `geo_location_check` rules, which management rejects when the GeoLite database
  was never initialized — so the container must be allowed to download it.
- **The agent-network bootstrap test needs a virgin account.** CI runs
  `Test_AgentNetworkSettings_BootstrapViaCluster` in its own step, before the
  rest of the suite bootstraps the account's settings row.
- **A warm stack makes iteration bearable.** Bootstrapping costs minutes, so the
  suite leaves the deployment running unless `NB_E2E_TEARDOWN=1` is set.
- **The agents do not use the image's entrypoint.** `agent-entrypoint.sh` runs
  `netbird up` first and falls back to a foreground `netbird login` when that
  fails to register, because some client builds drop the setup key on the
  daemon's login path. The script documents the three non-obvious couplings that
  make the fallback work; the short version is that the order of daemon start,
  daemon stop and `NB_LOG_FILE=console` all matter.
- **Each agent needs `hostname: peerN`.** The reconnect after a fallback login
  takes the peer name from the system hostname, and the harness looks its peer
  fixtures up by name.
