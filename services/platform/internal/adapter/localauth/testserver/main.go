//go:build authfixture

// Test-only SDK fixture: real auth/discovery, Git inspection and Project HTTP
// adapters over fake state. It is not product startup and uses no database or agent.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rluders/amaru/services/platform/internal/adapter/discovery"
	"github.com/rluders/amaru/services/platform/internal/adapter/httpapi"
	"github.com/rluders/amaru/services/platform/internal/adapter/localauth"
	"github.com/rluders/amaru/services/platform/internal/adapter/operational"
	"github.com/rluders/amaru/services/platform/internal/attach"
	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/worker/osproc"
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
	projects := fakes.NewProjectStore()
	c := coordinator.New(fakes.NewExecutionStore(), time.Now)
	access, err := coordinator.NewProjectAccess(c, projects, auth.Principal)
	if err != nil {
		return fmt.Errorf("fixture access init failed")
	}
	settings, err := httpapi.SettingsHandler(auth, access, projects)
	if err != nil {
		return fmt.Errorf("fixture settings init failed")
	}
	projectHTTP, err := httpapi.ProjectsHandler(auth, c, projects, attach.Attacher{Runner: osproc.Runner{}, Resolver: osproc.Resolver{}})
	if err != nil {
		return fmt.Errorf("fixture Project HTTP init failed")
	}

	health := auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "platformVersion": "sdk-fixture", "protocolVersion": 1, "startedAt": started, "uptimeSec": int(time.Since(started).Seconds())})
	}))
	next := http.NewServeMux()
	next.Handle("/v1/health", health)
	next.Handle("/v1/projects/{projectId}/settings", settings)
	next.Handle("/", projectHTTP)
	logger, err := operational.New(os.Stdout)
	if err != nil {
		return err
	}
	handler := logger.HTTP("sdk-fixture", next, func(err error) { fmt.Fprintln(os.Stderr, err.Error()) })
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
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
