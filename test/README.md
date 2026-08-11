# Acceptance test deployment

The acceptance suite runs against a real NetBird deployment that the tests
themselves start. There is no compose file and no seeded database: the harness
in `internal/provider/e2e_stack_test.go` creates a private Docker network,
starts the components on it, and onboards the account through the product's own
setup endpoint.

## What runs

```text
                    POST /api/setup (create_pat)
    go test  ─────────────────────────────────────┐
       │                                          ▼
       │   terraform ──▶ provider ──▶ HTTP ──▶ netbird-server (netbird.local:8080)
       │                                          │  management + signal + relay
       │                                          │  + STUN + embedded IdP, sqlite
       │                                          │
       ├── dashboard  ────────────────────────────┤  NETBIRD_MGMT_API_ENDPOINT
       ├── reverse proxy  ────────────────────────┤  proxy access token, self-signed cert
       └── peer1, peer2, peer3  ──────────────────┘  netbird agents, setup key
```

`netbird-server` is the combined server: one process serving the management API,
signal, relay, STUN and the embedded identity provider. Its alias on the network,
`netbird.local`, is also the deployment's domain, so peers, the reverse proxy
cluster and the agent-network endpoints all agree on one name.

The reverse proxy and the agents start lazily, on the first test that needs
them, so a run that only touches the API does not pay for them.

## Images

`test/build-images.sh` builds the server, reverse proxy and agent images from the
netbird version pinned in `go.mod`, using the Dockerfiles in that module's source
and the module directory as the build context. The server under test is therefore
the same revision the provider's client library was generated from, which a
published `latest` cannot guarantee.

Images are tagged with the pinned version (`netbird-server:v0.76.2-0.2026...`),
and an existing tag is reused, so only a `go.mod` bump pays for a rebuild.

The dashboard lives in another repository and is used as a published image.

| Variable                 | Effect                                                                       |
| ------------------------ | ---------------------------------------------------------------------------- |
| `NB_E2E_SERVER_IMAGE`    | Use this image instead of building. A value with a `/` is treated as a        |
| `NB_E2E_PROXY_IMAGE`     | registry reference and pulled; a bare tag is built under that name.           |
| `NB_E2E_CLIENT_IMAGE`    |                                                                              |
| `NB_E2E_DASHBOARD_IMAGE` | Dashboard image, default `netbirdio/dashboard:latest`.                        |
| `NB_E2E_REBUILD_IMAGES`  | `1` rebuilds even when the tag already exists locally.                        |

## Bootstrap

Nothing is written into the management store directly. The account comes up the
way an installation does:

1. `GET /api/instance` until it serves, then read `setup_required`.
2. `POST /api/setup` with `create_pat`, which creates the owner user and returns
   a plaintext API token. This needs `NB_SETUP_PAT_ENABLED=true`, which the
   harness sets on the server container.
3. Every fixture (groups, the network and its resources, setup keys) is created
   through the public API with that token.
4. The agents register with a setup key minted through the API, so the peers the
   tests address are peers a real device created.
5. The reverse proxy registers with an access token minted through the server's
   own `token create` CLI, and serves a self-signed wildcard certificate: the
   deployment's domain does not resolve publicly, so ACME could only ever fail.

A change to onboarding, token issuance or the API contract therefore fails the
bootstrap instead of silently diverging from a hand-written seed.

## Running

The harness is compiled only under the `e2e` build tag. Without it the Docker
client is not linked into the test binary at all — `e2e_disabled_test.go` stands
in for the harness and every acceptance test skips — so the default `go test`
cannot start a container even by accident. `TF_ACC` is terraform-plugin-testing's
own switch and is still honoured, so a tagged run without it skips rather than
spending minutes on a deployment for tests that would then skip anyway.

```sh
# Everything, against a deployment the run starts itself. Needs Docker.
TF_ACC=1 go test -tags e2e ./internal/provider/

# Unit and integration tests only. No Docker, no deployment, no Terraform CLI.
go test ./internal/provider/
```

That the tag really does keep Docker out is checkable:

```sh
go list -deps -test ./internal/provider/ | grep -c testcontainers            # 0
go list -tags e2e -deps -test ./internal/provider/ | grep -c testcontainers  # 9
```

| Variable                | Effect                                                                  |
| ----------------------- | ----------------------------------------------------------------------- |
| `NB_E2E_MANAGEMENT_URL` | Run against a deployment that is already up and skip Docker entirely.   |
| `NB_E2E_TOKEN`          | API token for that deployment; required once it is already set up.      |
| `NB_E2E_DASHBOARD_URL`  | Dashboard URL for that deployment.                                      |
| `NB_E2E_KEEP_STACK`     | `1` leaves the containers running after the suite, for inspection.      |
| `NB_E2E_DISABLE_GEOLOCATION` | `1` starts the server without the GeoLite download, for a machine that cannot reach it. The geolocation posture checks then fail. |

Tests that need agents or a reverse proxy skip when pointed at a deployment that
has none, so an external management server does not have to provide them.

## Notes

- The suite runs the components from the pinned module, so a server-side change
  that breaks the provider shows up as soon as `go.mod` moves, not after a
  release.
- Geolocation is deliberately left enabled. The posture-check tests create
  location rules that management can only evaluate with the GeoLite database
  loaded, so disabling the download would turn them into false passes.
- The relay is multiplexed over WebSocket on the server's single listener and
  there is no QUIC listener, so the proxy and the agents are pinned to
  `NB_RELAY_TRANSPORT=ws`. With the default QUIC transport the relay link flaps
  and the proxy's cluster never settles.
- Peers are named through `NB_HOSTNAME` and the container's own hostname: the
  first names the peer, the second is what it reports as its hostname, and tests
  assert on both.
- The agents run `test/agent-entrypoint.sh` instead of the image entrypoint.
  `netbird up` alone does not register a peer against this server: the daemon
  probes management with an empty setup key first, management answers
  `InvalidArgument`, and the client treats that as fatal before it ever tries the
  key it was given. The script falls back to a foreground `netbird login
  --setup-key`, and its comments spell out the three couplings that make the
  fallback work.
- Containers are removed when the suite ends. A bootstrap failure carries the
  relevant container's logs in the error, because by the time the run reports the
  failure the container is gone.
