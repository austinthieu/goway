// Command gateway is the HTTP load balancer / API gateway entrypoint.
//
// This is the Day 1 skeleton: it proxies every request to a single static
// backend. Config-driven multi-backend routing, load balancing strategies,
// health checks, rate limiting, and the circuit breaker land in later
// milestones.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/athieu123/goway/internal/backendpool"
)

func main() {
	listenAddr := flag.String("listen", ":8080", "address for the gateway to listen on")
	backendURL := flag.String("backend", "http://localhost:8081", "backend URL to proxy to")
	flag.Parse()

	pool, err := backendpool.NewPool([]string{*backendURL})
	if err != nil {
		log.Fatalf("failed to build backend pool: %v", err)
	}
	backend := pool.Backends[0]

	log.Printf("gateway listening on %s, proxying to %s", *listenAddr, backend.URL)
	log.Fatal(http.ListenAndServe(*listenAddr, backend.ReverseProxy))
}
