//go:build authfixture

// Test-only SDK fixture. It serves real auth/discovery adapters and health,
// without product commands, a database, an agent or a logging middleware.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rluders/amaru/services/platform/internal/adapter/discovery"
	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("state directory required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("bind failed")
	}
	defer listener.Close()
	auth, err := localauth.New(listener.Addr())
	if err != nil {
		return fmt.Errorf("auth init failed")
	}
	defer auth.Close()
	started := time.Now().UTC()
	publication, err := discovery.Publish(os.Args[1], listener, auth.AdministratorSecret(), "sdk-fixture", started)
	if err != nil {
		return fmt.Errorf("private discovery failed")
	}
	defer publication.Close()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "platformVersion": "sdk-fixture", "protocolVersion": 1, "startedAt": started, "uptimeSec": int(time.Since(started).Seconds())})
	})
	server := &http.Server{Handler: auth.Handler(next), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	err = server.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve failed")
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
