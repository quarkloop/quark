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
SERVER_JAR="quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar"
CLI_BIN="cli/quarkctl"

if [ ! -f "$SERVER_JAR" ]; then
    echo "❌ Server jar not found at $SERVER_JAR — run 'make build' first." >&2
    exit 1
fi
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

cleanup_nats() {
    kill "$NATS_PID" 2>/dev/null || true
    wait "$NATS_PID" 2>/dev/null || true
}

# ---- Start the server ------------------------------------------------------
echo "▶ Starting Quark server (background)..."
STATE_DIR="$(pwd)/quark-state"
rm -rf "$STATE_DIR"
mkdir -p "$STATE_DIR"

# Use a dedicated port to avoid clashes with any running instance.
export QUARK_STATE_ROOT="$STATE_DIR"
java -jar "$SERVER_JAR" \
    -Dquarkus.http.port=8080 \
    > /tmp/quark-server.log 2>&1 &
SERVER_PID=$!

cleanup() {
    echo ""
    echo "⏹ Stopping server (PID $SERVER_PID)..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    echo "⏹ Stopping NATS (PID $NATS_PID)..."
    kill "$NATS_PID" 2>/dev/null || true
    wait "$NATS_PID" 2>/dev/null || true
}
trap cleanup EXIT

# ---- Wait for readiness ----------------------------------------------------
echo "⏳ Waiting for server to be ready..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
        echo "✓ Server ready (after ${i}s)"
        break
    fi
    sleep 1
done

if ! curl -sf http://localhost:8080/health/live >/dev/null 2>&1; then
    echo "❌ Server did not become ready in 30s. Server log:" >&2
    cat /tmp/quark-server.log >&2
    exit 1
fi

# ---- Deploy the example ----------------------------------------------------
echo "▶ Deploying example under namespace 'alice'..."
./$CLI_BIN apply -f example/simple-streaming/system.quark.ts -n alice

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
