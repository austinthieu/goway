#!/usr/bin/env bash
# Runs the gateway load test used to produce docs/benchmarks/*.txt.
#
# Requires: go, hey (go install github.com/rakyll/hey@latest)
#
# Usage: ./scripts/loadtest.sh
# Starts 3 demo-backends + the gateway (with rate limiting effectively
# disabled via testdata/loadtest-config.yaml, since this measures proxy
# throughput, not the rate limiter), runs a 15s steady-state load test,
# then a second run that kills backend-2 mid-test to measure failover
# under load. Cleans up all spawned processes on exit.
set -euo pipefail
cd "$(dirname "$0")/.."

BIN_DIR=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$BIN_DIR"' EXIT

go build -o "$BIN_DIR/demo-backend" ./cmd/demo-backend
go build -o "$BIN_DIR/gateway" ./cmd/gateway

"$BIN_DIR/demo-backend" -port 8081 -name backend-1 &
"$BIN_DIR/demo-backend" -port 8082 -name backend-2 &
"$BIN_DIR/demo-backend" -port 8083 -name backend-3 &
"$BIN_DIR/gateway" -config testdata/loadtest-config.yaml &
sleep 1

echo "=== Steady-state: 15s @ 50 concurrent ==="
hey -z 15s -c 50 http://localhost:8080/

echo
echo "=== Failover under load: kill backend-2 mid-test ==="
hey -z 12s -c 50 http://localhost:8080/ &
HEY_PID=$!
sleep 3
pkill -9 -f "demo-backend -port 8082"
wait $HEY_PID
