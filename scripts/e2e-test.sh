#!/usr/bin/env bash
set -euo pipefail

# E2E test for tunnelman v2 CLI
# Tests daemon lifecycle and tunnel CRUD operations via the binary.
# NOTE: Does not test actual SSH connections (no start/stop of tunnels).

command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required"; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BINARY="$PROJECT_ROOT/build/tunnelman"
TEST_TMPDIR="$(mktemp -d)"
SOCKET="$TEST_TMPDIR/tunnelman.sock"
CONFIG="$TEST_TMPDIR/config.yaml"

cleanup() {
    # Stop daemon if running
    "$BINARY" daemon stop --socket "$SOCKET" 2>/dev/null || true
    rm -rf "$TEST_TMPDIR"
}
trap cleanup EXIT

echo "=== tunnelman E2E test ==="

# 1. Build
echo "--- Build ---"
(cd "$PROJECT_ROOT" && make build)
test -x "$BINARY" || { echo "FAIL: binary not found"; exit 1; }
echo "OK: binary built"

# 2. Daemon start (foreground, background it)
echo "--- Daemon start ---"
"$BINARY" daemon start --foreground --socket "$SOCKET" --config "$CONFIG" &
DAEMON_PID=$!

# Wait for daemon to be reachable
for i in $(seq 1 50); do
    if "$BINARY" daemon status --socket "$SOCKET" --json 2>/dev/null | grep -q '"success":true'; then
        break
    fi
    sleep 0.1
done

# Verify daemon is running
STATUS=$("$BINARY" daemon status --socket "$SOCKET" --json 2>/dev/null)
echo "$STATUS" | jq -e '.success == true' > /dev/null || { echo "FAIL: daemon not running"; exit 1; }
echo "OK: daemon started (PID=$DAEMON_PID)"

# 3. Add tunnel (ID is auto-generated, capture it from JSON output)
echo "--- Add tunnel ---"
ADD_OUT=$("$BINARY" add --socket "$SOCKET" --config "$CONFIG" --json \
    --name "Web Server" --type local \
    --ssh-host bastion --local-port 8080 --remote-host db --remote-port 5432)
WEB_ID=$(echo "$ADD_OUT" | jq -r '.data.id')
test -n "$WEB_ID" || { echo "FAIL: could not get tunnel ID"; echo "$ADD_OUT"; exit 1; }
echo "OK: tunnel added (ID=$WEB_ID)"

# 4. List tunnels (JSON)
echo "--- List tunnels ---"
LIST=$("$BINARY" list --socket "$SOCKET" --json)
echo "$LIST" | jq -e '.data.tunnels | length == 1' > /dev/null || { echo "FAIL: expected 1 tunnel"; echo "$LIST"; exit 1; }
echo "$LIST" | jq -e --arg id "$WEB_ID" '.data.tunnels[0].id == $id' > /dev/null || { echo "FAIL: tunnel ID mismatch"; exit 1; }
echo "OK: list shows 1 tunnel"

# 5. Status tunnel (JSON)
echo "--- Status tunnel ---"
TSTATUS=$("$BINARY" status "$WEB_ID" --socket "$SOCKET" --json)
echo "$TSTATUS" | jq -e --arg id "$WEB_ID" '.data.id == $id' > /dev/null || { echo "FAIL: status ID mismatch"; exit 1; }
echo "$TSTATUS" | jq -e '.data.status == "stopped"' > /dev/null || { echo "FAIL: expected stopped"; exit 1; }
echo "OK: tunnel status is stopped"

# 6. Edit tunnel
echo "--- Edit tunnel ---"
"$BINARY" edit "$WEB_ID" --socket "$SOCKET" --config "$CONFIG" --name "Web Edited"
EDITED=$("$BINARY" status "$WEB_ID" --socket "$SOCKET" --json)
echo "$EDITED" | jq -e '.data.name == "Web Edited"' > /dev/null || { echo "FAIL: edit name mismatch"; exit 1; }
echo "OK: tunnel edited"

# 7. Remove tunnel
echo "--- Remove tunnel ---"
"$BINARY" rm "$WEB_ID" --socket "$SOCKET" --config "$CONFIG"
LIST2=$("$BINARY" list --socket "$SOCKET" --json)
echo "$LIST2" | jq -e '.data.tunnels | length == 0' > /dev/null || { echo "FAIL: expected 0 tunnels"; exit 1; }
echo "OK: tunnel removed"

# 8. Profile operations
echo "--- Profile operations ---"
"$BINARY" profile create dev --socket "$SOCKET" --config "$CONFIG" --description "Dev profile"
PROFILES=$("$BINARY" profile list --socket "$SOCKET" --json)
echo "$PROFILES" | jq -e '.data.profiles | length == 1' > /dev/null || { echo "FAIL: expected 1 profile"; exit 1; }
"$BINARY" profile rm dev --socket "$SOCKET" --config "$CONFIG"
echo "OK: profile create/list/remove"

# 9. Daemon status
echo "--- Daemon status ---"
DS=$("$BINARY" daemon status --socket "$SOCKET" --json)
echo "$DS" | jq -e '.data.pid > 0' > /dev/null || { echo "FAIL: daemon PID"; exit 1; }
echo "OK: daemon status"

# 10. Daemon stop
echo "--- Daemon stop ---"
"$BINARY" daemon stop --socket "$SOCKET"
sleep 0.5
if "$BINARY" daemon status --socket "$SOCKET" --json 2>/dev/null | grep -q '"success":true'; then
    echo "FAIL: daemon still running after stop"
    exit 1
fi
echo "OK: daemon stopped"

echo ""
echo "=== All E2E tests passed ==="
