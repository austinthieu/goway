// Command gateway is the HTTP load balancer / API gateway entrypoint.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/austinthieu/goway/internal/backendpool"
	"github.com/austinthieu/goway/internal/balancer"
	"github.com/austinthieu/goway/internal/config"
	"github.com/austinthieu/goway/internal/healthcheck"
	"github.com/austinthieu/goway/internal/proxy"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go healthcheck.Run(ctx, pool, healthcheck.DefaultOptions())

	gw := &proxy.Gateway{Pool: pool, Balancer: bal}
	server := &http.Server{Addr: cfg.ListenAddr, Handler: gw}

	go func() {
		<-ctx.Done()
		log.Println("shutting down gateway")
		server.Close()
	}()

	log.Printf("gateway listening on %s, strategy=%s, backends=%d", cfg.ListenAddr, cfg.Strategy, len(pool.Backends))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
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
