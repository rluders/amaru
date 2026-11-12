//go:build pg && linux

package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator/coordtest"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

func TestCoordinatorCrashProcess(t *testing.T) {
	if os.Getenv("AMARU_CRASH_CHILD") == "1" {
		db, err := sql.Open("pgx", os.Getenv("AMARU_CRASH_DSN"))
		if err != nil {
			t.Fatal("open child database")
		}
		defer db.Close()
		coordtest.ProcessCrashChild(t, NewExecutionStore(db), os.Getenv("AMARU_CRASH_SCENARIO"), os.Getenv("AMARU_CRASH_POINT"))
		return
	}
	var dsn string
	coordtest.ProcessCrashInjection(t, func(t *testing.T) ports.ExecutionStore {
		db := migratedDB(t)
		var name string
		if err := db.QueryRow("select current_database()").Scan(&name); err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(os.Getenv("AMARU_TEST_PG"))
		if err != nil {
			t.Fatal("parse test database address")
		}
		u.Path = "/" + name
		dsn = u.String()
		return NewExecutionStore(db)
	}, func(t *testing.T, scenario, point string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCoordinatorCrashProcess$")
		cmd.Env = append(os.Environ(), "AMARU_CRASH_CHILD=1", "AMARU_CRASH_DSN="+dsn,
			"AMARU_CRASH_SCENARIO="+scenario, "AMARU_CRASH_POINT="+point)
		// The DSN is only in the environment, never a command argument or a test log.
		var output bytes.Buffer
		cmd.Stdout, cmd.Stderr = &output, &output
		ready, signal, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer ready.Close()
		defer signal.Close()
		cmd.ExtraFiles = []*os.File{signal}
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		defer stdin.Close()
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		reaped := false
		defer func() {
			if !reaped {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			}
		}()
		if err := signal.Close(); err != nil {
			t.Fatal(err)
		}
		if point == "reconcile" {
			err := cmd.Wait()
			reaped = true
			if err != nil {
				t.Fatalf("restart: %v\n%s", err, output.String())
			}
			return
		}
		if err := ready.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var b [1]byte
		_, readErr := io.ReadFull(ready, b[:])
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		reaped = true
		if readErr != nil {
			t.Fatalf("child did not reach %s: %v\n%s", point, readErr, output.String())
		}
		if killErr != nil {
			t.Fatal(killErr)
		}
		if waitErr == nil || cmd.ProcessState.Success() {
			t.Fatal("child exited successfully instead of being killed")
		}
	})
}
