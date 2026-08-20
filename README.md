# goway

An HTTP load balancer / API gateway written from scratch in Go: routes
traffic across a pool of backends, detects and routes around failures
automatically, and enforces per-client rate limits — with zero-downtime
config reload.

## Features

- **Reverse proxy** across N configurable backends
- **Three load balancing strategies**, hot-swappable via config:
  round-robin, least-connections, and smooth (Nginx-style) weighted
  round-robin
- **Circuit breaker per backend** (Closed → Open → HalfOpen), reacting to
  live-traffic failures (connection errors or 5xx) independently of and
  faster than active health checks
- **Active health checking** — periodic `/healthz` probes per backend
- **Per-client rate limiting** — token bucket, keyed by `X-API-Key` header
  or source IP
- **Prometheus metrics** on a separate admin port (`:9090/metrics`):
  request counts/latency, rate-limit rejections, backend health, breaker
  state
- **Zero-downtime config reload** via `SIGHUP` — new backends picked up,
  in-flight requests unaffected, a bad config is rejected without
  disrupting service
- **Graceful shutdown** on `SIGTERM`/`SIGINT` — drains in-flight requests
  before exiting

See `CONTEXT.md` for the project's domain vocabulary and `docs/adr/` /
`docs/design-decisions.md` for why things are built this way.

## Quick start

```bash
go run ./cmd/demo-backend -port 8081 -name backend-1 &
go run ./cmd/demo-backend -port 8082 -name backend-2 &
go run ./cmd/demo-backend -port 8083 -name backend-3 &
go run ./cmd/gateway -config testdata/config.yaml
```

```bash
curl localhost:8080/          # proxied, round-robin across the 3 backends
curl localhost:9090/metrics   # Prometheus metrics, separate admin port
```

Reload config without downtime:

```bash
# edit testdata/config.yaml (add a backend, change strategy, ...)
kill -HUP $(pgrep -f 'cmd/gateway|/gateway ')
```

### Docker

```bash
docker compose up
```

Starts 3 demo backends + the gateway with one command (gateway on
`:8080`/`:9090`). **Note**: this repo's Dockerfile/compose were authored
against Docker's documented behavior but not run locally — this dev
environment doesn't have Docker installed. Please open an issue if
something doesn't build as expected.

## Demo

See `docs/demo.md` for a real, captured terminal walkthrough: round-robin
across 3 backends, a backend crash caught by the active health checker,
the rate limiter kicking in on a burst, a zero-downtime `SIGHUP` reload
adding a 4th backend live, and the resulting Prometheus metrics.

To turn that into a GIF (e.g. for a resume/README banner), the walkthrough
is scripted at `scripts/demo.sh` and wired up for
[VHS](https://github.com/charmbracelet/vhs) at `docs/demo.tape`:

```bash
# Ubuntu/Debian:
sudo apt install -y ffmpeg
# ttyd isn't in apt — grab a static binary from
# https://github.com/tsl0922/ttyd/releases (e.g. ttyd.x86_64), chmod +x,
# and put it on PATH.
go install github.com/charmbracelet/vhs@latest   # or a release binary

vhs docs/demo.tape   # -> docs/demo.gif
```

Then embed it at the top of this README with `![demo](docs/demo.gif)`.

## Architecture

See `docs/architecture.md` for the request-flow and goroutine diagrams,
and the config-reload lifecycle.

## Testing

```bash
go test ./...                          # unit + integration tests
go test -bench=. -benchmem ./...       # hot-path allocation benchmarks
```

`internal/proxy/integration_test.go` spins up real `httptest.Server` fake
backends behind a real `Gateway` and proves failover end-to-end: killing
one backend resolves to 100% traffic on the survivor, driven by the
circuit breaker (not the slower health-check timer) in well under a
second.

Note: `go test -race` isn't available in this dev environment (no C
compiler for cgo); the concurrency-sensitive packages (`circuitbreaker`,
`tokenbucket`, `balancer`) do have dedicated concurrent-access tests, just
run without the race detector here.

## Benchmarks

Real numbers from `scripts/loadtest.sh` (raw `hey` output in
`docs/benchmarks/`), 3 backends, 50 concurrent clients:

| Scenario | Requests/sec | p50 | p99 | Result |
|---|---|---|---|---|
| Steady state (15s) | **18,200** | 2.4ms | 8.0ms | 273,055/273,055 succeeded |
| Backend `kill -9`'d mid-test (12s) | 17,834 | 2.4ms | 8.3ms | 214,025/214,045 succeeded (**99.99%**), 20 failed during the failover window |

Hot-path allocation benchmarks (`go test -bench=. -benchmem`):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `RoundRobin.Next` | 33 | 24 | 1 (from the eligible-backend filter slice) |
| `tokenbucket.Bucket.Allow` | 55 | 0 | 0 |

## Resume bullet

> Built *goway*, a Go HTTP load balancer/API gateway from scratch
> supporting round-robin, least-connections, and weighted routing across N
> backends, with active/passive (circuit breaker) health detection, a
> half-open recovery protocol, and per-client token-bucket rate limiting.
> Verified zero-downtime config reload and 99.99% request success under
> sustained 18k req/s load through an abrupt backend failure in
> integration and load testing; instrumented with Prometheus metrics and
> packaged with Docker for a one-command demo.
