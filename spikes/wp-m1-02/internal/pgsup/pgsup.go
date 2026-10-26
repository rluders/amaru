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
	"strconv"
	"strings"
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
	// WaitedPrevious is how long Start waited for a postmaster of a previous
	// owner (killed or crashed) to exit before it could use the data directory.
	WaitedPrevious time.Duration

	sockDir string
	sockTmp bool
}

// Start runs PostgreSQL with a persistent data directory in stateDir. On
// Linux and macOS it listens on a Unix socket only; on Windows on a random
// loopback port. Authentication is SCRAM-SHA-256 everywhere (initdb --auth).
// Nothing is downloaded: the provider is LocalProvider and the supervisor path
// is explicit.
func Start(ctx context.Context, b Bundle, stateDir string) (*Instance, error) {
	// PostgreSQL sees absolute paths; the socket length check must too.
	stateDir, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, err
	}
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
	var sd string
	var tmp bool
	if sock {
		var err error
		sd, tmp, err = socketDir(stateDir)
		if err != nil {
			return nil, err
		}
		cfg = cfg.UnixSocket(sd)
	}
	for _, d := range []string{"run", "cache"} {
		if err := os.MkdirAll(filepath.Join(stateDir, d), 0o700); err != nil {
			return nil, err
		}
	}
	// A kill during the first start (initdb, createdb) leaves a half-built
	// cluster that the library cannot recover (observed). The marker is written
	// only after the first successful start; without it the data directory holds
	// no user data and is rebuilt.
	marker := filepath.Join(stateDir, "initialized")
	if _, err := os.Stat(marker); err != nil {
		waitPrevious(filepath.Join(stateDir, "data", "postmaster.pid"), 30*time.Second)
		if err := os.RemoveAll(filepath.Join(stateDir, "data")); err != nil {
			return nil, err
		}
	}
	waited := waitPrevious(filepath.Join(stateDir, "data", "postmaster.pid"), 30*time.Second)
	db := ep.NewDatabase(cfg)
	t0 := time.Now()
	if err := db.StartContext(ctx); err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		db.Close()
		return nil, err
	}
	return &Instance{DB: db, URL: db.ConnectionURL(), StartedIn: time.Since(t0), Socket: sock, WaitedPrevious: waited, sockDir: sd, sockTmp: tmp}, nil
}

func (i *Instance) Close() error {
	err := i.DB.Close()
	if i.sockTmp {
		os.RemoveAll(i.sockDir)
	}
	return err
}

// maxSocketPath is below the smallest sun_path limit (104 bytes on macOS;
// Linux allows 107). PostgreSQL appends "/ep-<10 digits>/.s.PGSQL.<port>" (about
// 30 bytes) to the directory it is given.
const maxSocketPath = 100

// socketDir returns the directory for the Unix socket. It lives under stateDir
// unless that path is too long for sun_path (observed: a CI checkout 4 levels
// deep made PostgreSQL fail with "Unix-domain socket path ... is too long"), in
// which case a short private directory under /tmp is used and removed on Close.
func socketDir(stateDir string) (dir string, tmp bool, err error) {
	dir = filepath.Join(stateDir, "s")
	if len(dir)+32 <= maxSocketPath {
		return dir, false, os.MkdirAll(dir, 0o700)
	}
	d, err := os.MkdirTemp("/tmp", "ap")
	if err != nil {
		return "", false, err
	}
	return d, true, nil
}

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

// waitPrevious blocks while the postmaster named in postmaster.pid is alive.
// After a hard kill of the owner, the supervisor still has to shut that
// postmaster down; starting a new one on the same data directory before it
// exits fails with: lock file "postmaster.pid" already exists (observed, see
// the investigation report). It never kills anything.
func waitPrevious(pidFile string, max time.Duration) time.Duration {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	first, _, _ := strings.Cut(string(b), "\n")
	pid, err := strconv.Atoi(strings.TrimSpace(first))
	if err != nil {
		return 0
	}
	t0 := time.Now()
	for Alive(pid) && time.Since(t0) < max {
		time.Sleep(25 * time.Millisecond)
	}
	return time.Since(t0)
}
