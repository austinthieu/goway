# gobalance

An HTTP load balancer / API gateway that routes traffic across a pool of backend servers, detecting and routing around failures automatically.

## Language

**Backend**:
An upstream server the gateway routes requests to. Identified by URL, carries a static weight and live eligibility state.
_Avoid_: Server, upstream, target (use Backend consistently)

**Healthy / Unhealthy**:
A Backend's status as reported by the *active* health checker's periodic `/healthz` probes only. Distinct from the circuit breaker's state — see Eligible.
_Avoid_: Up/down, live/dead (reserve these for casual conversation, not code or docs)

**Circuit Breaker**:
A per-Backend state machine (Closed → Open → HalfOpen → Closed) that reacts to failures observed on live proxied traffic, independent of and faster than the active health checker. Closed allows traffic; Open blocks it entirely; HalfOpen allows a limited number of trial requests to test recovery.
_Avoid_: Passive health check (this project has no separate passive-health mechanism — the breaker is the passive-failure signal)

**Failure** (Circuit Breaker context):
A proxied request that either errors at the connection level (dial failure, timeout) or receives an HTTP 5xx from the Backend. Both count identically toward the breaker's trip threshold.
_Avoid_: Error (too broad — a 4xx from a backend is not a Failure, it's a valid response)

**Eligible**:
Whether a Backend may currently receive traffic: true only when the Backend is Healthy *and* its Circuit Breaker is not Open. The Balancer only selects from eligible Backends.
_Avoid_: Available, active

**Balancer / Strategy**:
The pluggable algorithm that picks a Backend for each request from the eligible set. The three canonical strategies are **RoundRobin**, **LeastConnections**, and **WeightedRoundRobin** (smooth/interleaved, not a flattened weight list).
_Avoid_: Algorithm, policy (use Strategy)

**Client** (rate limiting):
The identity a rate-limit quota is tracked against: the `X-API-Key` header if present, otherwise the request's source IP.
_Avoid_: User, caller

**Admin Endpoint**:
The separate listener (distinct port from the public-facing proxy listener) that serves `/metrics` and other operational routes not meant to be reachable by proxied traffic.
_Avoid_: Metrics port (Admin Endpoint may host more than metrics later)

**Token Bucket**:
The rate-limiting mechanism backing each Client's quota: a bucket with a fixed Capacity (max burst size) that refills continuously at the Refill Rate (steady-state requests/second). A request is admitted only if a token is available.
_Avoid_: Quota, allowance
