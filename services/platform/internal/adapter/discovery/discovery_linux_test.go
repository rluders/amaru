//go:build linux

package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureSecret = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func listener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}
func TestPrivateAtomicPublicationAndOwnership(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	l := listener(t)
	start := time.Now().UTC()
	p, err := Publish(dir, l, fixtureSecret, "test", start)
	if err != nil {
		t.Fatal("publish failed")
	}
	defer p.Close()
	got, err := Read(dir)
	if err != nil || got.Endpoint != "http://"+l.Addr().String() || !got.StartedAt.Equal(start) {
		t.Fatal("read failed")
	}
	for path, mode := range map[string]os.FileMode{dir: 0700, filepath.Join(dir, FileName): 0600, filepath.Join(dir, lockName): 0600} {
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != mode {
			t.Fatal("unsafe mode")
		}
	}
	if other, err := Publish(dir, l, fixtureSecret, "other", start); !errors.Is(err, ErrRunning) {
		if other != nil {
			other.Close()
		}
		t.Fatal("second owner accepted")
	}
	// A separate process must also be excluded, including if it shares the PID's
	// private state path. Never include the discovery body in command output.
	cmd := exec.Command(os.Args[0], "-test.run=^TestDiscoveryLockChild$")
	cmd.Env = append(os.Environ(), "AMARU_LOCK_TEST_DIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = out
		t.Fatal("cross-process owner lock failed")
	}
	if err = p.Close(); err != nil {
		t.Fatal("cleanup failed")
	}
	if _, err = os.Stat(filepath.Join(dir, FileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("record survived close")
	}
	replacement, err := Publish(dir, l, strings.Repeat("B", 42)+"A", "next", start)
	if err != nil {
		t.Fatal("replacement failed")
	}
	defer replacement.Close()
	p.Close()
	if _, err = Read(dir); err != nil {
		t.Fatal("old cleanup erased replacement")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("temporary files leaked")
	}
}
func TestDiscoveryLockChild(t *testing.T) {
	dir := os.Getenv("AMARU_LOCK_TEST_DIR")
	if dir == "" {
		return
	}
	p, err := Publish(dir, listener(t), fixtureSecret, "child", time.Now())
	if p != nil {
		p.Close()
	}
	if !errors.Is(err, ErrRunning) {
		t.Fatal("active owner not excluded")
	}
}
func TestRefuseUnsafeExistingPaths(t *testing.T) {
	for _, kind := range []string{"directory-mode", "directory-symlink", "file-mode", "file-symlink", "lock-symlink", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "state")
			os.Mkdir(dir, 0700)
			switch kind {
			case "directory-mode":
				os.Chmod(dir, 0755)
			case "directory-symlink":
				os.Remove(dir)
				os.Symlink(parent, dir)
			case "file-mode":
				os.WriteFile(filepath.Join(dir, FileName), []byte("{}"), 0644)
			case "file-symlink":
				os.Symlink(filepath.Join(parent, "outside"), filepath.Join(dir, FileName))
			case "lock-symlink":
				os.Symlink(filepath.Join(parent, "outside"), filepath.Join(dir, lockName))
			case "malformed":
				os.WriteFile(filepath.Join(dir, FileName), []byte("{}"), 0600)
			}
			if _, err := Read(dir); err == nil {
				t.Fatal("unsafe read accepted")
			}
			if kind != "malformed" {
				p, err := Publish(dir, listener(t), fixtureSecret, "test", time.Now())
				if p != nil {
					p.Close()
				}
				if err == nil {
					t.Fatal("unsafe publication accepted")
				}
			}
		})
	}
}
func TestSafeHealthProbe(t *testing.T) {
	start := time.Now().UTC()
	var mode atomic.Value
	mode.Store("valid")
	var redirected atomic.Bool
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Store(true) }))
	defer foreign.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential sent to probe")
		}
		switch mode.Load().(string) {
		case "redirect":
			http.Redirect(w, r, foreign.URL, 302)
			return
		case "misleading":
			w.Write([]byte(`{}`))
			return
		case "invalid":
			w.Write([]byte(`not json`))
			return
		}
		protocol := 1
		if mode.Load() == "protocol" {
			protocol = 2
		}
		build := "test"
		if mode.Load() == "build" {
			build = "other"
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "protocolVersion": protocol, "platformVersion": build, "startedAt": start, "uptimeSec": 0})
	}))
	defer srv.Close()
	addr := srv.Listener.Addr().(*net.TCPAddr)
	info := Info{Endpoint: srv.URL, Port: addr.Port, PID: os.Getpid(), AdministratorSecret: fixtureSecret, ProtocolVersion: 1, PlatformVersion: "test", StartedAt: start}
	for _, v := range []string{"valid", "redirect", "misleading", "invalid", "protocol", "build"} {
		mode.Store(v)
		if Live(context.Background(), info) != (v == "valid") {
			t.Fatal("health probe result incorrect")
		}
	}
	if redirected.Load() {
		t.Fatal("probe followed redirect")
	}
	for _, endpoint := range []string{foreign.URL + "?token=secret", "http://example.test:7777", "http://127.0.0.1:1", "http://localhost:7777", "http://user:pass@127.0.0.1:7777"} {
		bad := info
		bad.Endpoint = endpoint
		if Live(context.Background(), bad) {
			t.Fatal("unsafe endpoint accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mode.Store("valid")
	if Live(ctx, info) {
		t.Fatal("cancelled probe succeeded")
	}
}

type noEntropy struct{}

func (noEntropy) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
func TestPublicationFailurePreservesRecord(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	l := listener(t)
	p, err := Publish(dir, l, fixtureSecret, "first", time.Now())
	if err != nil {
		t.Fatal("initial publication")
	}
	// Simulate a crash by dropping ownership without deleting the record.
	p.release()
	before, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal("missing previous record")
	}
	saved := randomSource
	randomSource = noEntropy{}
	defer func() { randomSource = saved }()
	replacement, err := Publish(dir, l, fixtureSecret, "second", time.Now())
	if replacement != nil {
		replacement.Close()
	}
	if err == nil {
		t.Fatal("entropy failure published")
	}
	after, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil || string(after) != string(before) {
		t.Fatal("failed publication destroyed previous record")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("failed publication left temporary file")
	}
	randomSource = saved
	replacement, err = Publish(dir, l, fixtureSecret, "third", time.Now())
	if err != nil {
		t.Fatal("failure leaked ownership lock")
	}
	defer replacement.Close()
}
