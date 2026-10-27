package migrations

import (
	"errors"
	"flag"
	"os"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"ariga.io/atlas/sql/migrate"
)

var update = flag.Bool("update", false, "rewrite sql/atlas.sum for the embedded directory")

const (
	v1 = "20261101000000"
	v2 = "20261102000000"
)

const createA = `-- +goose Up
CREATE TABLE a (id integer);

-- +goose Down
DROP TABLE a;
`

// fixture builds a directory from name to content and writes a matching atlas.sum.
func fixture(t *testing.T, files map[string]string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	var list []migrate.File
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
		if strings.HasSuffix(name, ".sql") {
			list = append(list, migrate.NewLocalFile(name, []byte(body)))
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	sum, err := migrate.NewHashFile(list)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sum.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	fsys[migrate.HashFileName] = &fstest.MapFile{Data: b}
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

func TestOpenDirIgnoresTheDownSection(t *testing.T) {
	d, err := OpenDir(fixture(t, map[string]string{v1 + "_a.sql": createA}))
	if err != nil {
		t.Fatal(err)
	}
	stmts, err := d.files[0].Stmts()
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 1 || !strings.HasPrefix(stmts[0], "CREATE TABLE a") {
		t.Fatalf("statements = %q, want only the Up statement (migrations are forward-only)", stmts)
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

func TestOpenDirRefusesWhatAtlasSumDoesNotCover(t *testing.T) {
	good := map[string]string{v1 + "_a.sql": createA}
	for name, mutate := range map[string]func(fstest.MapFS){
		"an edited file": func(f fstest.MapFS) { f[v1+"_a.sql"] = &fstest.MapFile{Data: []byte(createA + "-- edited\n")} },
		"an added file":  func(f fstest.MapFS) { f[v2+"_b.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n")} },
		"a removed file": func(f fstest.MapFS) { delete(f, v1+"_a.sql") },
		"a missing sum":  func(f fstest.MapFS) { delete(f, migrate.HashFileName) },
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
	if _, err := OpenDir(fixture(t, map[string]string{"first_a.sql": createA})); err == nil {
		t.Fatal("a non-timestamp version was accepted")
	}
	_, err := OpenDir(fixture(t, map[string]string{v1 + "_a.sql": createA, v1 + "_b.sql": createA}))
	if err == nil || !strings.Contains(err.Error(), "share version") {
		t.Fatalf("err = %v, want a duplicate-version error", err)
	}
}

func TestOpenDirIsReadOnly(t *testing.T) {
	d, err := OpenDir(fixture(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFile("x.sql", nil); err == nil {
		t.Fatal("WriteFile succeeded")
	}
}

// TestEmbeddedDirectory: the directory in the binary is valid. Run with -update after adding or
// changing a migration file to rewrite atlas.sum (or run `atlas migrate hash --dir-format goose`).
func TestEmbeddedDirectory(t *testing.T) {
	if *update {
		var list []migrate.File
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
				list = append(list, migrate.NewLocalFile(e.Name(), b))
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
		sum, err := migrate.NewHashFile(list)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := sum.MarshalText()
		if err := os.WriteFile("sql/"+migrate.HashFileName, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := OpenDir(Embedded()); err != nil {
		t.Fatalf("embedded migration directory: %v (run go test -update ./internal/store/migrations)", err)
	}
}
