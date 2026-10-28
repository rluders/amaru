//go:build !windows

package osproc

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

func ignoreTerm() { signal.Ignore(syscall.SIGTERM) }

// detach puts the grandchild in a session of its own, outside the process group.
func detach(cmd *exec.Cmd, escape bool) {
	if escape {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
}

// alive reports whether the process exists and is not a zombie.
func alive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	if b, err := os.ReadFile("/proc/" + itoa(pid) + "/stat"); err == nil {
		// the state letter follows the closing parenthesis of the command name
		if i := strings.LastIndexByte(string(b), ')'); i >= 0 && i+2 < len(b) && b[i+2] == 'Z' {
			return false
		}
	}
	return true
}

func itoa(n int) string { return fmtInt(n) }

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

func TestTerminateEndsTheWholeTree(t *testing.T) {
	p := start(t, context.Background(), Runner{}, "tree", nil)
	pid := grandchildPID(t, p)
	if !alive(pid) {
		t.Fatal("the grandchild is not running")
	}
	p.Terminate(time.Second)
	waitEnds(t, p, 5*time.Second)
	eventually(t, 5*time.Second, "the grandchild to end", func() bool { return !alive(pid) })
}

func TestContextCancelEndsTheWholeTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := start(t, ctx, Runner{Grace: time.Second}, "tree", nil)
	pid := grandchildPID(t, p)
	cancel()
	waitEnds(t, p, 5*time.Second)
	eventually(t, 5*time.Second, "the grandchild to end", func() bool { return !alive(pid) })
}

// A process that ignores the polite signal is killed when the grace period ends, and the kill
// reaches a descendant even though the root has already gone.
func TestTheGracePeriodIsHonoredAndThenTheTreeIsKilled(t *testing.T) {
	const grace = 700 * time.Millisecond
	p := start(t, context.Background(), Runner{}, "stubborn-tree", nil)
	pid := grandchildPID(t, p)
	began := time.Now()
	p.Terminate(grace)
	// The polite signal does not end either process: both ignore it.
	time.Sleep(grace / 3)
	if !alive(pid) {
		t.Fatal("the grandchild ended before the grace period; it should ignore the polite signal")
	}
	waitEnds(t, p, 5*time.Second)
	eventually(t, 5*time.Second, "the grandchild to be killed", func() bool { return !alive(pid) })
	if d := time.Since(began); d < grace {
		t.Fatalf("the tree ended after %s, before the %s grace period", d, grace)
	}
}

// The known limit of process groups (ADR-0053): a descendant that starts its own session is
// outside the group and is not reached. The test records the limit so it cannot be forgotten, and
// cleans up after itself.
func TestADescendantThatLeavesTheGroupIsNotReached(t *testing.T) {
	p := start(t, context.Background(), Runner{}, "escape-tree", nil)
	pid := grandchildPID(t, p)
	defer syscall.Kill(pid, syscall.SIGKILL)
	p.Terminate(200 * time.Millisecond)
	waitEnds(t, p, 5*time.Second)
	time.Sleep(500 * time.Millisecond)
	if !alive(pid) {
		t.Skip("the escaped descendant ended; the documented limit did not show on this system")
	}
}
