// Command gateway is the HTTP load balancer / API gateway entrypoint.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/athieu123/goway/internal/backendpool"
	"github.com/athieu123/goway/internal/balancer"
	"github.com/athieu123/goway/internal/config"
	"github.com/athieu123/goway/internal/proxy"
)

func main() {
	configPath := flag.String("config", "testdata/config.yaml", "path to gateway YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	pool, err := backendpool.NewPool(cfg.Backends)
	if err != nil {
		log.Fatalf("failed to build backend pool: %v", err)
	}

	bal, err := newBalancer(cfg.Strategy)
	if err != nil {
		log.Fatalf("failed to init balancer: %v", err)
	}

	gw := &proxy.Gateway{Pool: pool, Balancer: bal}

	log.Printf("gateway listening on %s, strategy=%s, backends=%d", cfg.ListenAddr, cfg.Strategy, len(pool.Backends))
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, gw))
}

func newBalancer(strategy string) (balancer.Balancer, error) {
	switch strategy {
	case "round_robin", "":
		return &balancer.RoundRobin{}, nil
	default:
		return nil, errUnknownStrategy(strategy)
	}
}

type errUnknownStrategy string

func (e errUnknownStrategy) Error() string {
	return "unknown load balancing strategy: " + string(e)
}
