package migrations

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

//go:embed sql
var embedded embed.FS

// SumFile is the name of the integrity manifest inside the directory.
const SumFile = "migrations.sum"

// versionRE: a 14-digit UTC timestamp, then an underscore and a name.
var versionRE = regexp.MustCompile(`^(\d{14})_[A-Za-z0-9_.-]+\.sql$`)

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
	fsys     fs.FS
	versions []string          // ascending
	sums     map[string]string // version to sha256 hex of the file
}

// Sum renders the manifest for the given files (name to content): one "<sha256>  <name>" line
// per file, sorted by name. OpenDir compares a directory with it.
func Sum(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	var b bytes.Buffer
	for _, n := range names {
		h := sha256.Sum256(files[n])
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(h[:]), n)
	}
	return b.Bytes()
}

// OpenDir reads goose-format migration files from fsys and verifies migrations.sum. Only files
// ending in ".sql" are migrations; the manifest must list exactly those files.
func OpenDir(fsys fs.FS) (*Dir, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("migrations: read directory: %w", err)
	}
	d := &Dir{fsys: fsys, sums: map[string]string{}}
	files := map[string][]byte{}
	names := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := versionRE.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migrations: %s: name must be <YYYYMMDDHHMMSS>_<name>.sql", e.Name())
		}
		if prev, dup := names[m[1]]; dup {
			return nil, fmt.Errorf("migrations: %s and %s share version %s", prev, e.Name(), m[1])
		}
		names[m[1]] = e.Name()
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrations: read %s: %w", e.Name(), err)
		}
		files[e.Name()] = data
		d.versions = append(d.versions, m[1])
		h := sha256.Sum256(data)
		d.sums[m[1]] = hex.EncodeToString(h[:])
	}
	sort.Strings(d.versions)
	want, err := fs.ReadFile(fsys, SumFile)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrChecksum, err)
	}
	if !bytes.Equal(normalize(want), normalize(Sum(files))) {
		return nil, fmt.Errorf("%w: %s lists different files or contents (run go test ./internal/store/migrations -update after an intended change)", ErrChecksum, SumFile)
	}
	return d, nil
}

func normalize(b []byte) []byte {
	var out bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			out.WriteString(l + "\n")
		}
	}
	return out.Bytes()
}

// Versions lists the migration versions in application order.
func (d *Dir) Versions() []string { return append([]string(nil), d.versions...) }

// Latest is the newest version, or "" when the directory has no migration.
func (d *Dir) Latest() string {
	if len(d.versions) == 0 {
		return ""
	}
	return d.versions[len(d.versions)-1]
}

// ErrChecksum: the directory does not match migrations.sum.
var ErrChecksum = errors.New("migrations: directory does not match " + SumFile)
