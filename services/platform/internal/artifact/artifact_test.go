package artifact

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
	"github.com/rluders/amaru/services/platform/internal/store/fsblob"
)

var at = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

type env struct {
	i        *Ingester
	index    ports.ArtifactIndex
	blobs    ports.BlobStore
	location string
	outside  string
}

func setup(t *testing.T, blobs ports.BlobStore) env {
	t.Helper()
	dir := t.TempDir()
	loc, outside := filepath.Join(dir, "out"), filepath.Join(dir, "secret")
	for _, d := range []string{loc, filepath.Join(loc, "sub"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(loc, "report.json"), `{"ok":true}`)
	write(t, filepath.Join(loc, "sub", "log.txt"), "log line")
	write(t, filepath.Join(outside, "passwd"), "root:x:0:0")
	index := fakes.NewArtifactIndex()
	return env{i: &Ingester{Blobs: blobs, Index: index, Now: func() time.Time { return at }}, index: index, blobs: blobs, location: loc, outside: outside}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

var src = Source{ExecutionID: "e1", StepID: "e1.s1", AttemptID: "att-1"}

func both(t *testing.T, f func(t *testing.T, e env)) {
	t.Run("fake blobs", func(t *testing.T) { f(t, setup(t, fakes.NewBlobStore())) })
	t.Run("filesystem blobs", func(t *testing.T) {
		b, err := fsblob.New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		f(t, setup(t, b))
	})
}

func TestIngestComputesTheHashItself(t *testing.T) {
	both(t, func(t *testing.T, e env) {
		vs, err := e.i.Ingest(context.Background(), src, e.location, []Output{
			{ArtifactID: "report", Path: "report.json", MediaType: "application/json"},
			{ArtifactID: "log", Path: "sub/log.txt", MediaType: "text/plain"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if vs[0].Hash != domain.ContentHash([]byte(`{"ok":true}`)) || vs[0].Size != 11 || vs[0].Version != 1 ||
			vs[0].ExecutionID != "e1" || vs[0].AttemptID != "att-1" || vs[1].Path != "sub/log.txt" {
			t.Fatalf("%+v", vs)
		}
		r, err := e.blobs.Open(context.Background(), vs[1].Hash)
		if err != nil {
			t.Fatal(err)
		}
		r.Close()
	})
}

func TestProviderHashIsOnlyAClaim(t *testing.T) {
	both(t, func(t *testing.T, e env) {
		// A right claim is accepted and the stored hash is still the Platform's.
		good := domain.ContentHash([]byte(`{"ok":true}`))
		vs, err := e.i.Ingest(context.Background(), src, e.location, []Output{{ArtifactID: "report", Path: "report.json", ClaimedHash: good}})
		if err != nil || vs[0].Hash != good {
			t.Fatalf("%+v %v", vs, err)
		}
		// A wrong claim rejects the output; the lie is never recorded.
		bad := domain.ContentHash([]byte("something else"))
		_, err = e.i.Ingest(context.Background(), Source{"e2", "e2.s1", "att-2"}, e.location, []Output{{ArtifactID: "other", Path: "report.json", ClaimedHash: bad}})
		if !errors.Is(err, domain.ErrHashMismatch) {
			t.Fatalf("got %v, want hash mismatch", err)
		}
		if vs, _ := e.index.Versions(context.Background(), "other"); len(vs) != 0 {
			t.Fatalf("a rejected claim was recorded: %+v", vs)
		}
	})
}

func TestOutputsOutsideTheLocationAreRejected(t *testing.T) {
	both(t, func(t *testing.T, e env) {
		if err := os.Symlink(filepath.Join(e.outside, "passwd"), filepath.Join(e.location, "link-file")); err != nil {
			t.Skipf("symbolic links are not available: %v", err)
		}
		if err := os.Symlink(e.outside, filepath.Join(e.location, "link-dir")); err != nil {
			t.Fatal(err)
		}
		cases := map[string]string{
			"parent element":          "../secret/passwd",
			"nested parent element":   "sub/../../secret/passwd",
			"absolute path":           filepath.Join(e.outside, "passwd"),
			"symbolic link to a file": "link-file",
			"through a symbolic link": "link-dir/passwd",
			"backslash":               `sub\log.txt`,
			"the location itself":     ".",
			"empty":                   "",
		}
		for name, p := range cases {
			_, err := e.i.Ingest(context.Background(), src, e.location, []Output{{ArtifactID: "x", Path: p}})
			if !errors.Is(err, domain.ErrOutsideOutput) {
				t.Errorf("%s (%q): got %v, want outside the output location", name, p, err)
			}
		}
		if vs, _ := e.index.ByExecution(context.Background(), "e1"); len(vs) != 0 {
			t.Fatalf("a rejected output was recorded: %+v", vs)
		}
	})
}

func TestOneBadOutputRecordsNone(t *testing.T) {
	both(t, func(t *testing.T, e env) {
		_, err := e.i.Ingest(context.Background(), src, e.location, []Output{
			{ArtifactID: "report", Path: "report.json"},
			{ArtifactID: "evil", Path: "../secret/passwd"},
		})
		if !errors.Is(err, domain.ErrOutsideOutput) {
			t.Fatal(err)
		}
		if vs, _ := e.index.ByExecution(context.Background(), "e1"); len(vs) != 0 {
			t.Fatalf("recorded %+v", vs)
		}
	})
}

func TestMissingDirectoryAndOversizedOutputsAreRefused(t *testing.T) {
	both(t, func(t *testing.T, e env) {
		ctx := context.Background()
		if _, err := e.i.Ingest(ctx, src, e.location, []Output{{ArtifactID: "x", Path: "nope.txt"}}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("missing file: %v", err)
		}
		if _, err := e.i.Ingest(ctx, src, e.location, []Output{{ArtifactID: "x", Path: "sub"}}); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("directory: %v", err)
		}
		e.i.MaxSize = 4
		if _, err := e.i.Ingest(ctx, src, e.location, []Output{{ArtifactID: "x", Path: "report.json"}}); !errors.Is(err, domain.ErrTooLarge) {
			t.Errorf("large: %v", err)
		}
		if _, err := e.i.Ingest(ctx, Source{}, e.location, nil); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("no producer: %v", err)
		}
	})
}

func TestIngestingTheSameAttemptAgainAddsNoVersion(t *testing.T) {
	both(t, func(t *testing.T, e env) {
		out := []Output{{ArtifactID: "report", Path: "report.json"}}
		first, err := e.i.Ingest(context.Background(), src, e.location, out)
		if err != nil {
			t.Fatal(err)
		}
		again, err := e.i.Ingest(context.Background(), src, e.location, out)
		if err != nil || again[0].Version != first[0].Version {
			t.Fatalf("%+v %v", again, err)
		}
		// A new attempt that produced new content is a new version of the same Artifact.
		write(t, filepath.Join(e.location, "report.json"), `{"ok":false}`)
		next, err := e.i.Ingest(context.Background(), Source{"e1", "e1.s1", "att-2"}, e.location, out)
		if err != nil || next[0].Version != 2 || next[0].Hash == first[0].Hash {
			t.Fatalf("%+v %v", next, err)
		}
	})
}
