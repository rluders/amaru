// Package pgsup starts the bundled PostgreSQL under the embedded-postgres v2
// supervisor, the way ADR-0015 and Q28/Q29 describe the compact install.
package pgsup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	ep "github.com/fergusstrange/embedded-postgres/v2"
)

// Bundle is the installer-shipped layout: <dir>/pg (PostgreSQL installation)
// and <dir>/supervisor (the embedded-postgres supervisor binary).
type Bundle struct{ Dir string }

func (b Bundle) PG() string { return filepath.Join(b.Dir, "pg") }
func (b Bundle) Supervisor() string {
	name := "supervisor"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(b.Dir, name)
}

// Instance is a running supervised PostgreSQL.
type Instance struct {
	DB        *ep.EmbeddedPostgres
	URL       string
	StartedIn time.Duration
	Socket    bool
}

// Start runs PostgreSQL with a persistent data directory in stateDir. On
// Linux and macOS it listens on a Unix socket only; on Windows on a random
// loopback port. Authentication is SCRAM-SHA-256 everywhere (initdb --auth).
// Nothing is downloaded: the provider is LocalProvider and the supervisor path
// is explicit.
func Start(ctx context.Context, b Bundle, stateDir string) (*Instance, error) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}
	pw, err := password(filepath.Join(stateDir, "pg.password"))
	if err != nil {
		return nil, err
	}
	logf, err := os.OpenFile(filepath.Join(stateDir, "postgres.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	cfg := ep.DefaultConfig().
		Port(0).
		Database("amaru").
		Username("amaru").
		Password(pw).
		Provider(ep.LocalProvider(b.PG())).
		Supervisor(b.Supervisor()).
		DataPath(filepath.Join(stateDir, "data")).
		RuntimePath(filepath.Join(stateDir, "run")).
		CachePath(filepath.Join(stateDir, "cache")).
		Logger(logf)
	sock := runtime.GOOS != "windows"
	if sock {
		sd := filepath.Join(stateDir, "s")
		if err := os.MkdirAll(sd, 0o700); err != nil {
			return nil, err
		}
		cfg = cfg.UnixSocket(sd)
	}
	for _, d := range []string{"run", "cache"} {
		if err := os.MkdirAll(filepath.Join(stateDir, d), 0o700); err != nil {
			return nil, err
		}
	}
	db := ep.NewDatabase(cfg)
	t0 := time.Now()
	if err := db.StartContext(ctx); err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	return &Instance{DB: db, URL: db.ConnectionURL(), StartedIn: time.Since(t0), Socket: sock}, nil
}

func (i *Instance) Close() error { return i.DB.Close() }

func password(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		return string(b), nil
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	pw := hex.EncodeToString(raw)
	return pw, os.WriteFile(path, []byte(pw), 0o600)
}
