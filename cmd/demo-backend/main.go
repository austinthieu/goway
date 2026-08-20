// Command demo-backend runs a trivial HTTP server used to demo the gateway's
// load balancing, health checks, and failover behavior.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func main() {
	port := flag.String("port", "8081", "port to listen on")
	name := flag.String("name", "backend", "name to identify this instance in responses")
	flag.Parse()

	var requestCount int64

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&requestCount, 1)
		fmt.Fprintf(w, "hello from %s :%s (request #%d)\n", *name, *port, n)
	})

	addr := ":" + *port
	log.Printf("demo-backend %q listening on %s", *name, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
