package osproc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

// These tests run real confined processes against the kernel (integration-tested on this host,
// not on every kernel): they need Landlock ABI >= MinLandlockABI and skip, loudly, without it.
func realRunner(t *testing.T) Runner {
	t.Helper()
	abi, err := landlockABI()
	if err != nil || abi < MinLandlockABI {
		t.Skipf("Landlock ABI >= %d is not available on this host (%v, abi %d): the confinement is NOT validated here", MinLandlockABI, err, abi)
	}
	t.Logf("kernel Landlock ABI %d", abi)
	exe, _ := os.Executable()
	return Runner{Self: exe}
}

// runConfined starts the test binary confined to root with the given operations, and returns the
// outcome line of each operation by its text.
func runConfined(t *testing.T, r Runner, root string, ops ...string) map[string]string {
	t.Helper()
	exe, _ := os.Executable()
	p, err := r.Start(context.Background(), ports.ProcSpec{Path: exe, Args: ops, Dir: root,
		Env: []string{helperEnv + "=confine-ops"}, Confine: &ports.Confinement{WritableRoots: []string{root}}})
	if err != nil {
		t.Fatal(err)
	}
	if cp, ok := p.(ports.ConfinedProc); !ok || cp.Confinement().ABI < MinLandlockABI || len(cp.Confinement().WritableRoots) != 1 {
		t.Fatalf("a confined proc must report what was enforced: %v", p)
	}
	out := readAll(t, p)
	res := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		op, r, ok := strings.Cut(line, " -> ")
		if !ok {
			t.Fatalf("unexpected output %q in\n%s", line, out)
		}
		res[op] = r
	}
	for _, op := range ops {
		if _, ok := res[op]; !ok {
			t.Fatalf("no result for %s in\n%s", op, out)
		}
	}
	return res
}

func expect(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()
	for op, w := range want {
		if got[op] != w {
			t.Errorf("%s: got %q, want %q", op, got[op], w)
		}
	}
}

func fileContent(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConfinedProcessWritesOnlyInsideItsRoot(t *testing.T) {
	r := realRunner(t)
	root, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "victim")
	os.WriteFile(victim, []byte("original"), 0o644)
	inFile := filepath.Join(root, "in")
	os.WriteFile(inFile, []byte("original"), 0o644)

	got := runConfined(t, r, root,
		"write:"+filepath.Join(root, "new"),
		"mkdir:"+filepath.Join(root, "dir"),
		"append:"+inFile,
		"trunc:"+inFile,
		"remove:"+filepath.Join(root, "new"),
		"devnull:",
		"read:"+victim,
		"marker:",
		"write:"+filepath.Join(outside, "new"),
		"mkdir:"+filepath.Join(outside, "dir"),
		"append:"+victim,
		"trunc:"+victim,
		"remove:"+victim,
	)
	expect(t, got, map[string]string{
		"write:" + filepath.Join(root, "new"):    "ok",
		"mkdir:" + filepath.Join(root, "dir"):    "ok",
		"append:" + inFile:                       "ok",
		"trunc:" + inFile:                        "ok",
		"remove:" + filepath.Join(root, "new"):   "ok",
		"devnull:":                               "ok",
		"read:" + victim:                         "ok", // reads are not restricted (ADR-0057)
		"marker:":                                "ok",
		"write:" + filepath.Join(outside, "new"): "denied",
		"mkdir:" + filepath.Join(outside, "dir"): "denied",
		"append:" + victim:                       "denied",
		"trunc:" + victim:                        "denied",
		"remove:" + victim:                       "denied",
	})
	if fileContent(t, victim) != "original" {
		t.Fatal("the file outside the root was changed")
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); err == nil {
		t.Fatal("a file was created outside the root")
	}
}

func TestConfinedProcessCannotEscapeByLinks(t *testing.T) {
	r := realRunner(t)
	root, outside := t.TempDir(), t.TempDir()
	victim := filepath.Join(outside, "victim")
	os.WriteFile(victim, []byte("original"), 0o644)
	inside := filepath.Join(root, "inside")
	os.WriteFile(inside, []byte("x"), 0o644)
	link := filepath.Join(root, "sym")
	os.Symlink(victim, link) // placed by the test: the agent may also create one (below)

	got := runConfined(t, r, root,
		"write:"+link, // through a symlink that points out
		"symlink:"+victim+"|"+filepath.Join(root, "sym2"),
		"write:"+filepath.Join(root, "sym2"),
		"rename:"+victim+"|"+filepath.Join(root, "stolen"),  // an outside file moved in
		"link:"+victim+"|"+filepath.Join(root, "hard"),      // an outside file hard-linked in
		"rename:"+inside+"|"+filepath.Join(outside, "gone"), // an inside file moved out
		"rename:"+inside+"|"+filepath.Join(root, "renamed"),
		"mkdir:"+filepath.Join(root, "d1"),
		"mkdir:"+filepath.Join(root, "d2"),
		"write:"+filepath.Join(root, "d1", "f"),
		"rename:"+filepath.Join(root, "d1", "f")+"|"+filepath.Join(root, "d2", "f"), // across directories inside the root
		"setsid:write:"+filepath.Join(outside, "from-setsid"),
		"setsid:write:"+filepath.Join(root, "from-setsid"),
	)
	expect(t, got, map[string]string{
		"write:" + link: "denied",
		"symlink:" + victim + "|" + filepath.Join(root, "sym2"):                           "ok", // creating a link is not a write outside
		"write:" + filepath.Join(root, "sym2"):                                            "denied",
		"rename:" + victim + "|" + filepath.Join(root, "stolen"):                          "denied",
		"link:" + victim + "|" + filepath.Join(root, "hard"):                              "denied",
		"rename:" + inside + "|" + filepath.Join(outside, "gone"):                         "denied",
		"rename:" + inside + "|" + filepath.Join(root, "renamed"):                         "ok",
		"rename:" + filepath.Join(root, "d1", "f") + "|" + filepath.Join(root, "d2", "f"): "ok",
		"setsid:write:" + filepath.Join(outside, "from-setsid"):                           "error: write:" + filepath.Join(outside, "from-setsid") + " -> denied",
		"setsid:write:" + filepath.Join(root, "from-setsid"):                              "ok",
	})
	if fileContent(t, victim) != "original" {
		t.Fatal("the file outside the root was changed")
	}
	if _, err := os.Stat(filepath.Join(outside, "gone")); err == nil {
		t.Fatal("a file left the root")
	}
}

func TestConfinementDoesNotOutliveTheChild(t *testing.T) {
	r := realRunner(t)
	if _, err := os.Stat("/proc/self/status"); err != nil {
		t.Skip("needs /proc")
	}
	root, outside := t.TempDir(), t.TempDir()
	runConfined(t, r, root, "write:"+filepath.Join(root, "a"))
	// the test process itself is unrestricted: only the child was confined
	if err := os.WriteFile(filepath.Join(outside, "ok"), []byte("x"), 0o644); err != nil {
		t.Fatalf("the Worker side must stay unconfined: %v", err)
	}
}
