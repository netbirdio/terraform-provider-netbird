#!/bin/sh
# Entrypoint for the e2e agent containers.
#
# The normal path is the one the shipped image uses: start the daemon, then
# `netbird up` with NB_SETUP_KEY. Some client builds drop the setup key on the
# daemon's login path, though — `up` fails with
#
#   invalid setup-key or no sso information provided, err: invalid UUID length: 0
#
# even when the key is supplied as a flag, an env var or a file. When that
# happens the peer never registers and the harness times out waiting for it, so
# fall back to the client's foreground login, which does honour the key.
#
# Three details make this work, none of them obvious:
#
#   * Only the daemon writes the management URL into the client profile
#     (`--management-url` is ignored when the profile is first created), so the
#     daemon has to have run before a foreground login can reach the right
#     server at all.
#   * `netbird login` picks the foreground path only when its log destination
#     resolves to no file — util.FindFirstLogPath skips "console" and "syslog".
#     The image sets NB_LOG_FILE to "console,/var/log/netbird/client.log", which
#     does contain a file, so the fallback must run with NB_LOG_FILE=console or
#     it takes the daemon path and fails the same way `up` did. It has to be the
#     environment variable rather than --log-file: the client applies env vars
#     over flags (SetFlagsFromEnvVars), so the flag alone gets overwritten.
#   * The daemon must be stopped first regardless, because a reachable daemon
#     wins over the foreground path.
#
# Once the peer is registered it authenticates with its WireGuard key, so the
# reconnect afterwards needs no setup key and simply succeeds.
set -eu

NETBIRD_BIN="${NETBIRD_BIN:-netbird}"

wait_for_daemon() {
    i=0
    while [ "$i" -lt 30 ]; do
        if "$NETBIRD_BIN" status --check live >/dev/null 2>&1; then
            return 0
        fi
        i=$((i + 1))
        sleep 1
    done
    echo "agent-entrypoint: daemon did not become responsive" >&2
    return 1
}

start_daemon() {
    "$NETBIRD_BIN" service run &
    daemon_pid=$!
    wait_for_daemon
}

start_daemon

if "$NETBIRD_BIN" up; then
    echo "agent-entrypoint: registered and connected via 'netbird up'" >&2
    wait "$daemon_pid"
    exit 0
fi

echo "agent-entrypoint: 'netbird up' did not register the peer, falling back to foreground login" >&2
kill "$daemon_pid" 2>/dev/null || true
wait "$daemon_pid" 2>/dev/null || true
sleep 3

# NB_LOG_FILE=console is load-bearing: it is what selects the foreground path.
NB_LOG_FILE=console "$NETBIRD_BIN" login \
    --setup-key "$NB_SETUP_KEY" \
    --hostname "${NB_HOSTNAME:-$(hostname)}" \
    --log-file console
echo "agent-entrypoint: registered via foreground login" >&2

# Reconnect so the peer is live, not merely enrolled. This login carries no
# setup key and works because the peer is now known to the server. The hostname
# has to be repeated: without it this login renames the peer to the container's
# system hostname, and the harness looks its fixtures up by name.
start_daemon
"$NETBIRD_BIN" up --hostname "${NB_HOSTNAME:-$(hostname)}"
echo "agent-entrypoint: connected" >&2
wait "$daemon_pid"
