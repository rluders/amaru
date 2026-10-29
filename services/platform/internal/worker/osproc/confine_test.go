package osproc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

func fakeProbe(abi int, err error) func() (int, error) {
	return func() (int, error) { return abi, err }
}

func selfRunner(t *testing.T) Runner {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Runner{Self: exe, Probe: fakeProbe(MinLandlockABI, nil)}
}

func confinedSpec(root string, env ...string) ports.ProcSpec {
	exe, _ := os.Executable()
	return ports.ProcSpec{Path: exe, Dir: root, Env: append([]string{helperEnv + "=env"}, env...),
		Confine: &ports.Confinement{WritableRoots: []string{root}}}
}

func TestConfinedStartIsRefusedWhenItCannotBeEnforced(t *testing.T) {
	root := t.TempDir()
	cases := map[string]Runner{
		"no mechanism":      {Self: "/x", Probe: fakeProbe(0, errors.New("Landlock is not available"))},
		"ABI below minimum": {Self: "/x", Probe: fakeProbe(MinLandlockABI-1, nil)},
		"no helper":         {Probe: fakeProbe(MinLandlockABI, nil)},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := r.Start(context.Background(), confinedSpec(root))
			if !errors.Is(err, ports.ErrConfinementUnavailable) || p != nil {
				t.Fatalf("a confined Step must be refused: proc %v, err %v", p, err)
			}
		})
	}
}

func TestConfinementRootsAreValidated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ssh := filepath.Join(home, ".ssh")
	checkout := filepath.Join(home, "checkout")
	inside := filepath.Join(home, "worktrees", "wi-1")
	for _, d := range []string{ssh, checkout, inside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(inside, "file")
	os.WriteFile(file, nil, 0o644)
	link := filepath.Join(t.TempDir(), "link")
	os.Symlink(home, link)

	bad := map[string][]string{
		"none":                    nil,
		"relative":                {"worktrees/wi-1"},
		"dot-dot":                 {inside + "/../wi-1"},
		"missing":                 {filepath.Join(home, "nope")},
		"a file":                  {file},
		"filesystem root":         {"/"},
		"the home directory":      {home},
		"a symlink to home":       {link},
		"above the home":          {filepath.Dir(home)},
		"inside .ssh":             {ssh},
		"the Project checkout":    {checkout},
		"the checkout itself":     {checkout},
		"above the checkout":      {home},
		"one bad among good ones": {inside, home},
	}
	r := Runner{Self: "/x", Probe: fakeProbe(MinLandlockABI, nil), Forbidden: []string{checkout}}
	// the home rules must hold on their own, not only because the checkout lies under the home
	noCheckout := Runner{Self: "/x", Probe: fakeProbe(MinLandlockABI, nil)}
	for name, roots := range bad {
		rr := r
		if strings.Contains(name, "home") || name == "inside .ssh" || name == "a symlink to home" {
			rr = noCheckout
		}
		t.Run(name, func(t *testing.T) {
			_, err := rr.Start(context.Background(), ports.ProcSpec{Path: "/x", Dir: inside, Confine: &ports.Confinement{WritableRoots: roots}})
			if !errors.Is(err, ports.ErrInvalidConfinement) {
				t.Fatalf("roots %v must be invalid, got %v", roots, err)
			}
		})
	}
	got, err := resolveRoots([]string{inside, inside + "/"}, []string{checkout})
	if err != nil || len(got) != 1 {
		t.Fatalf("a good root, listed twice, must be accepted once: %v %v", got, err)
	}
}

func TestMarkerNeverComesFromASpec(t *testing.T) {
	root := t.TempDir()
	exe, _ := os.Executable()
	p, err := (Runner{Environ: func() []string { return nil }}).Start(context.Background(),
		ports.ProcSpec{Path: exe, Dir: root, Env: []string{helperEnv + "=env", confineEnv + "=1"}})
	if err != nil {
		t.Fatal(err)
	}
	out := readAll(t, p)
	if strings.Contains(out, confineEnv) {
		t.Fatalf("an unconfined start must not pass the launch-helper marker:\n%s", out)
	}
}

func TestHelperFailsClosedOnBadUsage(t *testing.T) {
	exe, _ := os.Executable()
	for name, args := range map[string][]string{
		"no separator":  {"/tmp"},
		"no roots":      {"--", "/bin/true"},
		"no program":    {"/tmp", "--"},
		"relative root": {"rel", "--", "/bin/true"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(exe, args...)
			cmd.Env = []string{confineEnv + "=1"}
			err := cmd.Run()
			var ee *exec.ExitError
			if !errors.As(err, &ee) || ee.ExitCode() != exitConfine {
				t.Fatalf("the helper must exit %d without running anything, got %v", exitConfine, err)
			}
		})
	}
}

func readAll(t *testing.T, p ports.Proc) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := p.Stdout().Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	p.Wait()
	return sb.String()
}
