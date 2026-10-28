// Package arch enforces the dependency rules that keep the Platform core pure: no OS,
// process, network or vendor imports (AGENTS.md architecture boundaries).
package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/rluders/amaru/services/platform"

// core packages (prefix match on the path relative to the module).
var core = []string{"internal/domain"}

// forbiddenInternal: module packages the core must never import, directly or transitively.
// Adapters are wired in the composition root only.
var forbiddenInternal = []string{"internal/adapter", "internal/provider", "internal/platform", "internal/worker", "internal/store", "cmd"}

// forbiddenStd: standard-library packages that reach the OS, a process or the network.
var forbiddenStd = map[string]bool{
	"os": true, "os/exec": true, "os/signal": true, "os/user": true, "syscall": true, "unsafe": true,
	"net": true, "net/http": true, "net/rpc": true, "net/smtp": true, "io/ioutil": true, "plugin": true,
}

// imports parses the non-test Go files of one package directory.
func imports(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, n), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			seen[p] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// packages lists every package dir (slash-separated, relative to root) that contains Go files.
func packages(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		if len(imports(t, p)) > 0 {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func under(pkg, prefix string) bool { return pkg == prefix || strings.HasPrefix(pkg, prefix+"/") }

// TestCoreIsPure: core packages import only the standard library minus OS, process and network
// packages, and (transitively) no forbidden module package.
func TestCoreIsPure(t *testing.T) {
	root := filepath.Join("..", "..")
	pkgs := packages(t, root)

	var walk func(pkg string, path []string, visited map[string]bool) []string
	walk = func(pkg string, path []string, visited map[string]bool) []string {
		if visited[pkg] {
			return nil
		}
		visited[pkg] = true
		var bad []string
		for _, im := range imports(t, filepath.Join(root, filepath.FromSlash(pkg))) {
			chain := append(append([]string{}, path...), pkg)
			switch {
			case forbiddenStd[im]:
				bad = append(bad, strings.Join(append(chain, im), " -> "))
			case strings.HasPrefix(im, module+"/"):
				rel := strings.TrimPrefix(im, module+"/")
				for _, f := range forbiddenInternal {
					if under(rel, f) {
						bad = append(bad, strings.Join(append(chain, rel), " -> "))
					}
				}
				bad = append(bad, walk(rel, chain, visited)...)
			case strings.Contains(strings.Split(im, "/")[0], "."):
				// a path whose first element has a dot is a third-party module: vendors stay out of the core
				bad = append(bad, strings.Join(append(chain, im), " -> "))
			}
		}
		return bad
	}

	checked := 0
	for _, pkg := range pkgs {
		isCore := false
		for _, c := range core {
			isCore = isCore || under(pkg, c)
		}
		if !isCore {
			continue
		}
		checked++
		for _, b := range walk(pkg, nil, map[string]bool{}) {
			t.Errorf("core package breaks the architecture rule: %s", b)
		}
	}
	if checked == 0 {
		t.Fatal("no core package found; wrong root?")
	}
}

// storageNoFiles: the storage ports, their fakes and the PostgreSQL adapter reach the database and
// memory only. They import no package that opens files, so nothing Amaru stores can land in a
// Project tree (ADR-0043, FEAT-PRJ-005).
var storageNoFiles = []string{"internal/ports", "internal/store/postgres"}

var fileStd = map[string]bool{"os": true, "path/filepath": true, "io/fs": true, "io/ioutil": true, "embed": true}

func TestStorageDoesNotTouchFiles(t *testing.T) {
	root := filepath.Join("..", "..")
	checked := 0
	for _, pkg := range packages(t, root) {
		for _, prefix := range storageNoFiles {
			if !under(pkg, prefix) {
				continue
			}
			checked++
			for _, im := range imports(t, filepath.Join(root, filepath.FromSlash(pkg))) {
				if fileStd[im] {
					t.Errorf("%s imports %s: storage must not read or write files", pkg, im)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no storage package found; wrong root?")
	}
}

// onlyModuleImports: for each package prefix, the module packages (under internal/) it may import;
// everything else in the module is refused. The Worker side and the coordinator meet only through
// ports (FEAT-ENV-001: the embedded Worker talks to the coordinator through the Worker interface,
// like a remote one), and neither reaches a store or an adapter.
var onlyModuleImports = map[string][]string{
	"internal/worker/embedded":    {"internal/ports"},
	"internal/worker/inproc":      {"internal/ports"},
	"internal/worker/osproc":      {"internal/ports"},
	"internal/provider/remotejob": {}, // HTTP helpers: the standard library only
	"internal/coordinator":        {"internal/domain", "internal/ports"},
}

func TestWorkerAndCoordinatorMeetOnlyThroughPorts(t *testing.T) {
	root := filepath.Join("..", "..")
	checked := 0
	for _, pkg := range packages(t, root) {
		allowed, ok := onlyModuleImports[pkg]
		if !ok {
			continue
		}
		checked++
		for _, im := range imports(t, filepath.Join(root, filepath.FromSlash(pkg))) {
			if !strings.HasPrefix(im, module+"/") {
				continue
			}
			rel := strings.TrimPrefix(im, module+"/")
			ok := false
			for _, a := range allowed {
				ok = ok || under(rel, a)
			}
			if !ok {
				t.Errorf("%s imports %s: allowed module imports are %v", pkg, rel, allowed)
			}
		}
	}
	if checked != len(onlyModuleImports) {
		t.Fatalf("checked %d of %d packages; a package moved?", checked, len(onlyModuleImports))
	}
}
