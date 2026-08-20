# Demo walkthrough

A real terminal transcript, captured end-to-end against a running gateway
(3 backends started via `demo-backend`, gateway via `testdata/config.yaml`).
Not staged output — every command below was actually run.

![goway demo](demo.gif)

The recording above is this same walkthrough, produced by
`scripts/demo.sh` via `docs/demo.tape` (see the README for how to
regenerate it).

## 1. Round-robin across 3 backends

```
$ curl localhost:8080/
hello from backend-2 :8082 (request #1)
$ curl localhost:8080/
hello from backend-3 :8083 (request #1)
$ curl localhost:8080/
hello from backend-1 :8081 (request #1)
$ curl localhost:8080/
hello from backend-2 :8082 (request #2)
```

## 2. Backend crashes — active health check catches it

```
$ kill -9 27368   # simulate backend-2 crashing
(waiting for the active health checker to detect it...)
$ curl localhost:8080/
hello from backend-3 :8083 (request #2)
$ curl localhost:8080/
hello from backend-1 :8081 (request #2)
$ curl localhost:8080/
hello from backend-3 :8083 (request #3)
$ curl localhost:8080/
hello from backend-1 :8081 (request #3)
```

backend-2 drops out of rotation cleanly — every subsequent request lands on
backend-1 or backend-3. (For a backend that stays *up* but starts
returning 5xx instead of crashing outright, it's the circuit breaker that
catches it — see `docs/benchmarks/failover-under-load.txt` for that
scenario under real load, where the breaker resolved a `kill -9` in well
under a second.)

## 3. Per-client rate limiting (token bucket: capacity 20, refill 10/s)

```
$ for i in $(seq 1 25); do curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/; done
200
200
200
200
200
200
200
200
200
200
200
200
200
200
200
200
200
200
200
200
429
200
429
429
429
```

First 20 requests succeed (the bucket's full capacity), then 429s — with
one extra 200 slipping through mid-burst as the bucket earns back a
fraction of a token at the 10/s refill rate. This is the token bucket
working exactly as designed, not a bug: real bursts get real partial
refills.

## 4. Zero-downtime config reload — add a 4th backend via SIGHUP

```
$ cat >> testdata/config.yaml << EOF
  - url: "http://localhost:8084"
    weight: 1
EOF
$ kill -HUP 27442
```

(backend-4 process started right after)

```
$ curl localhost:8080/
hello from backend-3 :8083 (request #15)
$ curl localhost:8080/
hello from backend-4 :8084 (request #1)
$ curl localhost:8080/
hello from backend-1 :8081 (request #14)
$ curl localhost:8080/
hello from backend-3 :8083 (request #16)
```

No restart, no dropped connections, no client-visible disruption — backend-4
is in rotation within one reload cycle.

## 5. Prometheus metrics, live

```
$ curl -s localhost:9090/metrics | grep "^goway_" | head -12
goway_backend_healthy{backend="http://localhost:8081"} 1
goway_backend_healthy{backend="http://localhost:8082"} 0
goway_backend_healthy{backend="http://localhost:8083"} 1
goway_backend_healthy{backend="http://localhost:8084"} 1
goway_circuit_breaker_state{backend="http://localhost:8081"} 0
goway_circuit_breaker_state{backend="http://localhost:8082"} 0
goway_circuit_breaker_state{backend="http://localhost:8083"} 0
goway_circuit_breaker_state{backend="http://localhost:8084"} 0
goway_ratelimit_rejections_total 4
goway_request_duration_seconds_bucket{backend="http://localhost:8081",le="0.005"} 14
goway_request_duration_seconds_bucket{backend="http://localhost:8081",le="0.01"} 14
goway_request_duration_seconds_bucket{backend="http://localhost:8081",le="0.025"} 14
```

This snapshot is a nice illustration of the two-mechanism health model on
its own: `backend_healthy` is `0` for the crashed backend-2 (caught by the
*active checker*), while every `circuit_breaker_state` reads `0` (Closed) —
because backend-2 was killed outright, not returning 5xx, so the breaker
never had a live-traffic failure to react to. Exactly the separation of
concerns described in `docs/adr/0001-two-mechanism-health-model.md`.
