package migrations

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"
)

//go:embed sql
var embedded embed.FS

// Embedded returns the migration directory that ships in the binary.
func Embedded() fs.FS {
	sub, err := fs.Sub(embedded, "sql")
	if err != nil {
		panic(err) // the directory is embedded at build time; failing here is a build defect
	}
	return sub
}

// Dir is a validated, read-only migration directory.
type Dir struct {
	fsys  fs.FS
	files []migrate.File
}

// OpenDir reads goose-format migration files from fsys and verifies atlas.sum. Only files
// ending in ".sql" are migrations; the sum file must list exactly those files.
func OpenDir(fsys fs.FS) (*Dir, error) {
	d := &Dir{fsys: fsys}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("migrations: read directory: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrations: read %s: %w", e.Name(), err)
		}
		d.files = append(d.files, &sqltool.GooseFile{LocalFile: migrate.NewLocalFile(e.Name(), data)})
	}
	sort.Slice(d.files, func(i, j int) bool { return d.files[i].Name() < d.files[j].Name() })
	seen := map[string]string{}
	for _, f := range d.files {
		if prev, dup := seen[f.Version()]; dup {
			return nil, fmt.Errorf("migrations: %s and %s share version %q", prev, f.Name(), f.Version())
		}
		seen[f.Version()] = f.Name()
		if err := migrate.CheckVersion(f.Version()); err != nil {
			return nil, fmt.Errorf("migrations: %s: %w", f.Name(), err)
		}
	}
	if err := migrate.Validate(d); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrChecksum, err)
	}
	return d, nil
}

// Versions lists the migration versions in application order.
func (d *Dir) Versions() []string {
	out := make([]string, len(d.files))
	for i, f := range d.files {
		out[i] = f.Version()
	}
	return out
}

// Latest is the newest version, or "" when the directory has no migration.
func (d *Dir) Latest() string {
	if len(d.files) == 0 {
		return ""
	}
	return d.files[len(d.files)-1].Version()
}

// migrate.Dir implementation (read-only).

// Open implements fs.FS.
func (d *Dir) Open(name string) (fs.File, error) { return d.fsys.Open(name) }

// WriteFile implements migrate.Dir; the embedded directory is read-only.
func (d *Dir) WriteFile(string, []byte) error {
	return errors.New("migrations: directory is read-only")
}

// Files implements migrate.Dir.
func (d *Dir) Files() ([]migrate.File, error) { return append([]migrate.File(nil), d.files...), nil }

// Checksum implements migrate.Dir.
func (d *Dir) Checksum() (migrate.HashFile, error) { return migrate.NewHashFile(d.files) }

func (d *Dir) file(version string) migrate.File {
	for _, f := range d.files {
		if f.Version() == version {
			return f
		}
	}
	return nil
}
