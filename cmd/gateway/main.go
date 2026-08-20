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
	"time"

	"github.com/austinthieu/goway/internal/config"
	"github.com/austinthieu/goway/internal/metrics"
	"github.com/austinthieu/goway/internal/proxy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const shutdownTimeout = 5 * time.Second

func main() {
	configPath := flag.String("config", "testdata/config.yaml", "path to gateway YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	registry := prometheus.NewRegistry()
	recorder := metrics.NewRecorder(registry)

	gw := &proxy.Gateway{Metrics: recorder}
	if err := gw.Reload(ctx, cfg); err != nil {
		log.Fatalf("failed to load initial config: %v", err)
	}
	go watchSIGHUP(ctx, gw, *configPath)

	server := &http.Server{Addr: cfg.ListenAddr, Handler: gw}

	adminMux := http.NewServeMux()
	adminMux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	adminServer := &http.Server{Addr: cfg.AdminAddr, Handler: adminMux}

	go func() {
		log.Printf("admin endpoint listening on %s (/metrics)", cfg.AdminAddr)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("admin server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		log.Println("shutting down gateway: waiting up to", shutdownTimeout, "for in-flight requests")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		server.Shutdown(shutdownCtx)
		adminServer.Shutdown(shutdownCtx)
	}()

	log.Printf("gateway listening on %s, backends=%d", cfg.ListenAddr, len(cfg.Backends))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// watchSIGHUP reloads the config from configPath every time the process
// receives SIGHUP, e.g. `kill -HUP <pid>`, and rebuilds the gateway's
// backend pool/balancer/limiter without dropping any in-flight request.
func watchSIGHUP(ctx context.Context, gw *proxy.Gateway, configPath string) {
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	defer signal.Stop(sighup)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sighup:
			cfg, err := config.Load(configPath)
			if err != nil {
				log.Printf("config reload failed, keeping previous config: %v", err)
				continue
			}
			if err := gw.Reload(ctx, cfg); err != nil {
				log.Printf("config reload failed, keeping previous config: %v", err)
				continue
			}
			log.Printf("config reloaded from %s, backends=%d", configPath, len(cfg.Backends))
		}
	}
}
