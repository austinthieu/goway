# Design decisions

Short-form tradeoff notes. See `docs/adr/` for decisions formal enough to
warrant a standalone ADR; this file covers the smaller ones.

## Token bucket over sliding-window log

A sliding-window log (store every request's timestamp, count how many fall
in the last N seconds) is more precise but costs O(requests) memory per
client. A token bucket is O(1) memory per client and naturally allows
controlled bursts (up to `capacity`) rather than a hard step function at
the window boundary — better fit for a gateway with potentially many
concurrent clients.

## Circuit breaker HalfOpen trial instead of immediate full retry

Once a backend might have recovered, sending it full production traffic
immediately risks a thundering herd back onto a server that's barely back
up. A single (configurable) trial request lets the breaker confirm
recovery cheaply before reopening the floodgates.

## Atomics vs. mutex, chosen per field-count

Single, independently-meaningful values on the request hot path
(`RoundRobin.counter`, `Backend.healthy`, `Backend.activeConns`) use
`atomic.*` — no lock contention on every request. Anything that mutates
several related fields as one unit (`CircuitBreaker`'s state + failure
count + timer; `WeightedRoundRobin`'s per-backend running weight + winner
selection) uses a `sync.Mutex` instead, since an atomic swap can't express
"these three fields change together or not at all." See `CONTEXT.md` and
the packages' own doc comments for the specific reasoning per field.

## Config reload: build-then-swap, never mutate in place

`Gateway.Reload` builds a brand new pool/balancer/limiter off to the side,
validates it (a parse error keeps the old config running — verified live,
see `docs/benchmarks/`), and only then does one atomic pointer swap. The
alternative — mutating the live config struct's fields in place — would
require a lock held across every request's config read, on the hot path,
for a feature (reload) that fires maybe once an hour. The chosen approach
means the hot path never takes a reload-related lock at all.

## Rate limiter's two-level locking

`ratelimit.Limiter.buckets` is a `map[string]*tokenbucket.Bucket` guarded
by its own mutex, even though each `*tokenbucket.Bucket` already has an
internal lock. Map access itself isn't goroutine-safe in Go — the map's
mutex protects "does this client have a bucket yet," the bucket's own
mutex protects "how many tokens does it have right now." Two different
questions, two different locks.
