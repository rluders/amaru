package migrations

import (
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

var update = flag.Bool("update", false, "rewrite sql/migrations.sum for the embedded directory")

const (
	v1 = "20261101000000"
	v2 = "20261102000000"
)

const createA = `-- +goose Up
CREATE TABLE a (id integer);

-- +goose Down
DROP TABLE a;
`

// fixture builds a directory from name to content and writes a matching migrations.sum.
func fixture(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	sums := map[string][]byte{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
		if strings.HasSuffix(name, ".sql") {
			sums[name] = []byte(body)
		}
	}
	fsys[SumFile] = &fstest.MapFile{Data: Sum(sums)}
	return fsys
}

func TestOpenDir(t *testing.T) {
	fsys := fixture(t, map[string]string{
		v1 + "_a.sql": createA,
		v2 + "_b.sql": "-- +goose Up\nCREATE TABLE b (id integer);\n",
		"README.md":   "not a migration",
	})
	d, err := OpenDir(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(d.Versions(), ","); got != v1+","+v2 {
		t.Fatalf("versions = %s", got)
	}
	if d.Latest() != v2 {
		t.Fatalf("latest = %s", d.Latest())
	}
}

func TestOpenDirEmpty(t *testing.T) {
	d, err := OpenDir(fixture(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if d.Latest() != "" || len(d.Versions()) != 0 {
		t.Fatalf("empty directory reports %q", d.Versions())
	}
}

func TestOpenDirRefusesWhatTheManifestDoesNotCover(t *testing.T) {
	good := map[string]string{v1 + "_a.sql": createA}
	for name, mutate := range map[string]func(fstest.MapFS){
		"an edited file": func(f fstest.MapFS) { f[v1+"_a.sql"] = &fstest.MapFile{Data: []byte(createA + "-- edited\n")} },
		"an added file":  func(f fstest.MapFS) { f[v2+"_b.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n")} },
		"a removed file": func(f fstest.MapFS) { delete(f, v1+"_a.sql") },
		"a missing sum":  func(f fstest.MapFS) { delete(f, SumFile) },
	} {
		t.Run(name, func(t *testing.T) {
			fsys := fixture(t, good)
			mutate(fsys)
			if _, err := OpenDir(fsys); !errors.Is(err, ErrChecksum) {
				t.Fatalf("err = %v, want ErrChecksum", err)
			}
		})
	}
}

func TestOpenDirRefusesBadVersions(t *testing.T) {
	for _, name := range []string{"first_a.sql", "2026110100000_short.sql", v1 + "a.sql"} {
		if _, err := OpenDir(fixture(t, map[string]string{name: createA})); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	_, err := OpenDir(fixture(t, map[string]string{v1 + "_a.sql": createA, v1 + "_b.sql": createA}))
	if err == nil || !strings.Contains(err.Error(), "share version") {
		t.Fatalf("err = %v, want a duplicate-version error", err)
	}
}

// TestEmbeddedDirectory: the directory in the binary is valid. Run with -update after adding or
// changing a migration file to rewrite migrations.sum.
func TestEmbeddedDirectory(t *testing.T) {
	if *update {
		files := map[string][]byte{}
		ents, err := os.ReadDir("sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".sql") {
				b, err := os.ReadFile("sql/" + e.Name())
				if err != nil {
					t.Fatal(err)
				}
				files[e.Name()] = b
			}
		}
		if err := os.WriteFile("sql/"+SumFile, Sum(files), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := OpenDir(Embedded()); err != nil {
		t.Fatalf("embedded migration directory: %v", err)
	}
}
