# Architecture

## Request flow

```
                    ┌─────────────────────────────────────────────┐
                    │                  Gateway                     │
  client request    │                                               │
 ─────────────────► │  1. rate limit check (per-client token bucket)│
                    │       │ 429 if empty                          │
                    │       ▼                                       │
                    │  2. balancer.Next(pool)                       │
                    │     picks from Eligible backends only         │
                    │       │ 503 if none eligible                  │
                    │       ▼                                       │
                    │  3. breaker.Reserve()                         │
                    │     claims a HalfOpen trial slot if needed    │
                    │       │ 503 if lost the race                  │
                    │       ▼                                       │
                    │  4. reverse proxy -> chosen backend            │
                    │     records latency + status into metrics     │
                    └─────────────────────────────────────────────┘
```

## Background goroutines

Two goroutines run per backend, independent of the request path above:

- **Active health checker** (`internal/healthcheck`): probes `/healthz` on a
  timer, writes `Backend.healthy`.
- **Metrics poller** (`internal/metrics.PollBackends`): snapshots
  `ActiveHealthy()` and `Breaker.State()` into Prometheus gauges once a
  second.

The circuit breaker itself has no background goroutine — it's driven
synchronously by the request path's own success/failure observations
(`ModifyResponse`/`ErrorHandler` hooks on the backend's reverse proxy), which
is exactly why it reacts faster than the timer-driven active checker.

## Config reload lifecycle

`proxy.Gateway` holds an `atomic.Pointer[snapshot]`, where a snapshot bundles
a pool, a balancer, and a rate limiter built together from one `Config`.
`Reload`:

1. Builds an entirely new pool/balancer/limiter from the new config.
2. Starts that snapshot's health-check and metrics-poll goroutines under a
   fresh, cancelable context.
3. Atomically swaps the pointer.
4. Cancels the *previous* snapshot's goroutines only after the swap.

A request already in flight holds a reference to the snapshot it started
with (via a single `g.current.Load()` at the top of `ServeHTTP`), so it
completes against consistent state even if a reload happens mid-request.
`SIGHUP` triggers this; `SIGTERM`/`SIGINT` triggers `http.Server.Shutdown`
instead, which drains in-flight requests before the process exits.

## Package dependency graph

```
cmd/gateway
    │
    ▼
internal/proxy ──► internal/balancer ──► internal/backendpool ──► internal/circuitbreaker
    │                                            │                          │
    ├──► internal/ratelimit ──► pkg/tokenbucket   └──► internal/config       │
    │                                                                        │
    └──► internal/metrics                                                   │
                                                                              │
internal/healthcheck ────────────────────────────────────────────────────────┘
```

No cycles. `pkg/tokenbucket` is the only package with zero dependency on the
rest of the module — it's a standalone, reusable rate limiter.
