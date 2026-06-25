#!/usr/bin/env bash
# =============================================================================
# Quark Platform — run-example.sh
# =============================================================================
# Deploys the multi-tenant streaming example via the CLI -> server workflow
# and observes the output for the given duration.
#
# Usage:
#   ./scripts/run-example.sh [DURATION_SECONDS]
#
# Prerequisites:
#   - The Java server is built (mvn install -DskipTests) — make build does this.
#   - The Go CLI binary exists at cli/quarkctl — make build does this.
#   - A NATS server is running on nats://localhost:4222 (or the embedded
#     one is enabled — currently we connect to an external one).
#
# What this script does:
#   1. Start the Quark server in the background.
#   2. Wait for it to be ready (poll /health).
#   3. Deploy example/simple-streaming/system.quark.ts under namespace "alice".
#   4. Sleep for DURATION_SECONDS (default 15).
#   5. Print the first 20 lines of the JSONL output file.
#   6. List deployed systems + nodes.
#   7. Undeploy and shut down the server cleanly.
# =============================================================================
set -euo pipefail

DURATION="${1:-15}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

# ---- Locate artifacts ------------------------------------------------------
BUILD_MODE="${BUILD_MODE:-jvm}"
CLI_BIN="cli/quarkctl"

# Use RUN_MODE if set (the Makefile uses RUN_MODE=jvm|native). Backwards-compat with BUILD_MODE.
RUN_MODE="${RUN_MODE:-${BUILD_MODE:-jvm}}"

# The server is always a Go binary now. RUN_MODE only affects the runtime
# (which the server spawns as a child process).
SERVER_BIN="server/quark-server"
if [ ! -x "$SERVER_BIN" ]; then
    echo "❌ Go server binary not found at $SERVER_BIN — run 'make build' first." >&2
    exit 1
fi
RUN_CMD=("$SERVER_BIN")

if [ ! -x "$CLI_BIN" ]; then
    echo "❌ CLI binary not found at $CLI_BIN — run 'make build' first." >&2
    exit 1
fi

# ---- Start NATS server ------------------------------------------------------
echo "▶ Starting NATS server (background)..."
NATS_BIN=""
for candidate in nats-server /usr/local/bin/nats-server /opt/homebrew/bin/nats-server; do
    if command -v "$candidate" >/dev/null 2>&1; then NATS_BIN="$candidate"; break; fi
done
if [ -z "$NATS_BIN" ]; then
    echo "❌ nats-server not found. Install it: https://docs.nats.io/nats-concepts/what-is-nats/walkthrough_install" >&2
    exit 1
fi
$NATS_BIN > /tmp/quark-nats.log 2>&1 &
NATS_PID=$!
echo "  NATS PID: $NATS_PID"

# Wait for NATS to be ready
for i in $(seq 1 10); do
    if curl -sf http://localhost:8222/varz >/dev/null 2>&1 || nc -z localhost 4222 2>/dev/null; then
        echo "  ✓ NATS ready"
        break
    fi
    sleep 0.5
done

# ---- Prepare state directory -----------------------------------------------
STATE_DIR="$(pwd)/quark-state"
rm -rf "$STATE_DIR"
mkdir -p "$STATE_DIR"

# ---- Start Catalog service -------------------------------------------------
echo "▶ Starting Catalog service (background)..."
CATALOG_BIN="quark-catalog/quark-catalog"
if [ ! -x "$CATALOG_BIN" ]; then
    echo "❌ Catalog binary not found at $CATALOG_BIN — run 'make build-catalog' first." >&2
    exit 1
fi
$CATALOG_BIN -nats nats://localhost:4222 -state "$STATE_DIR" > /tmp/quark-catalog.log 2>&1 &
CATALOG_PID=$!
echo "  Catalog PID: $CATALOG_PID"
sleep 1
echo "  ✓ Catalog ready"

# ---- Start the server ------------------------------------------------------
echo "▶ Starting Quark Go server (background)..."

# Use a dedicated port to avoid clashes with any running instance.
export QUARK_STATE_ROOT="$STATE_DIR"
export RUN_MODE
export BUILD_MODE="$RUN_MODE"   # backwards-compat for any downstream readers


"${RUN_CMD[@]}" \
    > /tmp/quark-server.log 2>&1 &
SERVER_PID=$!

