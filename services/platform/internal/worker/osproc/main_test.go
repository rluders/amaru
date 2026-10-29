package osproc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

const helperEnv = "AMARU_TEST_HELPER"

// The test binary re-executes itself as the child (the helper-process pattern), so the tests need
// no shell, no real tool and no network.
func TestMain(m *testing.M) {
	ConfineExecMain() // a confined test process comes through the launch helper first
	switch os.Getenv(helperEnv) {
	case "env":
		for _, kv := range os.Environ() {
			fmt.Println(kv)
		}
		os.Exit(0)
	case "cat":
		b, _ := io.ReadAll(os.Stdin)
		os.Stdout.Write(b)
		os.Exit(0)
	case "sleep":
		time.Sleep(time.Minute)
		os.Exit(0)
	case "stubborn": // ignores the polite signal, so only the kill ends it
		ignoreTerm()
		time.Sleep(time.Minute)
		os.Exit(0)
	case "confine-ops":
		runOps(os.Args[1:])
	case "tree", "stubborn-tree", "escape-tree":
		runTree(os.Getenv(helperEnv))
	}
	os.Exit(m.Run())
}

// runTree starts a grandchild, prints its pid, then runs (or, for the stubborn variant, ignores
// the polite signal and runs) until it is ended.
func runTree(mode string) {
	child := "sleep"
	if mode == "stubborn-tree" {
		ignoreTerm()
		child = "stubborn"
	}
	exe, _ := os.Executable()
	cmd := exec.Command(exe)
	cmd.Env = []string{helperEnv + "=" + child}
	detach(cmd, mode == "escape-tree")
	if err := cmd.Start(); err != nil {
		fmt.Println("start failed:", err)
		os.Exit(2)
	}
	fmt.Println(cmd.Process.Pid)
	time.Sleep(time.Minute)
	os.Exit(0)
}

func start(t *testing.T, ctx context.Context, r Runner, mode string, parent []string) ports.Proc {
	t.Helper()
	exe, _ := os.Executable()
	if r.Environ == nil {
		r.Environ = func() []string { return parent }
	}
	p, err := r.Start(ctx, ports.ProcSpec{Path: exe, Dir: t.TempDir(), Env: []string{helperEnv + "=" + mode}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// grandchildPID reads the pid a tree helper prints.
func grandchildPID(t *testing.T, p ports.Proc) int {
	t.Helper()
	line, err := bufio.NewReader(p.Stdout()).ReadString('\n')
	if err != nil {
		t.Fatalf("reading the grandchild pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("grandchild pid %q: %v", line, err)
	}
	return pid
}

func waitEnds(t *testing.T, p ports.Proc, within time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() { p.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("the process was still running after %s", within)
	}
}

func eventually(t *testing.T, within time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen within %s", what, within)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestEnvAllowListDropsSecrets(t *testing.T) {
	parent := []string{"PATH=/bin", "HOME=/h", "AWS_SECRET_ACCESS_KEY=s3cret", "GITHUB_TOKEN=t", "ANTHROPIC_API_KEY=k", "LC_ALL=C", "XDG_STATE_HOME=/x"}
	p := start(t, context.Background(), Runner{}, "env", parent)
	out, _ := io.ReadAll(p.Stdout())
	p.Wait()
	got := string(out)
	for _, keep := range []string{"PATH=/bin", "HOME=/h", "LC_ALL=C", "XDG_STATE_HOME=/x", helperEnv + "=env"} {
		if !strings.Contains(got, keep) {
			t.Errorf("%s was dropped", keep)
		}
	}
	for _, drop := range []string{"AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "ANTHROPIC_API_KEY"} {
		if strings.Contains(got, drop) {
			t.Errorf("%s reached the child", drop)
		}
	}
}

func TestStdinDelivered(t *testing.T) {
	exe, _ := os.Executable()
	p, err := Runner{Environ: func() []string { return nil }}.Start(context.Background(),
		ports.ProcSpec{Path: exe, Dir: t.TempDir(), Env: []string{helperEnv + "=cat"}, Stdin: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(p.Stdout())
	if code, err := p.Wait(); code != 0 || err != nil || string(out) != "hello" {
		t.Fatalf("code %d, err %v, out %q", code, err, out)
	}
}

func TestInteractiveStdin(t *testing.T) {
	exe, _ := os.Executable()
	p, err := Runner{Environ: func() []string { return nil }}.Start(context.Background(),
		ports.ProcSpec{Path: exe, Dir: t.TempDir(), Env: []string{helperEnv + "=cat"}, Interactive: true})
	if err != nil {
		t.Fatal(err)
	}
	ip, ok := p.(ports.InteractiveProc)
	if !ok {
		t.Fatal("an Interactive process must implement InteractiveProc")
	}
	io.WriteString(ip.Stdin(), "ping")
	ip.Stdin().Close()
	out, _ := io.ReadAll(p.Stdout())
	p.Wait()
	if string(out) != "ping" {
		t.Fatalf("out %q", out)
	}
}

func TestStartRequiresPathAndDir(t *testing.T) {
	if _, err := (Runner{}).Start(context.Background(), ports.ProcSpec{Path: "x"}); err == nil {
		t.Fatal("missing Dir must be refused")
	}
}

func TestWaitTwiceIsSafe(t *testing.T) {
	p := start(t, context.Background(), Runner{}, "cat", nil)
	p.Wait()
	p.Wait()
}

func TestTerminateStopsProcess(t *testing.T) {
	p := start(t, context.Background(), Runner{}, "sleep", nil)
	p.Terminate(time.Second)
	waitEnds(t, p, 5*time.Second)
}

func TestContextCancelTerminates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := start(t, ctx, Runner{}, "sleep", nil)
	cancel()
	waitEnds(t, p, 5*time.Second)
}

func TestResolver(t *testing.T) {
	if _, err := (Resolver{}).Find("definitely-not-installed-amaru"); err == nil {
		t.Fatal("expected not found")
	}
	exe, _ := os.Executable()
	if p, err := (Resolver{}).Find(exe); err != nil || p == "" {
		t.Fatalf("absolute candidate: %v", err)
	}
	dir := t.TempDir()
	if _, err := (Resolver{ExtraDirs: []string{dir}}).Find("nothing-here"); err == nil {
		t.Fatal("an empty extra directory must not match")
	}
}
