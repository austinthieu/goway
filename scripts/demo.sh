#!/usr/bin/env bash
# Runs the full docs/demo.md walkthrough against a real gateway, with
# pacing (`sleep`) tuned for recording (e.g. via VHS, see docs/demo.tape).
# Cleans up every process it starts on exit.
set -uo pipefail
cd "$(dirname "$0")/.."

WORK=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$WORK"' EXIT

pause() { sleep "${1:-1.5}"; }
step() { echo; echo "# $1"; pause 1; }

go build -o "$WORK/demo-backend" ./cmd/demo-backend
go build -o "$WORK/gateway" ./cmd/gateway

cp testdata/config.yaml "$WORK/config.yaml"

step "starting 3 backends + gateway"
"$WORK/demo-backend" -port 8081 -name backend-1 > "$WORK/backend-1.log" 2>&1 &
"$WORK/demo-backend" -port 8082 -name backend-2 > "$WORK/backend-2.log" 2>&1 &
disown # suppress bash's own job-control "Killed" notice when we kill -9 it below
"$WORK/demo-backend" -port 8083 -name backend-3 > "$WORK/backend-3.log" 2>&1 &
"$WORK/gateway" -config "$WORK/config.yaml" > "$WORK/gateway.log" 2>&1 &
GW_PID=$!
pause 2

step "round-robin across 3 backends"
for i in 1 2 3 4; do
  echo '$ curl localhost:8080/'
  curl -s localhost:8080/
  pause 0.6
done

step "backend-2 crashes — active health check catches it"
B2_PID=$(pgrep -f "demo-backend -port 8082")
echo "\$ kill -9 $B2_PID"
kill -9 "$B2_PID"
echo "(waiting for the active health checker...)"
pause 3
for i in 1 2 3 4; do
  echo '$ curl localhost:8080/'
  curl -s localhost:8080/
  pause 0.6
done

step "per-client rate limiting (capacity 20, refill 10/s)"
echo '$ for i in $(seq 1 25); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/; done'
for i in $(seq 1 25); do
  curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/
done
pause 3

step "zero-downtime reload: add backend-4 via SIGHUP"
"$WORK/demo-backend" -port 8084 -name backend-4 > "$WORK/backend-4.log" 2>&1 &
echo '$ echo backend-4 >> config.yaml && kill -HUP '"$GW_PID"
cat >> "$WORK/config.yaml" << 'EOF'
  - url: "http://localhost:8084"
    weight: 1
EOF
kill -HUP "$GW_PID"
pause 2
for i in 1 2 3 4; do
  echo '$ curl localhost:8080/'
  curl -s localhost:8080/
  pause 0.6
done

step "Prometheus metrics"
echo '$ curl -s localhost:9090/metrics | grep "^gobalance_" | head -9'
curl -s localhost:9090/metrics | grep "^gobalance_" | head -9

pause 2
echo
echo "# done"