cleanup() {
    echo ""
    echo "⏹ Stopping server (PID $SERVER_PID)..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    echo "⏹ Stopping Catalog (PID $CATALOG_PID)..."
    kill "$CATALOG_PID" 2>/dev/null || true
    wait "$CATALOG_PID" 2>/dev/null || true
    echo "⏹ Stopping NATS (PID $NATS_PID)..."
    kill "$NATS_PID" 2>/dev/null || true
    wait "$NATS_PID" 2>/dev/null || true
}
trap cleanup EXIT

# ---- Wait for readiness ----------------------------------------------------
# The Go control plane exposes /health/live (same path as the Java server
# for CLI/script compat).
echo "⏳ Waiting for server to be ready..."
READY=0
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/health/live >/dev/null 2>&1; then
        echo "✓ Server ready (after ${i}s)"
        READY=1
        break
    fi
    sleep 1
done

if [ "$READY" -ne 1 ]; then
    echo "❌ Server did not become ready in 30s. Server log:" >&2
    cat /tmp/quark-server.log >&2
    exit 1
fi

# ---- Push standard library nodes to the Catalog ----------------------------
# The runtime NEVER compiles node implementations into its binary. Every node
# the example system references (timer, cpu, memory, writer, stream) must be
# pushed to the Catalog before the deploy command, so the data plane can pull
# them at deploy time via the registry.node.pull NATS subject.
#
# This is the docker-image model: build → push → pull → run. The script does
# not push the 5 TypeScript nodes (stdout, json-parse, map, conditional, fetch)
# because the simple-streaming example doesn't reference them — but they would
# be pushed the same way if a system used them.
echo "▶ Building + pushing standard library nodes to the Catalog..."
NODES_TO_PUSH=(
    "quark/time/schedule/timer:v1"
    "quark/system/cpu/profile:v1"
    "quark/system/memory/profile:v1"
    "quark/io/file/write:v1"
    "quark/stream/sse/broadcast:v1"
)
for uri in "${NODES_TO_PUSH[@]}"; do
    echo "  • $uri"
    ./$CLI_BIN node build "$uri" 2>&1 | sed 's/^/    /'
    ./$CLI_BIN node push  "$uri" 2>&1 | sed 's/^/    /' || {
        echo "❌ node push failed for $uri" >&2
        exit 1
    }
done
echo "  ✓ All ${#NODES_TO_PUSH[@]} nodes pushed to the Catalog."

# ---- Deploy the example ----------------------------------------------------
echo ""
echo "▶ Deploying example under namespace 'alice'..."
echo "  (the data plane will pull each node from the Catalog on first use)"
./$CLI_BIN apply -f example/simple-streaming/system.quark.ts -n alice

# ---- Show node pull activity from the data plane log -----------------------
# The data plane logs every catalog pull at INFO level:
#   "Loaded node <uri> from catalog (type=<type>, <n> bytes)"
# Surface those lines so the user can see the docker-image-style pull flow.
echo ""
echo "──────────────────────────── NODE PULL ACTIVITY ──────────────────────"
DP_LOG="$STATE_DIR/dataplane-logs/dataplane-shared.log"
if [ -f "$DP_LOG" ]; then
    grep "Loaded node.*from catalog" "$DP_LOG" | sed 's/^.*Loaded node/  Pulled node/' | sort -u
else
    echo "  (data plane log not found at $DP_LOG)"
fi

# ---- Observe ---------------------------------------------------------------
echo ""
echo "⏱ Observing for ${DURATION}s..."
sleep "$DURATION"

# ---- Print results ---------------------------------------------------------
echo ""
echo "──────────────────────────── NAMESPACES ────────────────────────────"
./$CLI_BIN get namespaces

echo ""
echo "──────────────────────────── SYSTEMS ────────────────────────────"
./$CLI_BIN get systems -n alice

echo ""
echo "──────────────────────────── NODES ──────────────────────────────"
./$CLI_BIN get nodes -n alice -s monitor

echo ""
echo "──────────────────────────── EVENTS (last 10) ───────────────────"
./$CLI_BIN get events -n alice -s monitor --limit 10

echo ""
echo "──────────────────────────── JSONL OUTPUT ───────────────────────"
JSONL="example/simple-streaming/json/system-monitor.jsonl"
if [ -f "$JSONL" ]; then
    LINES=$(wc -l < "$JSONL")
    echo "Wrote $LINES lines to $JSONL. First 5 lines:"
    head -5 "$JSONL"
else
    echo "(no JSONL output yet — check that the timer + profilers ran)"
fi

# ---- Undeploy --------------------------------------------------------------
echo ""
echo "⏹ Undeploying..."
./$CLI_BIN delete system monitor -n alice

echo ""
echo "✓ Done."
